package builder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ekoci/mini-docker/internal/dockerfile"
	"github.com/ekoci/mini-docker/internal/image"
	"github.com/ekoci/mini-docker/internal/runtime"
)

type Executor interface {
	Execute(ctx context.Context, rootfs string, command []string) error
}

type Builder struct {
	store    *image.Store
	executor Executor
}

type runtimeExecutor struct{}

type fileIdentity struct {
	device uint64
	inode  uint64
}

func New(store *image.Store, executor Executor) *Builder {
	if executor == nil {
		executor = runtimeExecutor{}
	}
	return &Builder{store: store, executor: executor}
}

func (builder *Builder) Build(ctx context.Context, tag, contextDir string) error {
	instructions, err := parseDockerfile(contextDir)
	if err != nil {
		return err
	}
	fromInstruction, ok := instructions[0].(dockerfile.From)
	if !ok {
		return fmt.Errorf("Dockerfile must begin with FROM")
	}

	baseImage, err := builder.resolveBaseImage(fromInstruction.Image)
	if errors.Is(err, image.ErrNotFound) {
		return fmt.Errorf("base image is not local; run: mini-docker pull %s", fromInstruction.Image)
	}
	if err != nil {
		return err
	}

	buildRootFS, cleanup, err := cloneIntoBuildRoot(baseImage.RootFS)
	if err != nil {
		return err
	}
	defer cleanup()

	metadata := image.Metadata{
		Name:    tag,
		Command: append([]string(nil), baseImage.Metadata.Command...),
	}

	for _, instruction := range instructions[1:] {
		switch instruction := instruction.(type) {
		case dockerfile.Copy:
			if err := copyFileIntoRootFS(contextDir, buildRootFS, instruction.Source, instruction.Destination); err != nil {
				return err
			}
		case dockerfile.Run:
			if err := builder.executor.Execute(ctx, buildRootFS, []string{"/bin/sh", "-c", instruction.Command}); err != nil {
				return err
			}
		case dockerfile.Cmd:
			metadata.Command = append([]string(nil), instruction.Command...)
		default:
			return fmt.Errorf("unsupported Dockerfile instruction %T", instruction)
		}
	}

	return builder.store.Put(ctx, tag, buildRootFS, metadata)
}

func (runtimeExecutor) Execute(ctx context.Context, rootfs string, command []string) error {
	return runtime.RunRootFS(ctx, runtime.Config{RootFS: rootfs, Command: command})
}

func parseDockerfile(contextDir string) ([]dockerfile.Instruction, error) {
	dockerfilePath := filepath.Join(contextDir, "Dockerfile")
	dockerfileFile, err := os.Open(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("open Dockerfile: %w", err)
	}
	defer dockerfileFile.Close()

	instructions, err := dockerfile.Parse(dockerfileFile)
	if err != nil {
		return nil, err
	}
	return instructions, nil
}

func (builder *Builder) resolveBaseImage(reference string) (image.Image, error) {
	return builder.store.Resolve(reference)
}

func cloneIntoBuildRoot(sourceRootFS string) (string, func(), error) {
	temporaryRoot, err := os.MkdirTemp("", "mini-docker-build-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary build directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(temporaryRoot)
	}

	buildRootFS := filepath.Join(temporaryRoot, "rootfs")
	if err := copyTree(sourceRootFS, buildRootFS); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("clone base rootfs: %w", err)
	}
	return buildRootFS, cleanup, nil
}

func copyTree(sourceRoot, destinationRoot string) error {
	hardLinks := make(map[fileIdentity]string)
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			_, rawMode, err := fileModeAndRawModeAtPath(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
				return err
			}
			return syscall.Chmod(destinationRoot, rawMode)
		}

		destinationPath := filepath.Join(destinationRoot, relativePath)
		mode, rawMode, err := fileModeAndRawModeAtPath(path)
		if err != nil {
			return err
		}

		switch {
		case mode.IsDir():
			if err := os.MkdirAll(destinationPath, 0o700); err != nil {
				return err
			}
			return syscall.Chmod(destinationPath, rawMode)
		case mode.IsRegular():
			identity, linkCount, err := hardLinkIdentityAtPath(path)
			if err != nil {
				return err
			}
			if linkCount > 1 {
				if existingPath, ok := hardLinks[identity]; ok {
					return os.Link(existingPath, destinationPath)
				}
			}
			if err := copyTreeRegularFile(path, destinationPath, rawMode); err != nil {
				return err
			}
			if linkCount > 1 {
				hardLinks[identity] = destinationPath
			}
			return nil
		case mode&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(target, destinationPath)
		default:
			return fmt.Errorf("unsupported rootfs entry type at %q", relativePath)
		}
	})
}

func hardLinkIdentityAtPath(path string) (fileIdentity, uint64, error) {
	var stat syscall.Stat_t
	if err := syscall.Lstat(path, &stat); err != nil {
		return fileIdentity{}, 0, err
	}
	return fileIdentity{device: uint64(stat.Dev), inode: stat.Ino}, uint64(stat.Nlink), nil
}

func copyTreeRegularFile(sourcePath, destinationPath string, rawMode uint32) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fs.FileMode(rawMode&0o777))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destinationFile, sourceFile)
	closeErr := destinationFile.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return syscall.Chmod(destinationPath, rawMode)
}

func fileModeAndRawModeAtPath(path string) (fs.FileMode, uint32, error) {
	var stat syscall.Stat_t
	if err := syscall.Lstat(path, &stat); err != nil {
		return 0, 0, err
	}
	return fileModeFromStat(stat), stat.Mode & 0o7777, nil
}

func fileModeFromStat(stat syscall.Stat_t) fs.FileMode {
	mode := fs.FileMode(stat.Mode & 0o777)
	if stat.Mode&syscall.S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if stat.Mode&syscall.S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	if stat.Mode&syscall.S_ISVTX != 0 {
		mode |= os.ModeSticky
	}

	switch stat.Mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		mode |= os.ModeDir
	case syscall.S_IFLNK:
		mode |= os.ModeSymlink
	case syscall.S_IFBLK:
		mode |= os.ModeDevice
	case syscall.S_IFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	case syscall.S_IFIFO:
		mode |= os.ModeNamedPipe
	case syscall.S_IFSOCK:
		mode |= os.ModeSocket
	}

	return mode
}
