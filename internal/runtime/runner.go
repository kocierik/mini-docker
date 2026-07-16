package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ekoci/mini-docker/internal/image"
)

const (
	internalChildArgument = "__child"
	childConfigEnvName    = "MINI_DOCKER_CHILD_CONFIG"
)

var (
	errMissingCommand           = errors.New("image has no CMD and no command override was provided")
	runnerStdin       io.Reader = os.Stdin
	runnerStdout      io.Writer = os.Stdout
	runnerStderr      io.Writer = os.Stderr
)

type Runner struct {
	store *image.Store
}

type childConfig struct {
	RootFS   string   `json:"rootfs"`
	Hostname string   `json:"hostname,omitempty"`
	Command  []string `json:"command"`
}

func NewRunner(store *image.Store) *Runner {
	return &Runner{store: store}
}

func (runner *Runner) Run(ctx context.Context, imageName string, config Config) error {
	if err := runtimeSupported(); err != nil {
		return err
	}

	img, err := runner.store.Get(imageName)
	if err != nil {
		return err
	}

	command, err := resolveCommand(img.Metadata, config)
	if err != nil {
		return err
	}

	temporaryRootFS, err := cloneRootFS(img.RootFS)
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRootFS)

	return RunRootFS(ctx, Config{
		RootFS:   temporaryRootFS,
		Hostname: config.Hostname,
		Command:  command,
	})
}

func RunRootFS(ctx context.Context, config Config) error {
	if err := runtimeSupported(); err != nil {
		return err
	}
	if config.RootFS == "" {
		return fmt.Errorf("rootfs is required")
	}
	if len(config.Command) == 0 {
		return errMissingCommand
	}

	return runRootFS(ctx, childConfig{
		RootFS:   config.RootFS,
		Hostname: config.Hostname,
		Command:  append([]string(nil), config.Command...),
	})
}

func HandleInternalChild(args []string) (bool, int) {
	if len(args) != 1 || args[0] != internalChildArgument {
		return false, 0
	}

	encodedConfig := os.Getenv(childConfigEnvName)
	config, err := decodeChildConfig(encodedConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}

	if err := runInternalChild(config); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}

	return true, 0
}

func resolveCommand(metadata image.Metadata, config Config) ([]string, error) {
	if len(config.Command) > 0 {
		return append([]string(nil), config.Command...), nil
	}
	if len(metadata.Command) > 0 {
		return append([]string(nil), metadata.Command...), nil
	}
	return nil, errMissingCommand
}

func encodeChildConfig(config childConfig) (string, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode child config: %w", err)
	}
	return string(encoded), nil
}

func decodeChildConfig(encoded string) (childConfig, error) {
	if encoded == "" {
		return childConfig{}, fmt.Errorf("%s is not set", childConfigEnvName)
	}

	var config childConfig
	if err := json.Unmarshal([]byte(encoded), &config); err != nil {
		return childConfig{}, fmt.Errorf("decode child config: %w", err)
	}
	if config.RootFS == "" {
		return childConfig{}, fmt.Errorf("child config rootfs is required")
	}
	if len(config.Command) == 0 {
		return childConfig{}, errMissingCommand
	}
	return config, nil
}

func runRootFS(ctx context.Context, config childConfig) error {
	encodedConfig, err := encodeChildConfig(config)
	if err != nil {
		return err
	}

	command := exec.CommandContext(ctx, childExecutablePath(), internalChildArgument)
	command.Stdin = runnerStdin
	command.Stdout = runnerStdout
	command.Stderr = runnerStderr
	command.Env = append(os.Environ(), childConfigEnvName+"="+encodedConfig)

	if err := configureChildCommand(command); err != nil {
		return err
	}

	if err := command.Run(); err != nil {
		return fmt.Errorf("run container process: %w", err)
	}
	return nil
}

func cloneRootFS(sourceRootFS string) (string, error) {
	temporaryParent, err := os.MkdirTemp("", "mini-docker-run-*")
	if err != nil {
		return "", fmt.Errorf("create temporary runtime directory: %w", err)
	}

	destinationRootFS := filepath.Join(temporaryParent, "rootfs")
	if err := copyTree(sourceRootFS, destinationRootFS); err != nil {
		_ = os.RemoveAll(temporaryParent)
		return "", fmt.Errorf("clone image rootfs: %w", err)
	}
	return destinationRootFS, nil
}

func copyTree(sourceRoot, destinationRoot string) error {
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(destinationRoot, info.Mode().Perm())
		}

		destinationPath := filepath.Join(destinationRoot, relativePath)
		info, err := entry.Info()
		if err != nil {
			return err
		}

		switch mode := info.Mode(); {
		case mode.IsDir():
			return os.MkdirAll(destinationPath, mode.Perm())
		case mode.IsRegular():
			return copyRegularFile(path, destinationPath, mode)
		case mode&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, destinationPath)
		default:
			return fmt.Errorf("unsupported rootfs entry type at %q", relativePath)
		}
	})
}

func copyRegularFile(sourcePath, destinationPath string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return destination.Chmod(mode.Perm())
}
