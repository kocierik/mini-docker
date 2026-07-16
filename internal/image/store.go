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
)

var ErrNotFound = errors.New("image not found")

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
	sourceInfo, err := os.Lstat(sourceRootFS)
	if err != nil {
		return fmt.Errorf("inspect source rootfs: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source rootfs is not a directory")
	}
	if err := os.Mkdir(destinationRootFS, 0o700); err != nil {
		return fmt.Errorf("create object rootfs: %w", err)
	}

	type directoryMode struct {
		path string
		mode fs.FileMode
	}
	directories := []directoryMode{{path: destinationRootFS, mode: sourceInfo.Mode().Perm()}}
	err = filepath.WalkDir(sourceRootFS, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceRootFS, sourcePath)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}

		destinationPath := filepath.Join(destinationRootFS, relativePath)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			if err := os.Mkdir(destinationPath, 0o700); err != nil {
				return err
			}
			directories = append(directories, directoryMode{path: destinationPath, mode: info.Mode().Perm()})
			return nil
		case info.Mode().IsRegular():
			return copyRegularFile(sourcePath, destinationPath, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(sourcePath)
			if err != nil {
				return err
			}
			if !safeSymlinkTarget(relativePath, target) {
				return fmt.Errorf("unsafe symlink %q -> %q", relativePath, target)
			}
			return os.Symlink(target, destinationPath)
		default:
			return fmt.Errorf("unsupported file type at %q", relativePath)
		}
	})
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

func copyRegularFile(sourcePath, destinationPath string, mode fs.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

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

func safeSymlinkTarget(linkPath, target string) bool {
	if filepath.IsAbs(target) {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	return resolved != ".." && !strings.HasPrefix(resolved, ".."+string(filepath.Separator))
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
	if err := os.Rename(temporaryPath, filepath.Join(refsRoot, referenceName)); err != nil {
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
