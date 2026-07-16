package main

import (
	"os"

	"github.com/ekoci/mini-docker/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, cli.Services{}))
}
