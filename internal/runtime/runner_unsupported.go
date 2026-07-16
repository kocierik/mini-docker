//go:build !linux

package runtime

import (
	"os/exec"
)

func runtimeSupported() error {
	return errRuntimeUnsupported
}

func childExecutablePath() string {
	return ""
}

func configureChildCommand(command *exec.Cmd) error {
	return runtimeSupported()
}

func runInternalChild(config childConfig) error {
	return runtimeSupported()
}
