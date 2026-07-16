//go:build linux

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func runtimeSupported() error {
	return nil
}

func runtimePrivileges() error {
	if os.Geteuid() != 0 {
		return errRuntimePrivileges
	}
	return nil
}

func childExecutablePath() string {
	return "/proc/self/exe"
}

func configureChildCommand(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,
	}
	return nil
}

func translateRuntimeProcessError(err error) error {
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return errRuntimePrivileges
	}
	return fmt.Errorf("run container process: %w", err)
}

func runInternalChild(config childConfig) error {
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("failed to make mount propagation private: %w", err)
	}
	if config.Hostname != "" {
		if err := syscall.Sethostname([]byte(config.Hostname)); err != nil {
			return fmt.Errorf("failed to set hostname: %w", err)
		}
	}
	if err := syscall.Chroot(config.RootFS); err != nil {
		return fmt.Errorf("failed to chroot into rootfs: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to change directory to /: %w", err)
	}
	if err := os.MkdirAll("/proc", 0o555); err != nil {
		return fmt.Errorf("failed to create /proc mountpoint: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("failed to mount /proc: %w", err)
	}
	defer syscall.Unmount("/proc", 0)

	if err := syscall.Exec(config.Command[0], config.Command, os.Environ()); err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return fmt.Errorf("command not found inside root filesystem: %s", config.Command[0])
		}
		return fmt.Errorf("failed to exec command: %w", err)
	}
	return nil
}

var _ = context.Background
