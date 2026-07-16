package builder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

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

	baseImage, err := builder.store.Get(fromInstruction.Image)
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
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(destinationRoot, info.Mode().Perm())
		}

		destinationPath := filepath.Join(destinationRoot, relativePath)
		info, err := entry.Info()
		if err != nil {
			return err
		}

		switch mode := info.Mode(); {
		case mode.IsDir():
			return os.MkdirAll(destinationPath, mode.Perm())
		case mode.IsRegular():
			return copyTreeRegularFile(path, destinationPath, mode)
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

func copyTreeRegularFile(sourcePath, destinationPath string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
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
	return os.Chmod(destinationPath, mode.Perm())
}
