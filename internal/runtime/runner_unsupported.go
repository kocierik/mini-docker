//go:build !linux

package runtime

import (
	"fmt"
	"os/exec"
)

func runtimeSupported() error {
	return errRuntimeUnsupported
}

func runtimePrivileges() error {
	return nil
}

func childExecutablePath() string {
	return ""
}

func configureChildCommand(command *exec.Cmd) error {
	return runtimeSupported()
}

func translateRuntimeProcessError(err error) error {
	return fmt.Errorf("run container process: %w", err)
}

func runInternalChild(config childConfig) error {
	return runtimeSupported()
}
