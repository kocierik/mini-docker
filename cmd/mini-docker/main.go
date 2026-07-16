package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ekoci/mini-docker/internal/builder"
	"github.com/ekoci/mini-docker/internal/cli"
	"github.com/ekoci/mini-docker/internal/image"
	"github.com/ekoci/mini-docker/internal/registry"
	"github.com/ekoci/mini-docker/internal/runtime"
)

func main() {
	os.Exit(runProcess(os.Args[1:], os.Stdout, os.Stderr, os.Environ(), runtime.HandleInternalChild, newServices, nil))
}

func runProcess(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	environ []string,
	handleChild func([]string) (bool, int),
	buildServices func([]string) (cli.Services, error),
	runCLI func([]string, io.Writer, io.Writer, cli.Services) int,
) int {
	if handled, exitCode := handleChild(args); handled {
		return exitCode
	}

	invocation, handled, exitCode := cli.Prepare(args, stdout, stderr)
	if handled {
		return exitCode
	}

	services, err := buildServices(environ)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if runCLI == nil {
		return invocation.Run(stderr, services)
	}
	return runCLI(args, stdout, stderr, services)
}

func dispatchProcess(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	handleChild func([]string) (bool, int),
	runCLI func([]string, io.Writer, io.Writer, cli.Services) int,
	services cli.Services,
) int {
	if handled, exitCode := handleChild(args); handled {
		return exitCode
	}
	return runCLI(args, stdout, stderr, services)
}

func newServices(environ []string) (cli.Services, error) {
	storeRoot, err := resolveStoreRoot(envMap(environ))
	if err != nil {
		return cli.Services{}, err
	}

	store := image.NewStore(storeRoot)
	runner := runtime.NewRunner(store)
	puller := registry.NewPuller(store)
	builderService := builder.New(store, nil)

	return cli.Services{
		Puller:  puller,
		Builder: builderService,
		Runner:  runner,
	}, nil
}

func resolveStoreRoot(env map[string]string) (string, error) {
	if root := env["MINI_DOCKER_HOME"]; root != "" {
		return root, nil
	}

	home := env["HOME"]
	if home == "" {
		return "", fmt.Errorf("HOME is not set")
	}
	return filepath.Join(home, ".mini-docker", "images"), nil
}

func envMap(environ []string) map[string]string {
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		for index := 0; index < len(entry); index++ {
			if entry[index] != '=' {
				continue
			}
			values[entry[:index]] = entry[index+1:]
			break
		}
	}
	return values
}
