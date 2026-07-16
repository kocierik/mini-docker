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

func TestRunDispatchesPull(t *testing.T) {
	puller := &fakePuller{}
	services := Services{Puller: puller}

	exitCode := Run([]string{"pull", "alpine:3.22"}, &strings.Builder{}, &strings.Builder{}, services)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if puller.image != "alpine:3.22" {
		t.Fatalf("Pull() image = %q, want %q", puller.image, "alpine:3.22")
	}
}

func TestRunDispatchesBuild(t *testing.T) {
	builder := &fakeBuilder{}
	services := Services{Builder: builder}

	exitCode := Run([]string{"build", "-t", "hello:latest", "./testdata/demo"}, &strings.Builder{}, &strings.Builder{}, services)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if builder.tag != "hello:latest" {
		t.Errorf("Build() tag = %q, want %q", builder.tag, "hello:latest")
	}
	if builder.context != "./testdata/demo" {
		t.Errorf("Build() context = %q, want %q", builder.context, "./testdata/demo")
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

func TestRunReportsServiceError(t *testing.T) {
	serviceErr := errors.New("pull failed")
	stderr := &strings.Builder{}
	services := Services{Puller: &fakePuller{err: serviceErr}}

	exitCode := Run([]string{"pull", "alpine:3.22"}, &strings.Builder{}, stderr, services)

	if exitCode != 1 {
		t.Fatalf("Run() exit code = %d, want 1", exitCode)
	}
	if stderr.String() != "pull failed\n" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "pull failed\n")
	}
}
