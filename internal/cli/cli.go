package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/ekoci/mini-docker/internal/runtime"
)

type Puller interface {
	Pull(context.Context, string) error
}

type Builder interface {
	Build(context.Context, string, string) error
}

type Runner interface {
	Run(context.Context, string, runtime.Config) error
}

type Services struct {
	Puller  Puller
	Builder Builder
	Runner  Runner
}

const publicHelp = "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"

type Invocation struct {
	execute func(Services) error
}

func Run(args []string, stdout, stderr io.Writer, services Services) int {
	invocation, handled, exitCode := Prepare(args, stdout, stderr)
	if handled {
		return exitCode
	}
	return invocation.Run(stderr, services)
}

func Prepare(args []string, stdout, stderr io.Writer) (Invocation, bool, int) {
	if len(args) == 0 {
		writeHelp(stderr)
		return Invocation{}, true, 2
	}
	if isHelpRequest(args) {
		writeHelp(stdout)
		return Invocation{}, true, 0
	}

	ctx := context.Background()

	switch args[0] {
	case "pull":
		if len(args) != 2 {
			writeHelp(stderr)
			return Invocation{}, true, 2
		}
		image := args[1]
		return Invocation{execute: func(services Services) error {
			fmt.Fprintf(stdout, "Pulling %s...\n", image)
			if err := services.Puller.Pull(ctx, image); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Pulled %s\n", image)
			return nil
		}}, false, 0
	case "build":
		tag, buildContext, ok := parseBuildArgs(args[1:])
		if !ok {
			writeHelp(stderr)
			return Invocation{}, true, 2
		}
		return Invocation{execute: func(services Services) error {
			fmt.Fprintf(stdout, "Building %s...\n", tag)
			if err := services.Builder.Build(ctx, tag, buildContext); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Built %s\n", tag)
			return nil
		}}, false, 0
	case "run":
		image, config, ok := parseRunArgs(args[1:])
		if !ok {
			writeHelp(stderr)
			return Invocation{}, true, 2
		}
		return Invocation{execute: func(services Services) error {
			return services.Runner.Run(ctx, image, config)
		}}, false, 0
	default:
		writeHelp(stderr)
		return Invocation{}, true, 2
	}

}

func (invocation Invocation) Run(stderr io.Writer, services Services) int {
	err := invocation.execute(services)
	if err == errUsage {
		writeHelp(stderr)
		return 2
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func isHelpRequest(args []string) bool {
	return len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help")
}

func writeHelp(writer io.Writer) {
	_, _ = io.WriteString(writer, publicHelp)
}

var errUsage = fmt.Errorf("usage error")

func parseBuildArgs(args []string) (string, string, bool) {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tag := flags.String("t", "", "image tag")
	if err := flags.Parse(args); err != nil {
		return "", "", false
	}
	if *tag == "" || flags.NArg() != 1 {
		return "", "", false
	}
	return *tag, flags.Arg(0), true
}

func parseRunArgs(args []string) (string, runtime.Config, bool) {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	hostname := flags.String("hostname", "", "container hostname")
	if err := flags.Parse(args); err != nil {
		return "", runtime.Config{}, false
	}
	if flags.NArg() == 0 {
		return "", runtime.Config{}, false
	}

	config := runtime.Config{
		Hostname: *hostname,
		Command:  flags.Args()[1:],
	}
	return flags.Arg(0), config, true
}
