package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStorePutAndGet(t *testing.T) {
	storeRoot := t.TempDir()
	sourceRootFS := createRootFS(t, "first")
	store := NewStore(storeRoot)
	metadata := Metadata{Name: "demo:latest", Command: []string{"/etc/demo", "hello"}}

	if err := store.Put(context.Background(), "demo:latest", sourceRootFS, metadata); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := store.Get("demo:latest")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Metadata.Name != metadata.Name {
		t.Fatalf("Metadata.Name = %q, want %q", got.Metadata.Name, metadata.Name)
	}
	if len(got.Metadata.Command) != 2 || got.Metadata.Command[0] != "/etc/demo" || got.Metadata.Command[1] != "hello" {
		t.Fatalf("Metadata.Command = %#v, want %#v", got.Metadata.Command, metadata.Command)
	}

	demoPath := filepath.Join(got.RootFS, "etc", "demo")
	contents, err := os.ReadFile(demoPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", demoPath, err)
	}
	if string(contents) != "first" {
		t.Fatalf("demo contents = %q, want %q", contents, "first")
	}
	info, err := os.Stat(demoPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", demoPath, err)
	}
	if info.Mode().Perm() != 0o751 {
		t.Fatalf("demo mode = %o, want 751", info.Mode().Perm())
	}
	linkTarget, err := os.Readlink(filepath.Join(got.RootFS, "etc", "demo-link"))
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if linkTarget != "demo" {
		t.Fatalf("symlink target = %q, want %q", linkTarget, "demo")
	}

	objectID := readReference(t, storeRoot, "demo:latest")
	metadataBytes, err := os.ReadFile(filepath.Join(storeRoot, "objects", objectID, "metadata.json"))
	if err != nil {
		t.Fatalf("ReadFile(metadata.json) error = %v", err)
	}
	var persisted Metadata
	if err := json.Unmarshal(metadataBytes, &persisted); err != nil {
		t.Fatalf("Unmarshal(metadata.json) error = %v", err)
	}
	if persisted.Name != metadata.Name || len(persisted.Command) != 2 {
		t.Fatalf("persisted metadata = %#v, want %#v", persisted, metadata)
	}
}

func TestStoreGetUnknownImage(t *testing.T) {
	store := NewStore(t.TempDir())

	_, err := store.Get("missing:latest")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestStorePutReplacesTagWithoutReplacingObjectDirectory(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	name := "demo:latest"

	if err := store.Put(context.Background(), name, createRootFS(t, "first"), Metadata{Name: name}); err != nil {
		t.Fatalf("first Put() error = %v", err)
	}
	firstObjectID := readReference(t, storeRoot, name)
	if err := store.Put(context.Background(), name, createRootFS(t, "second"), Metadata{Name: name}); err != nil {
		t.Fatalf("second Put() error = %v", err)
	}
	secondObjectID := readReference(t, storeRoot, name)

	if firstObjectID == secondObjectID {
		t.Fatalf("replacement reused object ID %q", firstObjectID)
	}
	for _, objectID := range []string{firstObjectID, secondObjectID} {
		info, err := os.Stat(filepath.Join(storeRoot, "objects", objectID))
		if err != nil {
			t.Fatalf("Stat(object %q) error = %v", objectID, err)
		}
		if !info.IsDir() {
			t.Fatalf("object %q is not a directory", objectID)
		}
	}
	got, err := store.Get(name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(got.RootFS, "etc", "demo"))
	if err != nil {
		t.Fatalf("ReadFile(replaced demo) error = %v", err)
	}
	if string(contents) != "second" {
		t.Fatalf("replaced demo contents = %q, want %q", contents, "second")
	}
}

func TestStorePutHonorsCanceledContext(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Put(ctx, "demo:latest", createRootFS(t, "content"), Metadata{Name: "demo:latest"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() error = %v, want context.Canceled", err)
	}
	if _, err := store.Get("demo:latest"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after canceled Put error = %v, want ErrNotFound", err)
	}
}

func TestStoreHashesImageNames(t *testing.T) {
	parent := t.TempDir()
	storeRoot := filepath.Join(parent, "store")
	store := NewStore(storeRoot)
	name := "../escape"

	if err := store.Put(context.Background(), name, createRootFS(t, "content"), Metadata{Name: name}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if _, err := store.Get(name); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "escape")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("image name escaped store: Stat error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(storeRoot, "refs"))
	if err != nil {
		t.Fatalf("ReadDir(refs) error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != referenceName(name) {
		t.Fatalf("refs entries = %#v, want hashed name %q", entries, referenceName(name))
	}
}

func TestStorePutRejectsUnsafeSymlinks(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "absolute", target: "/etc/passwd"},
		{name: "escaping", target: "../../outside"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourceRootFS := t.TempDir()
			if err := os.MkdirAll(filepath.Join(sourceRootFS, "etc"), 0o755); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.Symlink(test.target, filepath.Join(sourceRootFS, "etc", "unsafe")); err != nil {
				t.Fatalf("Symlink() error = %v", err)
			}
			store := NewStore(t.TempDir())

			if err := store.Put(context.Background(), "unsafe:latest", sourceRootFS, Metadata{}); err == nil {
				t.Fatal("Put() error = nil, want unsafe symlink error")
			}
			if _, err := store.Get("unsafe:latest"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get() after rejected Put error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestStorePutRejectsUnsupportedFileTypes(t *testing.T) {
	sourceRootFS := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(sourceRootFS, "pipe"), 0o600); err != nil {
		t.Fatalf("Mkfifo() error = %v", err)
	}
	store := NewStore(t.TempDir())

	if err := store.Put(context.Background(), "unsafe:latest", sourceRootFS, Metadata{}); err == nil {
		t.Fatal("Put() error = nil, want unsupported file type error")
	}
	if _, err := store.Get("unsafe:latest"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after rejected Put error = %v, want ErrNotFound", err)
	}
}

func createRootFS(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	etc := filepath.Join(root, "etc")
	if err := os.MkdirAll(etc, 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(etc, "demo"), []byte(contents), 0o751); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink("demo", filepath.Join(etc, "demo-link")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	return root
}

func readReference(t *testing.T, storeRoot, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(storeRoot, "refs", referenceName(name)))
	if err != nil {
		t.Fatalf("ReadFile(reference) error = %v", err)
	}
	return string(contents)
}

func referenceName(name string) string {
	digest := sha256.Sum256([]byte(name))
	return hex.EncodeToString(digest[:])
}
