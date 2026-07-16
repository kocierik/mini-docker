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
	services, err := newServices(os.Environ())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(dispatchProcess(os.Args[1:], os.Stdout, os.Stderr, runtime.HandleInternalChild, cli.Run, services))
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
