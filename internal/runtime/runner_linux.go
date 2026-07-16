//go:build linux

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func runtimeSupported() error {
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

func runInternalChild(config childConfig) error {
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mount propagation private: %w", err)
	}
	if config.Hostname != "" {
		if err := syscall.Sethostname([]byte(config.Hostname)); err != nil {
			return fmt.Errorf("set hostname: %w", err)
		}
	}
	if err := syscall.Chroot(config.RootFS); err != nil {
		return fmt.Errorf("chroot into rootfs: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change directory to new root: %w", err)
	}
	if err := os.MkdirAll("/proc", 0o555); err != nil {
		return fmt.Errorf("create /proc mountpoint: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	defer syscall.Unmount("/proc", 0)

	if err := syscall.Exec(config.Command[0], config.Command, os.Environ()); err != nil {
		return fmt.Errorf("exec command: %w", err)
	}
	return nil
}

var _ = context.Background
