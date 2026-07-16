//go:build linux

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ekoci/mini-docker/internal/image"
)

func TestRunnerCreatesIsolatedProcess(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	rootFS, command := buildHelperRootFS(t)

	store := image.NewStore(filepath.Join(t.TempDir(), "store"))
	if err := store.Put(context.Background(), "integration:latest", rootFS, image.Metadata{
		Name:    "integration:latest",
		Command: command,
	}); err != nil {
		t.Fatalf("store.Put returned error: %v", err)
	}

	runner := NewRunner(store)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	previousStdout := runnerStdout
	previousStderr := runnerStderr
	runnerStdout = stdout
	runnerStderr = stderr
	t.Cleanup(func() {
		runnerStdout = previousStdout
		runnerStderr = previousStderr
	})

	if err := runner.Run(ctx, "integration:latest", Config{Hostname: "demo-box"}); err != nil {
		t.Fatalf("runner.Run returned error: %v\nstderr:\n%s", err, stderr.String())
	}

	var got helperReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode helper output: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	if got.PID != 1 {
		t.Fatalf("PID = %d, want 1", got.PID)
	}
	if got.Hostname != "demo-box" {
		t.Fatalf("hostname = %q, want %q", got.Hostname, "demo-box")
	}
	if !got.ProcMounted {
		t.Fatal("/proc was not mounted inside the container")
	}
	wantRootEntries := []string{"bin", "proc"}
	if !reflect.DeepEqual(got.RootEntries, wantRootEntries) {
		t.Fatalf("root entries = %v, want %v", got.RootEntries, wantRootEntries)
	}
	if got.HostEtcVisible {
		t.Fatal("helper could see host /etc despite isolated rootfs")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

type helperReport struct {
	PID            int      `json:"pid"`
	Hostname       string   `json:"hostname"`
	ProcMounted    bool     `json:"proc_mounted"`
	RootEntries    []string `json:"root_entries"`
	HostEtcVisible bool     `json:"host_etc_visible"`
}

func buildHelperRootFS(t *testing.T) (string, []string) {
	t.Helper()

	workspaceDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd returned error: %v", err)
	}

	helperSourceDir := filepath.Join(t.TempDir(), "helper-src")
	if err := os.MkdirAll(helperSourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll returned error: %v", err)
	}

	const helperSource = `package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

type report struct {
	PID            int      ` + "`json:\"pid\"`" + `
	Hostname       string   ` + "`json:\"hostname\"`" + `
	ProcMounted    bool     ` + "`json:\"proc_mounted\"`" + `
	RootEntries    []string ` + "`json:\"root_entries\"`" + `
	HostEtcVisible bool     ` + "`json:\"host_etc_visible\"`" + `
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	entries, err := os.ReadDir("/")
	if err != nil {
		panic(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	procMounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		panic(err)
	}
	_, err = os.Stat("/etc")
	report := report{
		PID:            os.Getpid(),
		Hostname:       hostname,
		ProcMounted:    strings.Contains(string(procMounts), "proc /proc proc"),
		RootEntries:    names,
		HostEtcVisible: err == nil,
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		panic(err)
	}
}
`

	helperSourcePath := filepath.Join(helperSourceDir, "main.go")
	if err := os.WriteFile(helperSourcePath, []byte(helperSource), 0o644); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	rootFS := filepath.Join(t.TempDir(), "rootfs")
	helperBinary := filepath.Join(rootFS, "bin", "helper")
	if err := os.MkdirAll(filepath.Dir(helperBinary), 0o755); err != nil {
		t.Fatalf("os.MkdirAll returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rootFS, "proc"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll returned error: %v", err)
	}

	command := exec.Command("go", "build", "-o", helperBinary, helperSourcePath)
	command.Dir = workspaceDir
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build helper binary: %v\n%s", err, string(output))
	}

	return rootFS, []string{"/bin/helper"}
}
