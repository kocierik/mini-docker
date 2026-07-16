package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ekoci/mini-docker/internal/cli"
)

func TestResolveStoreRootUsesMiniDockerHome(t *testing.T) {
	storeRoot, err := resolveStoreRoot(map[string]string{
		"MINI_DOCKER_HOME": "/tmp/custom-store",
		"HOME":             "/home/erik",
	})
	if err != nil {
		t.Fatalf("resolveStoreRoot() error = %v", err)
	}
	if storeRoot != "/tmp/custom-store" {
		t.Fatalf("resolveStoreRoot() = %q, want %q", storeRoot, "/tmp/custom-store")
	}
}

func TestResolveStoreRootDefaultsUnderHome(t *testing.T) {
	storeRoot, err := resolveStoreRoot(map[string]string{"HOME": "/home/erik"})
	if err != nil {
		t.Fatalf("resolveStoreRoot() error = %v", err)
	}
	if storeRoot != "/home/erik/.mini-docker/images" {
		t.Fatalf("resolveStoreRoot() = %q, want %q", storeRoot, "/home/erik/.mini-docker/images")
	}
}

func TestDispatchProcessHandlesInternalChildBeforeCLI(t *testing.T) {
	cliCalled := false
	exitCode := dispatchProcess(
		[]string{"__child"},
		io.Discard,
		io.Discard,
		func(args []string) (bool, int) {
			if len(args) != 1 || args[0] != "__child" {
				t.Fatalf("child handler args = %#v, want __child", args)
			}
			return true, 23
		},
		func([]string, io.Writer, io.Writer, cli.Services) int {
			cliCalled = true
			return 0
		},
		cli.Services{},
	)

	if exitCode != 23 {
		t.Fatalf("dispatchProcess() exit code = %d, want 23", exitCode)
	}
	if cliCalled {
		t.Fatal("dispatchProcess() called CLI after internal child was handled")
	}
}

func TestDispatchProcessForwardsPublicArgsToCLI(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	services := cli.Services{}

	exitCode := dispatchProcess(
		[]string{"pull", "alpine:3.22"},
		stdout,
		stderr,
		func([]string) (bool, int) { return false, 0 },
		func(args []string, gotStdout, gotStderr io.Writer, gotServices cli.Services) int {
			if len(args) != 2 || args[0] != "pull" || args[1] != "alpine:3.22" {
				t.Fatalf("CLI args = %#v, want pull alpine:3.22", args)
			}
			if gotStdout != stdout {
				t.Fatal("dispatchProcess() did not forward stdout")
			}
			if gotStderr != stderr {
				t.Fatal("dispatchProcess() did not forward stderr")
			}
			if gotServices != services {
				t.Fatal("dispatchProcess() did not forward services")
			}
			return 7
		},
		services,
	)

	if exitCode != 7 {
		t.Fatalf("dispatchProcess() exit code = %d, want 7", exitCode)
	}
}

func TestRunProcessHandlesInternalChildBeforeBuildingServices(t *testing.T) {
	serviceFactoryCalled := false
	cliCalled := false

	exitCode := runProcess(
		[]string{"__child"},
		io.Discard,
		io.Discard,
		nil,
		func(args []string) (bool, int) {
			if len(args) != 1 || args[0] != "__child" {
				t.Fatalf("child handler args = %#v, want __child", args)
			}
			return true, 19
		},
		func([]string) (cli.Services, error) {
			serviceFactoryCalled = true
			return cli.Services{}, nil
		},
		func([]string, io.Writer, io.Writer, cli.Services) int {
			cliCalled = true
			return 0
		},
	)

	if exitCode != 19 {
		t.Fatalf("runProcess() exit code = %d, want 19", exitCode)
	}
	if serviceFactoryCalled {
		t.Fatal("runProcess() built services for internal child")
	}
	if cliCalled {
		t.Fatal("runProcess() called CLI for internal child")
	}
}

func TestRunProcessReportsServiceFactoryError(t *testing.T) {
	stderr := &strings.Builder{}
	cliCalled := false

	exitCode := runProcess(
		[]string{"pull", "alpine:3.22"},
		io.Discard,
		stderr,
		nil,
		func([]string) (bool, int) { return false, 0 },
		func([]string) (cli.Services, error) {
			return cli.Services{}, errors.New("HOME is not set")
		},
		func([]string, io.Writer, io.Writer, cli.Services) int {
			cliCalled = true
			return 0
		},
	)

	if exitCode != 1 {
		t.Fatalf("runProcess() exit code = %d, want 1", exitCode)
	}
	if stderr.String() != "HOME is not set\n" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "HOME is not set\n")
	}
	if cliCalled {
		t.Fatal("runProcess() called CLI after service factory error")
	}
}

func TestRunProcessPrintsTopLevelHelpWithoutEnvironment(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "help", args: []string{"help"}},
		{name: "short flag", args: []string{"-h"}},
		{name: "long flag", args: []string{"--help"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			exitCode := runProcess(
				test.args,
				stdout,
				stderr,
				nil,
				func([]string) (bool, int) { return false, 0 },
				newServices,
				cli.Run,
			)

			if exitCode != 0 {
				t.Fatalf("runProcess() exit code = %d, want 0", exitCode)
			}
			const want = "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
			if stdout.String() != want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty stderr", stderr.String())
			}
		})
	}
}

func TestRunProcessPrintsPublicUsageForInvalidInvocationWithoutEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{
			name:     "no command",
			args:     nil,
			wantCode: 2,
			wantOut:  "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n",
		},
		{
			name:     "unknown command",
			args:     []string{"inspect"},
			wantCode: 2,
			wantOut:  "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n",
		},
		{
			name:     "pull missing image",
			args:     []string{"pull"},
			wantCode: 2,
			wantOut:  "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n",
		},
		{
			name:     "build missing args",
			args:     []string{"build", "-t", "demo:latest"},
			wantCode: 2,
			wantOut:  "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n",
		},
		{
			name:     "run missing image",
			args:     []string{"run", "--hostname", "demo-box"},
			wantCode: 2,
			wantOut:  "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			exitCode := runProcess(
				test.args,
				stdout,
				stderr,
				nil,
				func([]string) (bool, int) { return false, 0 },
				newServices,
				cli.Run,
			)

			if exitCode != test.wantCode {
				t.Fatalf("runProcess() exit code = %d, want %d", exitCode, test.wantCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty stdout", stdout.String())
			}
			if stderr.String() != test.wantOut {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.wantOut)
			}
		})
	}
}
