package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
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
	rootInfo, err := os.Stat(got.RootFS)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", got.RootFS, err)
	}
	if rootInfo.Mode()&os.ModeSticky == 0 || rootInfo.Mode().Perm() != 0o751 {
		t.Fatalf("rootfs mode = %v, want sticky with perms 751", rootInfo.Mode())
	}
	etcInfo, err := os.Stat(filepath.Join(got.RootFS, "etc"))
	if err != nil {
		t.Fatalf("Stat(etc) error = %v", err)
	}
	if etcInfo.Mode()&os.ModeSetgid == 0 || etcInfo.Mode().Perm() != 0o750 {
		t.Fatalf("etc mode = %v, want setgid with perms 750", etcInfo.Mode())
	}
	info, err := os.Stat(demoPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", demoPath, err)
	}
	if info.Mode()&os.ModeSetuid == 0 || info.Mode().Perm() != 0o751 {
		t.Fatalf("demo mode = %v, want setuid with perms 751", info.Mode())
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
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "refs"))
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

func TestStorePutCancelsDuringTraversal(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	sourceRootFS := createRootFS(t, "content")
	ctx, cancel := context.WithCancel(context.Background())
	previousHook := testHookBeforeCopySourceEntry
	testHookBeforeCopySourceEntry = func(relativePath string) error {
		if relativePath == filepath.Join("etc") {
			cancel()
		}
		return nil
	}
	t.Cleanup(func() {
		testHookBeforeCopySourceEntry = previousHook
	})

	err := store.Put(ctx, "demo:latest", sourceRootFS, Metadata{Name: "demo:latest"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() error = %v, want context.Canceled", err)
	}
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "objects"))
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "refs"))
}

func TestStorePutCancelsAfterTemporaryObjectCreation(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	ctx, cancel := context.WithCancel(context.Background())
	previousHook := testHookBeforeObjectPublication
	hookPath := ""
	testHookBeforeObjectPublication = func(temporaryObjectPath, objectPath string) error {
		hookPath = temporaryObjectPath
		cancel()
		return nil
	}
	t.Cleanup(func() {
		testHookBeforeObjectPublication = previousHook
	})

	err := store.Put(ctx, "demo:latest", createRootFS(t, "content"), Metadata{Name: "demo:latest"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() error = %v, want context.Canceled", err)
	}
	if hookPath == "" {
		t.Fatal("object publication hook was not triggered")
	}
	if _, err := os.Stat(hookPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary object still exists: Stat error = %v", err)
	}
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "objects"))
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "refs"))
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
	if objectID := readReference(t, storeRoot, name); len(objectID) != 64 {
		t.Fatalf("reference object ID length = %d, want 64", len(objectID))
	}
}

func TestStorePutPublishesReferenceWithoutTemporaryFiles(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	name := "demo:latest"

	if err := store.Put(context.Background(), name, createRootFS(t, "content"), Metadata{Name: name}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	objectID := readReference(t, storeRoot, name)
	if len(objectID) != 64 {
		t.Fatalf("reference object ID length = %d, want 64", len(objectID))
	}
	if _, err := hex.DecodeString(objectID); err != nil {
		t.Fatalf("reference object ID = %q, want hex: %v", objectID, err)
	}
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "refs"))
}

