package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func copyFileIntoRootFS(contextDir, rootfsDir, source, destination string) error {
	resolvedContextDir, err := filepath.EvalSymlinks(contextDir)
	if err != nil {
		return fmt.Errorf("resolve build context: %w", err)
	}
	resolvedRootFSDir, err := filepath.EvalSymlinks(rootfsDir)
	if err != nil {
		return fmt.Errorf("resolve rootfs: %w", err)
	}

	resolvedSourcePath, err := resolveSourcePath(resolvedContextDir, source)
	if err != nil {
		return err
	}
	resolvedDestinationPath, err := resolveDestinationPath(resolvedRootFSDir, destination)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedDestinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination parent: %w", err)
	}
	if err := removeExistingDestination(resolvedDestinationPath); err != nil {
		return err
	}

	return copyRegularFile(resolvedSourcePath, resolvedDestinationPath)
}

func resolveSourcePath(contextDir, source string) (string, error) {
	if source == "" || filepath.IsAbs(source) {
		return "", fmt.Errorf("COPY source must stay within build context")
	}
	cleanSource := filepath.Clean(source)
	if cleanSource == ".." || strings.HasPrefix(cleanSource, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("COPY source escapes build context")
	}

	resolvedSourcePath, err := filepath.EvalSymlinks(filepath.Join(contextDir, cleanSource))
	if err != nil {
		return "", fmt.Errorf("resolve COPY source: %w", err)
	}
	if !pathWithinRoot(contextDir, resolvedSourcePath) {
		return "", fmt.Errorf("COPY source escapes build context")
	}

	info, err := os.Stat(resolvedSourcePath)
	if err != nil {
		return "", fmt.Errorf("stat COPY source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("COPY source must be a regular file")
	}
	return resolvedSourcePath, nil
}

func resolveDestinationPath(rootfsDir, destination string) (string, error) {
	if !filepath.IsAbs(destination) {
		return "", fmt.Errorf("COPY destination must be absolute")
	}

	cleanDestination := filepath.Clean(destination)
	relativeDestination := strings.TrimPrefix(cleanDestination, string(filepath.Separator))
	if relativeDestination == "" || relativeDestination == "." {
		return "", fmt.Errorf("COPY destination must name a file path")
	}

	destinationPath := filepath.Join(rootfsDir, relativeDestination)
	resolvedParentPath, err := resolveExistingPath(filepath.Dir(destinationPath))
	if err != nil {
		return "", fmt.Errorf("resolve COPY destination parent: %w", err)
	}
	if !pathWithinRoot(rootfsDir, resolvedParentPath) {
		return "", fmt.Errorf("COPY destination escapes rootfs")
	}

	return destinationPath, nil
}

func resolveExistingPath(path string) (string, error) {
	currentPath := path
	for {
		_, err := os.Lstat(currentPath)
		switch {
		case err == nil:
			return filepath.EvalSymlinks(currentPath)
		case os.IsNotExist(err):
			parentPath := filepath.Dir(currentPath)
			if parentPath == currentPath {
				return "", err
			}
			currentPath = parentPath
		default:
			return "", err
		}
	}
}

func removeExistingDestination(destinationPath string) error {
	info, err := os.Lstat(destinationPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect COPY destination: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("COPY destination escapes rootfs")
	}
	if info.IsDir() {
		return fmt.Errorf("COPY destination is a directory")
	}
	if err := os.Remove(destinationPath); err != nil {
		return fmt.Errorf("replace COPY destination: %w", err)
	}
	return nil
}

func copyRegularFile(sourcePath, destinationPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat COPY source: %w", err)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open COPY source: %w", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create COPY destination: %w", err)
	}
	_, copyErr := io.Copy(destinationFile, sourceFile)
	closeErr := destinationFile.Close()
	if copyErr != nil {
		return fmt.Errorf("copy COPY contents: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close COPY destination: %w", closeErr)
	}
	if err := os.Chmod(destinationPath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("preserve COPY mode: %w", err)
	}
	return nil
}

func pathWithinRoot(rootPath, candidatePath string) bool {
	relativePath, err := filepath.Rel(rootPath, candidatePath)
	if err != nil {
		return false
	}
	return relativePath == "." || (relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}
