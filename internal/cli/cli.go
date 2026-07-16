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

func Run(args []string, stdout, stderr io.Writer, services Services) int {
	if len(args) == 0 {
		writeHelp(stderr)
		return 2
	}
	if isHelpRequest(args) {
		writeHelp(stdout)
		return 0
	}

	ctx := context.Background()
	var err error

	switch args[0] {
	case "pull":
		if len(args) != 2 {
			writeHelp(stderr)
			return 2
		}
		err = services.Puller.Pull(ctx, args[1])
	case "build":
		err = runBuild(ctx, args[1:], stderr, services.Builder)
	case "run":
		err = runContainer(ctx, args[1:], stderr, services.Runner)
	default:
		writeHelp(stderr)
		return 2
	}

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

func runBuild(ctx context.Context, args []string, stderr io.Writer, builder Builder) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tag := flags.String("t", "", "image tag")
	if err := flags.Parse(args); err != nil {
		return errUsage
	}
	if *tag == "" || flags.NArg() != 1 {
		return errUsage
	}
	return builder.Build(ctx, *tag, flags.Arg(0))
}

func runContainer(ctx context.Context, args []string, stderr io.Writer, runner Runner) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	hostname := flags.String("hostname", "", "container hostname")
	if err := flags.Parse(args); err != nil {
		return errUsage
	}
	if flags.NArg() == 0 {
		return errUsage
	}

	config := runtime.Config{
		Hostname: *hostname,
		Command:  flags.Args()[1:],
	}
	return runner.Run(ctx, flags.Arg(0), config)
}
