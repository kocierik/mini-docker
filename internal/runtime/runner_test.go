package runtime

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ekoci/mini-docker/internal/image"
)

func TestResolveCommandPrefersOverride(t *testing.T) {
	command, err := resolveCommand(image.Metadata{Command: []string{"/image-cmd"}}, Config{Command: []string{"/override", "hello"}})
	if err != nil {
		t.Fatalf("resolveCommand returned error: %v", err)
	}

	want := []string{"/override", "hello"}
	if !reflect.DeepEqual(command, want) {
		t.Fatalf("resolveCommand = %v, want %v", command, want)
	}
}

func TestResolveCommandFallsBackToImageCommand(t *testing.T) {
	command, err := resolveCommand(image.Metadata{Command: []string{"/image-cmd", "hello"}}, Config{})
	if err != nil {
		t.Fatalf("resolveCommand returned error: %v", err)
	}

	want := []string{"/image-cmd", "hello"}
	if !reflect.DeepEqual(command, want) {
		t.Fatalf("resolveCommand = %v, want %v", command, want)
	}
}

func TestResolveCommandRejectsMissingCommand(t *testing.T) {
	_, err := resolveCommand(image.Metadata{}, Config{})
	if err == nil {
		t.Fatal("resolveCommand unexpectedly succeeded")
	}

	const want = "image has no CMD and no command override was provided"
	if err.Error() != want {
		t.Fatalf("resolveCommand error = %q, want %q", err.Error(), want)
	}
}

func TestRunnerResolveImageMatchesNormalizedPulledReference(t *testing.T) {
	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	rootFS := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(rootFS, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(rootfs) error = %v", err)
	}
	if err := store.Put(context.Background(), "index.docker.io/library/alpine:3.22", rootFS, image.Metadata{Name: "index.docker.io/library/alpine:3.22", Command: []string{"/bin/sh"}}); err != nil {
		t.Fatalf("store.Put(normalized image) error = %v", err)
	}

	runner := NewRunner(store)
	img, err := runner.resolveImage("alpine:3.22")
	if err != nil {
		t.Fatalf("runner.resolveImage returned error: %v", err)
	}
	if img.Metadata.Name != "index.docker.io/library/alpine:3.22" {
		t.Fatalf("runner.resolveImage metadata name = %q, want %q", img.Metadata.Name, "index.docker.io/library/alpine:3.22")
	}
}

func TestChildConfigRoundTrip(t *testing.T) {
	want := childConfig{
		RootFS:   "/tmp/rootfs",
		Hostname: "demo-box",
		Command:  []string{"/bin/echo", "hello"},
	}

	encoded, err := encodeChildConfig(want)
	if err != nil {
		t.Fatalf("encodeChildConfig returned error: %v", err)
	}

	got, err := decodeChildConfig(encoded)
	if err != nil {
		t.Fatalf("decodeChildConfig returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeChildConfig(encodeChildConfig(...)) = %#v, want %#v", got, want)
	}
}

func TestHandleInternalChildIgnoresNonChildArgs(t *testing.T) {
	handled, exitCode := HandleInternalChild(nil)
	if handled {
		t.Fatal("HandleInternalChild(nil) unexpectedly handled args")
	}
	if exitCode != 0 {
		t.Fatalf("HandleInternalChild(nil) exitCode = %d, want 0", exitCode)
	}

	handled, exitCode = HandleInternalChild([]string{"run"})
	if handled {
		t.Fatal("HandleInternalChild(run) unexpectedly handled args")
	}
	if exitCode != 0 {
		t.Fatalf("HandleInternalChild(run) exitCode = %d, want 0", exitCode)
	}

	handled, exitCode = HandleInternalChild([]string{"__childish"})
	if handled {
		t.Fatal("HandleInternalChild(__childish) unexpectedly handled args")
	}
	if exitCode != 0 {
		t.Fatalf("HandleInternalChild(__childish) exitCode = %d, want 0", exitCode)
	}
}

func TestUnsupportedRuntimeErrorMessage(t *testing.T) {
	const want = "mini-docker run requires Linux or WSL2"

	if errRuntimeUnsupported.Error() != want {
		t.Fatalf("errRuntimeUnsupported.Error() = %q, want %q", errRuntimeUnsupported.Error(), want)
	}
}
