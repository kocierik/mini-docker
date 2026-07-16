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
	preservedModeMask     = fs.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
)

var (
	errMissingCommand               = errors.New("image has no CMD and no command override was provided")
	errRuntimeUnsupported           = errors.New("mini-docker run requires Linux or WSL2")
	errRuntimePrivileges            = errors.New("insufficient privileges: mini-docker run and Dockerfile RUN require root")
	runnerStdin           io.Reader = os.Stdin
	runnerStdout          io.Writer = os.Stdout
	runnerStderr          io.Writer = os.Stderr
)

type exitCodeCarrier struct {
	code int
	err  error
}

func (err *exitCodeCarrier) Error() string {
	return err.err.Error()
}

func (err *exitCodeCarrier) Unwrap() error {
	return err.err
}

func (err *exitCodeCarrier) ExitCode() int {
	return err.code
}

type Runner struct {
	store *image.Store
}

type childConfig struct {
	RootFS   string   `json:"rootfs"`
	Hostname string   `json:"hostname,omitempty"`
	Command  []string `json:"command"`
}

type directoryMode struct {
	path string
	mode fs.FileMode
}

type fileIdentity struct {
	device uint64
	inode  uint64
}

func NewRunner(store *image.Store) *Runner {
	return &Runner{store: store}
}

func (runner *Runner) Run(ctx context.Context, imageName string, config Config) error {
	if err := runtimeSupported(); err != nil {
		return err
	}

	img, err := runner.resolveImage(imageName)
	if err != nil {
		return err
	}

	command, err := resolveCommand(img.Metadata, config)
	if err != nil {
		return err
	}
	if err := runtimePrivileges(); err != nil {
		return err
	}

	temporaryRootFS, err := cloneRootFS(img.RootFS)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(temporaryRootFS))

	return RunRootFS(ctx, Config{
		RootFS:   temporaryRootFS,
		Hostname: config.Hostname,
		Command:  command,
	})
}

func (runner *Runner) resolveImage(name string) (image.Image, error) {
	return runner.store.Resolve(name)
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
	if err := runtimePrivileges(); err != nil {
		return err
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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &exitCodeCarrier{
				code: exitErr.ExitCode(),
				err:  fmt.Errorf("run container process: %w", err),
			}
		}
		return translateRuntimeProcessError(err)
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
	directories := make([]directoryMode, 0)
	hardLinks := make(map[fileIdentity]string)
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
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
			if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
				return err
			}
			directories = append(directories, directoryMode{path: destinationRoot, mode: info.Mode()})
			return nil
		}

		destinationPath := filepath.Join(destinationRoot, relativePath)
		info, err := entry.Info()
		if err != nil {
			return err
		}

		switch mode := info.Mode(); {
		case mode.IsDir():
			if err := os.MkdirAll(destinationPath, 0o700); err != nil {
				return err
			}
			directories = append(directories, directoryMode{path: destinationPath, mode: mode})
			return nil
		case mode.IsRegular():
			identity, linkCount, err := hardLinkIdentityAtPath(path)
			if err != nil {
				return err
			}
			if linkCount > 1 {
				if existingPath, ok := hardLinks[identity]; ok {
					return os.Link(existingPath, destinationPath)
				}
			}
			if err := copyRegularFile(path, destinationPath, mode); err != nil {
				return err
			}
			if linkCount > 1 {
				hardLinks[identity] = destinationPath
			}
			return nil
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
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := os.Chmod(directories[index].path, directories[index].mode&preservedModeMask); err != nil {
			return err
		}
	}
	return nil
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

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destinationPath, mode&preservedModeMask)
}
