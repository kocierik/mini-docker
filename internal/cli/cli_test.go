package cli

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ekoci/mini-docker/internal/runtime"
)

type fakePuller struct {
	image string
	err   error
}

func (fake *fakePuller) Pull(_ context.Context, image string) error {
	fake.image = image
	return fake.err
}

type fakeBuilder struct {
	tag     string
	context string
	err     error
}

func (fake *fakeBuilder) Build(_ context.Context, tag, buildContext string) error {
	fake.tag = tag
	fake.context = buildContext
	return fake.err
}

type fakeRunner struct {
	image  string
	config runtime.Config
	err    error
}

func (fake *fakeRunner) Run(_ context.Context, image string, config runtime.Config) error {
	fake.image = image
	fake.config = config
	return fake.err
}

type exitCodeError struct {
	code int
}

func (err exitCodeError) Error() string {
	return "exit status"
}

func (err exitCodeError) ExitCode() int {
	return err.code
}

func TestRunDispatchesPull(t *testing.T) {
	puller := &fakePuller{}
	services := Services{Puller: puller}
	stdout := &strings.Builder{}

	exitCode := Run([]string{"pull", "alpine:3.22"}, stdout, &strings.Builder{}, services)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if puller.image != "alpine:3.22" {
		t.Fatalf("Pull() image = %q, want %q", puller.image, "alpine:3.22")
	}
	const want = "Pulling alpine:3.22...\nPulled alpine:3.22\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunDispatchesBuild(t *testing.T) {
	builder := &fakeBuilder{}
	services := Services{Builder: builder}
	stdout := &strings.Builder{}

	exitCode := Run([]string{"build", "-t", "hello:latest", "./testdata/demo"}, stdout, &strings.Builder{}, services)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if builder.tag != "hello:latest" {
		t.Errorf("Build() tag = %q, want %q", builder.tag, "hello:latest")
	}
	if builder.context != "./testdata/demo" {
		t.Errorf("Build() context = %q, want %q", builder.context, "./testdata/demo")
	}
	const want = "Building hello:latest...\nBuilt hello:latest\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunDispatchesRun(t *testing.T) {
	runner := &fakeRunner{}
	services := Services{Runner: runner}

	exitCode := Run([]string{"run", "--hostname", "demo-box", "hello:latest", "/bin/echo", "hello"}, &strings.Builder{}, &strings.Builder{}, services)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if runner.image != "hello:latest" {
		t.Errorf("Run() image = %q, want %q", runner.image, "hello:latest")
	}
	wantConfig := runtime.Config{Hostname: "demo-box", Command: []string{"/bin/echo", "hello"}}
	if !reflect.DeepEqual(runner.config, wantConfig) {
		t.Errorf("Run() config = %#v, want %#v", runner.config, wantConfig)
	}
}

func TestRunReturnsUsageError(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no command"},
		{name: "unknown command", args: []string{"inspect"}},
		{name: "missing image", args: []string{"run"}},
		{name: "missing build tag", args: []string{"build", "./testdata/demo"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exitCode := Run(test.args, &strings.Builder{}, &strings.Builder{}, Services{})
			if exitCode != 2 {
				t.Fatalf("Run() exit code = %d, want 2", exitCode)
			}
		})
	}
}

func TestRunWritesPublicHelp(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	exitCode := Run([]string{"help"}, stdout, stderr, Services{})

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	const want = "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
}

func TestRunWritesUsageHelpForUsageErrors(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	exitCode := Run(nil, stdout, stderr, Services{})

	if exitCode != 2 {
		t.Fatalf("Run() exit code = %d, want 2", exitCode)
	}
	const want = "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty stdout", stdout.String())
	}
}

func TestRunBuildUsageErrorsWriteOnlyPublicHelp(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	exitCode := Run([]string{"build", "--bad-flag"}, stdout, stderr, Services{})

	if exitCode != 2 {
		t.Fatalf("Run() exit code = %d, want 2", exitCode)
	}
	const want = "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty stdout", stdout.String())
	}
}

func TestRunContainerUsageErrorsWriteOnlyPublicHelp(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	exitCode := Run([]string{"run", "--bad-flag"}, stdout, stderr, Services{})

	if exitCode != 2 {
		t.Fatalf("Run() exit code = %d, want 2", exitCode)
	}
	const want = "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty stdout", stdout.String())
	}
}

func TestRunReportsServiceError(t *testing.T) {
	serviceErr := errors.New("pull failed")
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	services := Services{Puller: &fakePuller{err: serviceErr}}

	exitCode := Run([]string{"pull", "alpine:3.22"}, stdout, stderr, services)

	if exitCode != 1 {
		t.Fatalf("Run() exit code = %d, want 1", exitCode)
	}
	if stderr.String() != "pull failed\n" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "pull failed\n")
	}
	const wantStdout = "Pulling alpine:3.22...\n"
	if stdout.String() != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
	}
}

func TestRunReportsServiceErrorsForAllCommands(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		services Services
		wantErr  string
	}{
		{
			name:     "build",
			args:     []string{"build", "-t", "hello:latest", "./testdata/demo"},
			services: Services{Builder: &fakeBuilder{err: errors.New("build failed")}},
			wantErr:  "build failed\n",
		},
		{
			name:     "run",
			args:     []string{"run", "hello:latest", "/bin/echo", "hello"},
			services: Services{Runner: &fakeRunner{err: errors.New("run failed")}},
			wantErr:  "run failed\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			exitCode := Run(test.args, stdout, stderr, test.services)

			if exitCode != 1 {
				t.Fatalf("Run() exit code = %d, want 1", exitCode)
			}
			if stderr.String() != test.wantErr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.wantErr)
			}
			if test.name == "build" {
				const wantStdout = "Building hello:latest...\n"
				if stdout.String() != wantStdout {
					t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
				}
			}
		})
	}
}

func TestRunPropagatesContainerExitStatus(t *testing.T) {
	stderr := &strings.Builder{}
	services := Services{Runner: &fakeRunner{err: exitCodeError{code: 17}}}

	exitCode := Run([]string{"run", "hello:latest", "/bin/false"}, &strings.Builder{}, stderr, services)

	if exitCode != 17 {
		t.Fatalf("Run() exit code = %d, want 17", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
}
