//go:build !linux

package runtime

import (
	"fmt"
	"os/exec"
)

func runtimeSupported() error {
	return fmt.Errorf("mini-docker run requires Linux or WSL2")
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
