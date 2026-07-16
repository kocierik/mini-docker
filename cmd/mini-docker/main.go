package main

import (
	"os"

	"github.com/ekoci/mini-docker/internal/cli"
	"github.com/ekoci/mini-docker/internal/runtime"
)

func main() {
	if handled, exitCode := runtime.HandleInternalChild(os.Args[1:]); handled {
		os.Exit(exitCode)
	}

	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, cli.Services{}))
}
