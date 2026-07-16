package image

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var ErrNotFound = errors.New("image not found")

const preservedModeMask = fs.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

var testHookBeforeCopySourceEntry func(relativePath string) error
var testHookBeforeObjectPublication func(temporaryObjectPath, objectPath string) error
var testHookBeforeReferencePublication func(temporaryReferencePath, referencePath string) error

type directoryMode struct {
	path string
	mode fs.FileMode
}

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (store *Store) Put(ctx context.Context, name, sourceRootFS string, metadata Metadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	objectsRoot := filepath.Join(store.root, "objects")
	refsRoot := filepath.Join(store.root, "refs")
	if err := os.MkdirAll(objectsRoot, 0o755); err != nil {
		return fmt.Errorf("create objects directory: %w", err)
	}
	if err := os.MkdirAll(refsRoot, 0o755); err != nil {
		return fmt.Errorf("create refs directory: %w", err)
	}

	temporaryObject, err := os.MkdirTemp(objectsRoot, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary object: %w", err)
	}
	removeTemporaryObject := true
	defer func() {
		if removeTemporaryObject {
			_ = os.RemoveAll(temporaryObject)
		}
	}()

	if err := copyRootFS(ctx, sourceRootFS, filepath.Join(temporaryObject, "rootfs")); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeMetadata(filepath.Join(temporaryObject, "metadata.json"), metadata); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	objectID, err := newObjectID()
	if err != nil {
		return err
	}
	objectPath := filepath.Join(objectsRoot, objectID)
	if testHookBeforeObjectPublication != nil {
		if err := testHookBeforeObjectPublication(temporaryObject, objectPath); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryObject, objectPath); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	removeTemporaryObject = false

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := replaceReference(refsRoot, hashName(name), objectID); err != nil {
		return err
	}
	return nil
}

func (store *Store) Get(name string) (Image, error) {
	referencePath := filepath.Join(store.root, "refs", hashName(name))
	contents, err := os.ReadFile(referencePath)
	if errors.Is(err, os.ErrNotExist) {
		return Image{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return Image{}, fmt.Errorf("read image reference: %w", err)
	}

	objectID := string(contents)
	if !validObjectID(objectID) {
		return Image{}, fmt.Errorf("invalid object ID in image reference")
	}
	objectPath := filepath.Join(store.root, "objects", objectID)
	metadataBytes, err := os.ReadFile(filepath.Join(objectPath, "metadata.json"))
	if err != nil {
		return Image{}, fmt.Errorf("read image metadata: %w", err)
	}
	var metadata Metadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return Image{}, fmt.Errorf("decode image metadata: %w", err)
	}

	rootFS := filepath.Join(objectPath, "rootfs")
	info, err := os.Stat(rootFS)
	if err != nil {
		return Image{}, fmt.Errorf("open image rootfs: %w", err)
	}
	if !info.IsDir() {
		return Image{}, fmt.Errorf("image rootfs is not a directory")
	}
	return Image{RootFS: rootFS, Metadata: metadata}, nil
}

func copyRootFS(ctx context.Context, sourceRootFS, destinationRootFS string) error {
	sourceRoot, err := openDirectoryNoFollow(sourceRootFS)
	if err != nil {
		return fmt.Errorf("inspect source rootfs: %w", err)
	}
	defer sourceRoot.Close()

	sourceInfo, err := sourceRoot.Stat()
	if err != nil {
		return fmt.Errorf("inspect source rootfs: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source rootfs is not a directory")
	}
	if err := os.Mkdir(destinationRootFS, 0o700); err != nil {
		return fmt.Errorf("create object rootfs: %w", err)
	}

	directories := []directoryMode{{path: destinationRootFS, mode: sourceInfo.Mode() & preservedModeMask}}
	err = copyDirectoryContents(ctx, sourceRoot, "", destinationRootFS, &directories)
	if err != nil {
		return fmt.Errorf("copy rootfs: %w", err)
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := os.Chmod(directories[index].path, directories[index].mode); err != nil {
			return fmt.Errorf("preserve directory mode: %w", err)
		}
	}
	return nil
}

