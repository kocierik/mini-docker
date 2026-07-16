package registry

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/ekoci/mini-docker/internal/image"
)

func TestPullPublishesFlattenedImage(t *testing.T) {
	store := image.NewStore(t.TempDir())
	puller := NewPuller(store)
	puller.fetch = func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
		return &v1.ConfigFile{Config: v1.Config{Cmd: []string{"/bin/demo"}}}, io.NopCloser(bytes.NewReader(makeTestTar(t,
			tarEntry{header: tar.Header{Name: "bin", Typeflag: tar.TypeDir, Mode: 0o755}},
			tarEntry{header: tar.Header{Name: "bin/demo", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len("hello"))}, body: "hello"},
			tarEntry{header: tar.Header{Name: "bin/busybox", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len("busybox"))}, body: "busybox"},
			tarEntry{header: tar.Header{Name: "bin/demo-link", Typeflag: tar.TypeSymlink, Linkname: "demo"}},
			tarEntry{header: tar.Header{Name: "bin/arch", Typeflag: tar.TypeSymlink, Linkname: "/bin/busybox"}},
		))), nil
	}

	if err := puller.Pull(context.Background(), "alpine:3.22"); err != nil {
		t.Fatalf("Pull() error = %v", err)
	}

	got, err := store.Get("index.docker.io/library/alpine:3.22")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Metadata.Name != "index.docker.io/library/alpine:3.22" {
		t.Fatalf("Metadata.Name = %q, want %q", got.Metadata.Name, "index.docker.io/library/alpine:3.22")
	}
	if len(got.Metadata.Command) != 1 || got.Metadata.Command[0] != "/bin/demo" {
		t.Fatalf("Metadata.Command = %#v, want %#v", got.Metadata.Command, []string{"/bin/demo"})
	}
	contents, err := os.ReadFile(filepath.Join(got.RootFS, "bin", "demo"))
	if err != nil {
		t.Fatalf("ReadFile(bin/demo) error = %v", err)
	}
	if string(contents) != "hello" {
		t.Fatalf("bin/demo contents = %q, want %q", string(contents), "hello")
	}
	linkTarget, err := os.Readlink(filepath.Join(got.RootFS, "bin", "demo-link"))
	if err != nil {
		t.Fatalf("Readlink(bin/demo-link) error = %v", err)
	}
	if linkTarget != "demo" {
		t.Fatalf("symlink target = %q, want %q", linkTarget, "demo")
	}
	absoluteLinkTarget, err := os.Readlink(filepath.Join(got.RootFS, "bin", "arch"))
	if err != nil {
		t.Fatalf("Readlink(bin/arch) error = %v", err)
	}
	if absoluteLinkTarget != "/bin/busybox" {
		t.Fatalf("absolute symlink target = %q, want %q", absoluteLinkTarget, "/bin/busybox")
	}
	if _, err := store.Get("alpine:3.22"); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("Get(original reference) error = %v, want ErrNotFound", err)
	}
}

func TestPullFailuresLeaveNoPublishedImage(t *testing.T) {
	fetchErr := errors.New("fetch failed")
	tests := []struct {
		name      string
		reference string
		fetch     func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error)
		wantErr   string
		target    string
	}{
		{
			name:      "empty reference",
			reference: "",
			wantErr:   "image reference is required",
		},
		{
			name:      "fetch failure",
			reference: "alpine:3.22",
			fetch: func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
				return nil, nil, fetchErr
			},
			wantErr: "fetch image",
			target:  "index.docker.io/library/alpine:3.22",
		},
		{
			name:      "malformed tar",
			reference: "alpine:3.22",
			fetch: func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
				return &v1.ConfigFile{}, io.NopCloser(strings.NewReader("not a tar archive")), nil
			},
			wantErr: "extract image",
			target:  "index.docker.io/library/alpine:3.22",
		},
		{
			name:      "absolute path",
			reference: "alpine:3.22",
			fetch: func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
				return &v1.ConfigFile{}, io.NopCloser(bytes.NewReader(makeTestTar(t,
					tarEntry{header: tar.Header{Name: "/etc/passwd", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len("nope"))}, body: "nope"},
				))), nil
			},
			wantErr: "unsafe archive path",
			target:  "index.docker.io/library/alpine:3.22",
		},
		{
			name:      "path traversal",
			reference: "alpine:3.22",
			fetch: func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
				return &v1.ConfigFile{}, io.NopCloser(bytes.NewReader(makeTestTar(t,
					tarEntry{header: tar.Header{Name: "../../escape", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len("nope"))}, body: "nope"},
				))), nil
			},
			wantErr: "unsafe archive path",
			target:  "index.docker.io/library/alpine:3.22",
		},
		{
			name:      "escaping symlink",
			reference: "alpine:3.22",
			fetch: func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
				return &v1.ConfigFile{}, io.NopCloser(bytes.NewReader(makeTestTar(t,
					tarEntry{header: tar.Header{Name: "bin", Typeflag: tar.TypeDir, Mode: 0o755}},
					tarEntry{header: tar.Header{Name: "bin/demo-link", Typeflag: tar.TypeSymlink, Linkname: "../../escape"}},
				))), nil
			},
			wantErr: "unsafe symlink",
			target:  "index.docker.io/library/alpine:3.22",
		},
		{
			name:      "hard link through absolute symlink",
			reference: "alpine:3.22",
			fetch: func(context.Context, name.Reference) (*v1.ConfigFile, io.ReadCloser, error) {
				return &v1.ConfigFile{}, io.NopCloser(bytes.NewReader(makeTestTar(t,
					tarEntry{header: tar.Header{Name: "host", Typeflag: tar.TypeSymlink, Linkname: "/tmp"}},
					tarEntry{header: tar.Header{Name: "escape", Typeflag: tar.TypeLink, Linkname: "host/mini-docker-hard-link-target"}},
				))), nil
			},
			wantErr: "unsafe hard link",
			target:  "index.docker.io/library/alpine:3.22",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := image.NewStore(t.TempDir())
			puller := NewPuller(store)
			if test.fetch != nil {
				puller.fetch = test.fetch
			}

			err := puller.Pull(context.Background(), test.reference)
			if err == nil {
				t.Fatal("Pull() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Pull() error = %q, want substring %q", err.Error(), test.wantErr)
			}
			if test.target != "" {
				if _, getErr := store.Get(test.target); !errors.Is(getErr, image.ErrNotFound) {
					t.Fatalf("Get(%q) error = %v, want ErrNotFound", test.target, getErr)
				}
			}
		})
	}
}

type tarEntry struct {
	header tar.Header
	body   string
}

func makeTestTar(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, entry := range entries {
		header := entry.header
		if header.ModTime.IsZero() {
			header.ModTime = time.Unix(0, 0)
		}
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", header.Name, err)
		}
		if header.Typeflag == tar.TypeReg && entry.body != "" {
			if _, err := io.WriteString(writer, entry.body); err != nil {
				t.Fatalf("Write(%q) error = %v", header.Name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buffer.Bytes()
}
