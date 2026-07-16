package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/ekoci/mini-docker/internal/image"
)

func TestCloneIntoBuildRootPreservesSpecialModeBits(t *testing.T) {
	sourceRootFS := t.TempDir()
	stickyDir := filepath.Join(sourceRootFS, "tmp")
	if err := os.Mkdir(stickyDir, 0o755); err != nil {
		t.Fatalf("Mkdir(sticky dir) error = %v", err)
	}
	if err := syscall.Chmod(stickyDir, 0o3755); err != nil {
		t.Fatalf("Chmod(sticky dir) error = %v", err)
	}

	setIDFile := filepath.Join(sourceRootFS, "usr", "local", "bin", "helper")
	if err := os.MkdirAll(filepath.Dir(setIDFile), 0o755); err != nil {
		t.Fatalf("MkdirAll(file parent) error = %v", err)
	}
	if err := os.WriteFile(setIDFile, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(setid file) error = %v", err)
	}
	if err := syscall.Chmod(setIDFile, 0o6755); err != nil {
		t.Fatalf("Chmod(setid file) error = %v", err)
	}

	assertUnixModePreserved(t, stickyDir, 0o7777, 0o3755)
	assertUnixModePreserved(t, setIDFile, 0o7777, 0o6755)

	buildRootFS, cleanup, err := cloneIntoBuildRoot(sourceRootFS)
	if err != nil {
		t.Fatalf("cloneIntoBuildRoot() error = %v", err)
	}
	defer cleanup()

	clonedStickyDir := filepath.Join(buildRootFS, "tmp")
	assertUnixModePreserved(t, clonedStickyDir, 0o7777, 0o3755)

	clonedSetIDFile := filepath.Join(buildRootFS, "usr", "local", "bin", "helper")
	assertUnixModePreserved(t, clonedSetIDFile, 0o7777, 0o6755)
}

