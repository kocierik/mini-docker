package registry

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/ekoci/mini-docker/internal/image"
)

const preservedArchiveModeMask = fs.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

type fetchFunc func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error)

type Puller struct {
	store *image.Store
	fetch fetchFunc
}

func NewPuller(store *image.Store) *Puller {
	return &Puller{store: store, fetch: fetchRemoteImage}
}

func (puller *Puller) Pull(ctx context.Context, reference string) error {
	if strings.TrimSpace(reference) == "" {
		return fmt.Errorf("image reference is required")
	}

	parsedReference, err := name.ParseReference(reference)
	if err != nil {
		return fmt.Errorf("parse image reference: %w", err)
	}

	configFile, archive, err := puller.fetch(ctx, parsedReference)
	if err != nil {
		return fmt.Errorf("fetch image: %w", err)
	}
	defer archive.Close()

	temporaryRoot, err := os.MkdirTemp("", "mini-docker-pull-")
	if err != nil {
		return fmt.Errorf("create temporary rootfs: %w", err)
	}
	defer os.RemoveAll(temporaryRoot)

	rootfs := filepath.Join(temporaryRoot, "rootfs")
	if err := os.Mkdir(rootfs, 0o755); err != nil {
		return fmt.Errorf("create temporary rootfs: %w", err)
	}
	if err := extractRootFS(rootfs, archive); err != nil {
		return fmt.Errorf("extract image: %w", err)
	}

	metadata := image.Metadata{
		Name:    parsedReference.Name(),
		Command: slices.Clone(configFile.Config.Cmd),
	}
	if err := puller.store.Put(ctx, parsedReference.Name(), rootfs, metadata); err != nil {
		return fmt.Errorf("publish image: %w", err)
	}
	return nil
}

func fetchRemoteImage(ctx context.Context, reference name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
	remoteImage, err := remote.Image(
		reference,
		remote.WithContext(ctx),
		remote.WithPlatform(v1.Platform{OS: "linux", Architecture: "amd64"}),
	)
	if err != nil {
		return nil, nil, err
	}

	configFile, err := remoteImage.ConfigFile()
	if err != nil {
		return nil, nil, err
	}
	return configFile, mutate.Extract(remoteImage), nil
}

func extractRootFS(rootfs string, archive io.Reader) error {
	reader := tar.NewReader(archive)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}

		relativePath, err := cleanArchivePath(header.Name)
		if err != nil {
			return err
		}
		if relativePath == "." {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := mkdirAllSafe(rootfs, relativePath, header.FileInfo().Mode()&preservedArchiveModeMask); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := writeRegularFile(rootfs, relativePath, header.FileInfo().Mode()&preservedArchiveModeMask, reader); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := writeSymlink(rootfs, relativePath, header.Linkname); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := writeHardLink(rootfs, relativePath, header.Linkname); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type %q at %q", string(header.Typeflag), header.Name)
		}
	}
}

func cleanArchivePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	if strings.Contains(name, `\`) || path.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}

	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return cleaned, nil
}

func mkdirAllSafe(rootfs, relativePath string, mode fs.FileMode) error {
	if relativePath == "." || relativePath == "" {
		return nil
	}

	currentPath := rootfs
	parts := strings.Split(relativePath, "/")
	for index, part := range parts {
		currentPath = filepath.Join(currentPath, part)
		info, err := os.Lstat(currentPath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("unsafe archive path %q", relativePath)
			}
			if !info.IsDir() {
				return fmt.Errorf("archive path collides with non-directory %q", relativePath)
			}
			if index == len(parts)-1 {
				if err := os.Chmod(currentPath, mode); err != nil {
					return fmt.Errorf("chmod directory %q: %w", relativePath, err)
				}
			}
			continue
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect archive path %q: %w", relativePath, err)
		}

		createMode := fs.FileMode(0o755)
		if index == len(parts)-1 {
			createMode = mode
		}
		if err := os.Mkdir(currentPath, createMode); err != nil {
			return fmt.Errorf("create directory %q: %w", relativePath, err)
		}
	}
	return nil
}

func ensureSafeParent(rootfs, relativePath string) error {
	parent := path.Dir(relativePath)
	if parent == "." {
		return nil
	}
	return mkdirAllSafe(rootfs, parent, 0o755)
}

func writeRegularFile(rootfs, relativePath string, mode fs.FileMode, contents io.Reader) error {
	if err := ensureSafeParent(rootfs, relativePath); err != nil {
		return err
	}

	destinationPath := filepath.Join(rootfs, filepath.FromSlash(relativePath))
	file, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create file %q: %w", relativePath, err)
	}
	_, copyErr := io.Copy(file, contents)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write file %q: %w", relativePath, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close file %q: %w", relativePath, closeErr)
	}
	if err := os.Chmod(destinationPath, mode); err != nil {
		return fmt.Errorf("chmod file %q: %w", relativePath, err)
	}
	return nil
}

func writeSymlink(rootfs, relativePath, target string) error {
	if !safeLinkTarget(relativePath, target) {
		return fmt.Errorf("unsafe symlink %q -> %q", relativePath, target)
	}
	if err := ensureSafeParent(rootfs, relativePath); err != nil {
		return err
	}

	destinationPath := filepath.Join(rootfs, filepath.FromSlash(relativePath))
	if err := os.Symlink(target, destinationPath); err != nil {
		return fmt.Errorf("create symlink %q: %w", relativePath, err)
	}
	return nil
}

func writeHardLink(rootfs, relativePath, target string) error {
	targetPath, err := safeHardLinkTarget(rootfs, target)
	if err != nil {
		return fmt.Errorf("unsafe hard link %q -> %q: %w", relativePath, target, err)
	}
	if err := ensureSafeParent(rootfs, relativePath); err != nil {
		return err
	}

	destinationPath := filepath.Join(rootfs, filepath.FromSlash(relativePath))
	if err := os.Link(targetPath, destinationPath); err != nil {
		return fmt.Errorf("create hard link %q: %w", relativePath, err)
	}
	return nil
}

func safeHardLinkTarget(rootfs, target string) (string, error) {
	cleaned, err := cleanArchivePath(target)
	if err != nil {
		return "", err
	}

	currentPath := rootfs
	components := strings.Split(cleaned, "/")
	for index, component := range components {
		currentPath = filepath.Join(currentPath, filepath.FromSlash(component))
		info, err := os.Lstat(currentPath)
		if err != nil {
			return "", fmt.Errorf("inspect target component %q: %w", strings.Join(components[:index+1], "/"), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("target traverses symlink %q", strings.Join(components[:index+1], "/"))
		}
		if index < len(components)-1 && !info.IsDir() {
			return "", fmt.Errorf("target component %q is not a directory", strings.Join(components[:index+1], "/"))
		}
		if index == len(components)-1 && !info.Mode().IsRegular() {
			return "", fmt.Errorf("target %q is not a regular file", target)
		}
	}
	return currentPath, nil
}

func safeRootfsPath(rootfs, archivePath string) (string, error) {
	cleaned, err := cleanArchivePath(archivePath)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(rootfs, filepath.FromSlash(cleaned))
	relativeToRoot, err := filepath.Rel(rootfs, fullPath)
	if err != nil {
		return "", err
	}
	if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", archivePath)
	}
	return fullPath, nil
}

func safeLinkTarget(linkPath, target string) bool {
	if target == "" || strings.Contains(target, `\`) {
		return false
	}
	if path.IsAbs(target) {
		return true
	}
	resolved := path.Clean(path.Join(path.Dir(linkPath), target))
	return resolved != ".." && !strings.HasPrefix(resolved, "../")
}