func copyDirectoryContents(ctx context.Context, sourceDir *os.File, relativeDirectory, destinationDirectory string, directories *[]directoryMode) error {
	entries, err := sourceDir.ReadDir(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		relativePath := entry.Name()
		if relativeDirectory != "" {
			relativePath = filepath.Join(relativeDirectory, entry.Name())
		}
		if testHookBeforeCopySourceEntry != nil {
			if err := testHookBeforeCopySourceEntry(relativePath); err != nil {
				return err
			}
		}

		mode, err := fileModeAt(int(sourceDir.Fd()), entry.Name())
		if err != nil {
			return err
		}
		if !sameEntryType(entry, mode) {
			return fmt.Errorf("source entry type changed during copy at %q", relativePath)
		}
		destinationPath := filepath.Join(destinationDirectory, entry.Name())

		switch {
		case mode.IsDir():
			if err := os.Mkdir(destinationPath, 0o700); err != nil {
				return err
			}
			*directories = append(*directories, directoryMode{path: destinationPath, mode: mode & preservedModeMask})

			childDir, err := openDirectoryAt(int(sourceDir.Fd()), entry.Name())
			if err != nil {
				return err
			}
			childErr := copyDirectoryContents(ctx, childDir, relativePath, destinationPath, directories)
			closeErr := childDir.Close()
			if childErr != nil {
				return childErr
			}
			if closeErr != nil {
				return closeErr
			}
		case mode.IsRegular():
			if err := copyRegularFileAt(int(sourceDir.Fd()), entry.Name(), destinationPath, mode&preservedModeMask); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			target, err := readlinkAt(int(sourceDir.Fd()), entry.Name())
			if err != nil {
				return err
			}
			if !safeSymlinkTarget(relativePath, target) {
				return fmt.Errorf("unsafe symlink %q -> %q", relativePath, target)
			}
			if err := os.Symlink(target, destinationPath); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported file type at %q", relativePath)
		}
	}

	return nil
}

func copyRegularFileAt(sourceDirFD int, sourceName, destinationPath string, mode fs.FileMode) error {
	source, err := openRegularFileAt(sourceDirFD, sourceName)
	if err != nil {
		return err
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source entry is no longer a regular file")
	}

	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destinationPath, mode)
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	fd, err := unix.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openDirectoryAt(sourceDirFD int, name string) (*os.File, error) {
	fd, err := unix.Openat(sourceDirFD, name, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func openRegularFileAt(sourceDirFD int, name string) (*os.File, error) {
	fd, err := unix.Openat(sourceDirFD, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func fileModeAt(sourceDirFD int, name string) (fs.FileMode, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(sourceDirFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return 0, err
	}
	return fileModeFromStat(stat), nil
}

func fileModeFromStat(stat unix.Stat_t) fs.FileMode {
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

func readlinkAt(sourceDirFD int, name string) (string, error) {
	size := 128
	for {
		buffer := make([]byte, size)
		read, err := unix.Readlinkat(sourceDirFD, name, buffer)
		if err != nil {
			return "", err
		}
		if read < len(buffer) {
			return string(buffer[:read]), nil
		}
		size *= 2
	}
}

func safeSymlinkTarget(linkPath, target string) bool {
	if target == "" {
		return false
	}
	if filepath.IsAbs(target) {
		return true
	}
	if strings.Contains(target, `\\`) {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	return resolved != ".." && !strings.HasPrefix(resolved, ".."+string(filepath.Separator))
}

func sameEntryType(entry fs.DirEntry, mode fs.FileMode) bool {
	switch {
	case entry.Type()&os.ModeSymlink != 0:
		return mode&os.ModeSymlink != 0
	case entry.IsDir():
		return mode.IsDir()
	default:
		return mode.IsRegular()
	}
}

func writeMetadata(path string, metadata Metadata) error {
	contents, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode image metadata: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write image metadata: %w", err)
	}
	return nil
}

func newObjectID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate object ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func replaceReference(refsRoot, referenceName, objectID string) error {
	temporaryReference, err := os.CreateTemp(refsRoot, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary reference: %w", err)
	}
	temporaryPath := temporaryReference.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err := temporaryReference.WriteString(objectID); err != nil {
		_ = temporaryReference.Close()
		return fmt.Errorf("write temporary reference: %w", err)
	}
	if err := temporaryReference.Sync(); err != nil {
		_ = temporaryReference.Close()
		return fmt.Errorf("sync temporary reference: %w", err)
	}
	if err := temporaryReference.Close(); err != nil {
		return fmt.Errorf("close temporary reference: %w", err)
	}
	referencePath := filepath.Join(refsRoot, referenceName)
	if testHookBeforeReferencePublication != nil {
		if err := testHookBeforeReferencePublication(temporaryPath, referencePath); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, referencePath); err != nil {
		return fmt.Errorf("publish image reference: %w", err)
	}
	removeTemporary = false
	return nil
}

func hashName(name string) string {
	digest := sha256.Sum256([]byte(name))
	return hex.EncodeToString(digest[:])
}

func validObjectID(objectID string) bool {
	if len(objectID) != 64 {
		return false
	}
	_, err := hex.DecodeString(objectID)
	return err == nil
}