func TestBuildAppliesDockerfileSequentially(t *testing.T) {
	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	baseRootFS := filepath.Join(t.TempDir(), "base-rootfs")
	if err := os.MkdirAll(filepath.Join(baseRootFS, "etc"), 0o755); err != nil {
		t.Fatalf("MkdirAll(base rootfs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseRootFS, "etc", "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile(base file) error = %v", err)
	}
	if err := store.Put(context.Background(), "alpine:3.22", baseRootFS, image.Metadata{Name: "alpine:3.22", Command: []string{"/bin/sh"}}); err != nil {
		t.Fatalf("store.Put(base) error = %v", err)
	}

	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM alpine:3.22\nCOPY hello.sh /hello.sh\nRUN chmod +x /hello.sh\nCMD [\"/hello.sh\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(Dockerfile) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, "hello.sh"), []byte("#!/bin/sh\necho hello\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	executor := &fakeExecutor{executeFn: func(_ context.Context, rootfs string, command []string) error {
		contents, err := os.ReadFile(filepath.Join(rootfs, "hello.sh"))
		if err != nil {
			return err
		}
		if string(contents) != "#!/bin/sh\necho hello\n" {
			return errors.New("COPY did not complete before RUN")
		}
		return os.WriteFile(filepath.Join(rootfs, "run-marker"), []byte(strings.Join(command, " ")), 0o644)
	}}

	builder := New(store, executor)
	if err := builder.Build(context.Background(), "hello:latest", contextDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(executor.calls) != 1 {
		t.Fatalf("executor call count = %d, want 1", len(executor.calls))
	}
	wantCommand := []string{"/bin/sh", "-c", "chmod +x /hello.sh"}
	if !reflect.DeepEqual(executor.calls[0].command, wantCommand) {
		t.Fatalf("executor command = %#v, want %#v", executor.calls[0].command, wantCommand)
	}

	builtImage, err := store.Get("hello:latest")
	if err != nil {
		t.Fatalf("store.Get(built image) error = %v", err)
	}
	baseContents, err := os.ReadFile(filepath.Join(builtImage.RootFS, "etc", "base.txt"))
	if err != nil {
		t.Fatalf("ReadFile(base file) error = %v", err)
	}
	if string(baseContents) != "base" {
		t.Fatalf("base file contents = %q, want %q", string(baseContents), "base")
	}
	copiedContents, err := os.ReadFile(filepath.Join(builtImage.RootFS, "hello.sh"))
	if err != nil {
		t.Fatalf("ReadFile(copied file) error = %v", err)
	}
	if string(copiedContents) != "#!/bin/sh\necho hello\n" {
		t.Fatalf("copied file contents = %q, want source contents", string(copiedContents))
	}
	runMarker, err := os.ReadFile(filepath.Join(builtImage.RootFS, "run-marker"))
	if err != nil {
		t.Fatalf("ReadFile(run marker) error = %v", err)
	}
	if string(runMarker) != strings.Join(wantCommand, " ") {
		t.Fatalf("run marker contents = %q, want %q", string(runMarker), strings.Join(wantCommand, " "))
	}
	if !reflect.DeepEqual(builtImage.Metadata.Command, []string{"/hello.sh"}) {
		t.Fatalf("built image CMD = %#v, want %#v", builtImage.Metadata.Command, []string{"/hello.sh"})
	}
}

func TestBuildResolvesNormalizedPulledBaseImage(t *testing.T) {
	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	baseRootFS := filepath.Join(t.TempDir(), "base-rootfs")
	if err := os.MkdirAll(filepath.Join(baseRootFS, "bin"), 0o755); err != nil {
		t.Fatalf("MkdirAll(base rootfs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseRootFS, "bin", "sh"), []byte("shell"), 0o755); err != nil {
		t.Fatalf("WriteFile(base shell) error = %v", err)
	}
	if err := store.Put(context.Background(), "index.docker.io/library/alpine:3.22", baseRootFS, image.Metadata{Name: "index.docker.io/library/alpine:3.22", Command: []string{"/bin/sh"}}); err != nil {
		t.Fatalf("store.Put(base) error = %v", err)
	}

	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM alpine:3.22\nCMD [\"/bin/sh\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(Dockerfile) error = %v", err)
	}

	builder := New(store, &fakeExecutor{})
	if err := builder.Build(context.Background(), "hello:latest", contextDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if _, err := store.Get("hello:latest"); err != nil {
		t.Fatalf("store.Get(built image) error = %v", err)
	}
}

func TestBuildMissingBaseImageGuidance(t *testing.T) {
	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM alpine:3.22\nCMD [\"/bin/sh\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(Dockerfile) error = %v", err)
	}

	builder := New(store, &fakeExecutor{})
	err := builder.Build(context.Background(), "hello:latest", contextDir)
	if err == nil {
		t.Fatal("Build() error = nil, want missing base failure")
	}
	const want = "base image is not local; run: mini-docker pull alpine:3.22"
	if err.Error() != want {
		t.Fatalf("Build() error = %q, want %q", err.Error(), want)
	}
	if _, getErr := store.Get("hello:latest"); !errors.Is(getErr, image.ErrNotFound) {
		t.Fatalf("store.Get(built image) error = %v, want ErrNotFound", getErr)
	}
}

func TestBuildRejectsUnsupportedDockerfileBeforeMutation(t *testing.T) {
	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	baseRootFS := t.TempDir()
	if err := store.Put(context.Background(), "alpine:3.22", baseRootFS, image.Metadata{Name: "alpine:3.22", Command: []string{"/bin/sh"}}); err != nil {
		t.Fatalf("store.Put(base) error = %v", err)
	}
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM alpine:3.22\nCOPY hello.sh /hello.sh\nENV DEMO=1\nCMD [\"/hello.sh\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(Dockerfile) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, "hello.sh"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	executor := &fakeExecutor{}
	builder := New(store, executor)
	err := builder.Build(context.Background(), "hello:latest", contextDir)
	if err == nil {
		t.Fatal("Build() error = nil, want Dockerfile rejection")
	}
	if !strings.Contains(err.Error(), "unsupported instruction") {
		t.Fatalf("Build() error = %q, want unsupported instruction", err.Error())
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor call count = %d, want 0", len(executor.calls))
	}
	if _, getErr := store.Get("hello:latest"); !errors.Is(getErr, image.ErrNotFound) {
		t.Fatalf("store.Get(built image) error = %v, want ErrNotFound", getErr)
	}
}

func TestBuildDoesNotPublishImageWhenRunFails(t *testing.T) {
	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	baseRootFS := t.TempDir()
	if err := store.Put(context.Background(), "alpine:3.22", baseRootFS, image.Metadata{Name: "alpine:3.22", Command: []string{"/bin/sh"}}); err != nil {
		t.Fatalf("store.Put(base) error = %v", err)
	}
	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM alpine:3.22\nRUN echo failing\nCMD [\"/bin/sh\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(Dockerfile) error = %v", err)
	}

	executor := &fakeExecutor{executeFn: func(_ context.Context, rootfs string, command []string) error {
		return errors.New("boom")
	}}
	builder := New(store, executor)
	err := builder.Build(context.Background(), "hello:latest", contextDir)
	if err == nil {
		t.Fatal("Build() error = nil, want executor failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Build() error = %q, want executor failure", err.Error())
	}
	if _, getErr := store.Get("hello:latest"); !errors.Is(getErr, image.ErrNotFound) {
		t.Fatalf("store.Get(built image) error = %v, want ErrNotFound", getErr)
	}
}

type fakeExecutor struct {
	calls     []executorCall
	executeFn func(context.Context, string, []string) error
}

type executorCall struct {
	rootfs  string
	command []string
}

func (executor *fakeExecutor) Execute(ctx context.Context, rootfs string, command []string) error {
	executor.calls = append(executor.calls, executorCall{
		rootfs:  rootfs,
		command: append([]string(nil), command...),
	})
	if executor.executeFn != nil {
		return executor.executeFn(ctx, rootfs, append([]string(nil), command...))
	}
	return nil
}

func TestCopyRegularFileIntoAbsoluteDestination(t *testing.T) {
	contextDir := t.TempDir()
	rootfsDir := t.TempDir()
	sourcePath := filepath.Join(contextDir, "bin", "hello.sh")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(source parent) error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("#!/bin/sh\necho hello\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	if err := copyFileIntoRootFS(contextDir, rootfsDir, "bin/hello.sh", "/usr/local/bin/hello.sh"); err != nil {
		t.Fatalf("copyFileIntoRootFS() error = %v", err)
	}

	destinationPath := filepath.Join(rootfsDir, "usr", "local", "bin", "hello.sh")
	contents, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatalf("ReadFile(destination) error = %v", err)
	}
	if string(contents) != "#!/bin/sh\necho hello\n" {
		t.Fatalf("destination contents = %q, want exact copy", string(contents))
	}
	info, err := os.Stat(destinationPath)
	if err != nil {
		t.Fatalf("Stat(destination) error = %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("destination mode = %o, want 755", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(rootfsDir, "usr", "local", "bin")); err != nil {
		t.Fatalf("Stat(destination parent) error = %v", err)
	}
}

func TestCopyRejectsUnsafePathsAndFileTypes(t *testing.T) {
	contextDir := t.TempDir()
	rootfsDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside) error = %v", err)
	}

	regularSource := filepath.Join(contextDir, "hello.txt")
	if err := os.WriteFile(regularSource, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular source) error = %v", err)
	}
	directorySource := filepath.Join(contextDir, "dir")
	if err := os.MkdirAll(directorySource, 0o755); err != nil {
		t.Fatalf("MkdirAll(directory source) error = %v", err)
	}
	fifoSource := filepath.Join(contextDir, "named-pipe")
	if err := syscall.Mkfifo(fifoSource, 0o644); err != nil {
		t.Fatalf("Mkfifo(source) error = %v", err)
	}
	symlinkSource := filepath.Join(contextDir, "link-outside")
	if err := os.Symlink(outsideFile, symlinkSource); err != nil {
		t.Fatalf("Symlink(source escape) error = %v", err)
	}

	rootfsEscapeTarget := filepath.Join(outsideDir, "escaped.txt")
	rootfsEscapeLink := filepath.Join(rootfsDir, "escape")
	if err := os.Symlink(outsideDir, rootfsEscapeLink); err != nil {
		t.Fatalf("Symlink(rootfs escape) error = %v", err)
	}

	tests := []struct {
		name        string
		source      string
		destination string
		wantErr     string
	}{
		{name: "parent traversal source", source: "../outside.txt", destination: "/app/outside.txt", wantErr: "build context"},
		{name: "absolute source", source: outsideFile, destination: "/app/outside.txt", wantErr: "build context"},
		{name: "symlink escaping context", source: "link-outside", destination: "/app/outside.txt", wantErr: "build context"},
		{name: "relative destination", source: "hello.txt", destination: "app/hello.txt", wantErr: "absolute"},
		{name: "directory source", source: "dir", destination: "/app/dir", wantErr: "regular file"},
		{name: "special file source", source: "named-pipe", destination: "/app/named-pipe", wantErr: "regular file"},
		{name: "rootfs symlink escape", source: "hello.txt", destination: "/escape/escaped.txt", wantErr: "rootfs"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := copyFileIntoRootFS(contextDir, rootfsDir, test.source, test.destination)
			if err == nil {
				t.Fatal("copyFileIntoRootFS() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("copyFileIntoRootFS() error = %q, want substring %q", err.Error(), test.wantErr)
			}
		})
	}

	if _, err := os.Stat(rootfsEscapeTarget); !os.IsNotExist(err) {
		t.Fatalf("rootfs escape target was modified: Stat error = %v", err)
	}
}

func assertUnixModePreserved(t *testing.T, path string, mask, want uint32) {
	t.Helper()

	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if got := stat.Mode & mask; got != want {
		t.Fatalf("mode(%s) = %o, want %o", path, got, want)
	}
}