func TestStorePutReferencePublicationUsesSingleDirectoryRename(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	previousHook := testHookBeforeReferencePublication
	var temporaryReferencePath string
	var finalReferencePath string
	testHookBeforeReferencePublication = func(tempPath, referencePath string) error {
		temporaryReferencePath = tempPath
		finalReferencePath = referencePath
		return nil
	}
	t.Cleanup(func() {
		testHookBeforeReferencePublication = previousHook
	})

	if err := store.Put(context.Background(), "demo:latest", createRootFS(t, "content"), Metadata{Name: "demo:latest"}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if temporaryReferencePath == "" || finalReferencePath == "" {
		t.Fatal("reference publication hook was not triggered")
	}
	if filepath.Dir(temporaryReferencePath) != filepath.Dir(finalReferencePath) {
		t.Fatalf("reference rename dirs = %q -> %q, want same directory", filepath.Dir(temporaryReferencePath), filepath.Dir(finalReferencePath))
	}
}

func TestStorePutReferencePublishFailureCleansTemporaryRefs(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	name := "demo:latest"
	if err := os.MkdirAll(filepath.Join(storeRoot, "refs", referenceName(name)), 0o755); err != nil {
		t.Fatalf("MkdirAll(conflicting ref) error = %v", err)
	}

	err := store.Put(context.Background(), name, createRootFS(t, "content"), Metadata{Name: name})
	if err == nil {
		t.Fatal("Put() error = nil, want reference publication failure")
	}
	assertNoTemporaryEntries(t, filepath.Join(storeRoot, "refs"))
	entries, readErr := os.ReadDir(filepath.Join(storeRoot, "refs"))
	if readErr != nil {
		t.Fatalf("ReadDir(refs) error = %v", readErr)
	}
	if len(entries) != 1 || !entries[0].IsDir() || entries[0].Name() != referenceName(name) {
		t.Fatalf("refs entries after failure = %#v, want only conflicting directory", entries)
	}
}

func TestStorePutRejectsUnsafeSymlinks(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
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

func TestStorePutPreservesAbsoluteSymlinksWithinRootFS(t *testing.T) {
	sourceRootFS := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceRootFS, "bin"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRootFS, "bin", "busybox"), []byte("busybox"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink("/bin/busybox", filepath.Join(sourceRootFS, "bin", "arch")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	store := NewStore(t.TempDir())
	if err := store.Put(context.Background(), "demo:latest", sourceRootFS, Metadata{Name: "demo:latest"}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := store.Get("demo:latest")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	linkTarget, err := os.Readlink(filepath.Join(got.RootFS, "bin", "arch"))
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if linkTarget != "/bin/busybox" {
		t.Fatalf("symlink target = %q, want %q", linkTarget, "/bin/busybox")
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

func TestStorePutRejectsSockets(t *testing.T) {
	sourceRootFS := t.TempDir()
	listener, err := net.Listen("unix", filepath.Join(sourceRootFS, "socket"))
	if err != nil {
		t.Fatalf("Listen(unix) error = %v", err)
	}
	defer listener.Close()
	store := NewStore(t.TempDir())

	if err := store.Put(context.Background(), "unsafe:latest", sourceRootFS, Metadata{}); err == nil {
		t.Fatal("Put() error = nil, want socket rejection")
	}
}

func TestFileModeFromStatMarksSpecialFilesUnsupported(t *testing.T) {
	charDevice := fileModeFromStat(unix.Stat_t{Mode: unix.S_IFCHR | unix.S_ISUID | 0o600})
	if charDevice&os.ModeDevice == 0 || charDevice&os.ModeCharDevice == 0 || charDevice&os.ModeSetuid == 0 {
		t.Fatalf("char device mode = %v, want device, char-device, and setuid bits", charDevice)
	}
	blockDevice := fileModeFromStat(unix.Stat_t{Mode: unix.S_IFBLK | unix.S_ISGID | 0o640})
	if blockDevice&os.ModeDevice == 0 || blockDevice&os.ModeCharDevice != 0 || blockDevice&os.ModeSetgid == 0 {
		t.Fatalf("block device mode = %v, want block-device and setgid bits", blockDevice)
	}
}

func TestStorePutRejectsSourcePathSwapOutsideRoot(t *testing.T) {
	storeRoot := t.TempDir()
	store := NewStore(storeRoot)
	sourceRootFS := createRootFS(t, "inside")
	outsideRootFS := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideRootFS, "demo"), []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside demo) error = %v", err)
	}

	previousHook := testHookBeforeCopySourceEntry
	triggered := false
	testHookBeforeCopySourceEntry = func(relativePath string) error {
		if triggered || relativePath != filepath.Join("etc") {
			return nil
		}
		triggered = true
		if err := os.Rename(filepath.Join(sourceRootFS, "etc"), filepath.Join(sourceRootFS, "etc-real")); err != nil {
			return err
		}
		return os.Symlink(outsideRootFS, filepath.Join(sourceRootFS, "etc"))
	}
	t.Cleanup(func() {
		testHookBeforeCopySourceEntry = previousHook
	})

	err := store.Put(context.Background(), "swap:latest", sourceRootFS, Metadata{Name: "swap:latest"})
	if err == nil {
		t.Fatal("Put() error = nil, want path swap rejection")
	}
	if !triggered {
		t.Fatal("path swap hook was not triggered")
	}
	if _, err := store.Get("swap:latest"); !errors.Is(err, ErrNotFound) {
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
	if err := os.Chmod(root, os.ModeSticky|0o751); err != nil {
		t.Fatalf("Chmod(root) error = %v", err)
	}
	if err := os.Chmod(etc, os.ModeSetgid|0o750); err != nil {
		t.Fatalf("Chmod(etc) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(etc, "demo"), []byte(contents), 0o751); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(filepath.Join(etc, "demo"), os.ModeSetuid|0o751); err != nil {
		t.Fatalf("Chmod(demo) error = %v", err)
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

func assertNoTemporaryEntries(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", directory, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("temporary entry %q still exists in %q", entry.Name(), directory)
		}
	}
}
