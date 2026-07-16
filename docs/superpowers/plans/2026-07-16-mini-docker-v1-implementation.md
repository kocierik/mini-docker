# Mini Docker v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that pulls a public `linux/amd64` image, builds a new image from a four-instruction Dockerfile subset, and runs it with real Linux PID, UTS, and mount isolation.

**Architecture:** A thin CLI dispatches to registry, builder, image-store, and runtime packages. Images are flattened root filesystems with JSON metadata under `$HOME/.mini-docker/images`; pull uses `go-containerregistry`, build applies instructions sequentially, and run uses a hidden re-exec entry point to enter Linux namespaces before `chroot` and `exec`.

**Tech Stack:** Go 1.24+, Go standard library, `github.com/google/go-containerregistry`, Linux namespaces and mount syscalls, WSL2 or native Linux for runtime verification.

## Global Constraints

- Public commands are exactly `pull`, `build`, and `run`; `__child` is internal and undocumented.
- The only supported image platform is `linux/amd64`.
- Dockerfiles support one leading `FROM`, plus `COPY`, shell-form `RUN`, and JSON-array `CMD` only.
- Builds are single-stage and have no cache.
- The local store defaults to `$HOME/.mini-docker/images` and stores a flattened `rootfs` plus JSON metadata.
- Store publication uses a temporary directory and atomic rename.
- Runtime isolation uses PID, UTS, and mount namespaces, `chroot`, and a private `/proc` mount.
- No push, private registry authentication, networking, cgroups, volumes, overlay filesystem, or multi-platform support.

## File Structure

```text
go.mod                                      module and dependencies
cmd/mini-docker/main.go                     process entry point and service wiring
internal/cli/cli.go                         command parsing and dispatch
internal/cli/cli_test.go                    CLI contract tests
internal/image/metadata.go                  image metadata schema
internal/image/store.go                     atomic local image store
internal/image/store_test.go                store behavior tests
internal/registry/puller.go                 OCI pull and flattening
internal/registry/puller_test.go            pull orchestration tests
internal/dockerfile/parser.go               restricted Dockerfile parser
internal/dockerfile/parser_test.go          parser contract tests
internal/builder/builder.go                  sequential image construction
internal/builder/copy.go                     build-context-safe COPY
internal/builder/builder_test.go             builder and traversal tests
internal/runtime/config.go                   runtime configuration
internal/runtime/runner.go                   public runner and child dispatch
internal/runtime/runner_linux.go             Linux isolation implementation
internal/runtime/runner_unsupported.go       non-Linux explanatory failure
internal/runtime/runner_test.go              portable runtime tests
internal/runtime/runner_linux_test.go        privileged Linux test
testdata/demo/Dockerfile                     speech demo image
testdata/demo/hello.sh                       speech demo command
README.md                                   setup, demo, and limitations
```

---

### Task 1: CLI Skeleton and Contracts

**Files:**
- Create: `go.mod`
- Create: `cmd/mini-docker/main.go`
- Create: `internal/cli/cli.go`
- Create: `internal/cli/cli_test.go`
- Create: `internal/runtime/config.go`

**Interfaces:**
- Produces: `cli.Services`, `cli.Run(args []string, stdout, stderr io.Writer, services Services) int`
- Produces: `runtime.Config{RootFS string, Hostname string, Command []string}`
- Produces: `Puller.Pull`, `Builder.Build`, and `Runner.Run` service contracts

- [ ] **Step 1: Write failing dispatch tests**

Create `go.mod` with module `github.com/ekoci/mini-docker` and `go 1.24`. In `cli_test.go`, use fakes implementing:

```go
type Puller interface { Pull(context.Context, string) error }
type Builder interface { Build(context.Context, string, string) error }
type Runner interface { Run(context.Context, string, runtime.Config) error }
```

Assert exact dispatch for `pull alpine:3.22`, `build -t hello:latest ./testdata/demo`, and `run --hostname demo-box hello:latest /bin/echo hello`. Assert no command, unknown command, missing image, and missing build tag return `2`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cli -v`

Expected: FAIL because the packages do not exist.

- [ ] **Step 3: Implement the parser and contracts**

Use `flag.NewFlagSet(..., flag.ContinueOnError)` separately for `build` and `run`. Return `2` for usage errors, `1` for service errors, and `0` for success. Write service errors as one line to `stderr`. Add:

```go
type Config struct {
	RootFS  string
	Hostname string
	Command []string
}
```

Create a temporary `main.go` that calls `cli.Run`; production services are wired in Task 7.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/cli -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/mini-docker internal/cli internal/runtime/config.go
git commit -m "feat: add mini docker CLI contracts"
```

---

### Task 2: Atomic Local Image Store

**Files:**
- Create: `internal/image/metadata.go`
- Create: `internal/image/store.go`
- Create: `internal/image/store_test.go`

**Interfaces:**
- Produces: `Metadata{Name string, Command []string}` and `Image{RootFS string, Metadata Metadata}`
- Produces: `NewStore(root string) *Store`, `(*Store).Put(ctx, name, sourceRootFS, metadata) error`, `(*Store).Get(name) (Image, error)`, and `ErrNotFound`

- [ ] **Step 1: Write failing store tests**

Using `t.TempDir`, assert `Put` copies `etc/demo`, preserves executable bits and symlinks, writes metadata, and `Get` returns both. Add cases for unknown images, replacement of an existing tag, canceled context, and names such as `../escape` not escaping the store.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/image -v`

Expected: FAIL because the image package is missing.

- [ ] **Step 3: Implement the store**

Serialize metadata as JSON. `Put` creates an immutable `<root>/objects/.tmp-*`, copies into `rootfs`, writes `metadata.json`, and renames it to a generated object ID. It then writes a temporary `<root>/refs/<sha256(name)>` file containing that object ID and atomically renames the reference file into place. `Get` resolves the reference before opening the object. Check `ctx.Err()` during `filepath.WalkDir`. Preserve directories, regular-file modes, and relative symlinks; reject sockets, device nodes, absolute symlinks, and escaping symlink targets. This indirection makes tag replacement atomic on Linux; unreferenced old objects may remain until a future garbage-collection feature.

- [ ] **Step 4: Run store tests**

Run: `go test ./internal/image -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/image
git commit -m "feat: add atomic local image store"
```

---

### Task 3: Public OCI Pull

**Files:**
- Modify: `go.mod`
- Create: `internal/registry/puller.go`
- Create: `internal/registry/puller_test.go`

**Interfaces:**
- Consumes: `image.Store.Put`
- Produces: `NewPuller(store *image.Store) *Puller` and `(*Puller).Pull(ctx, reference) error`

- [ ] **Step 1: Write a failing offline pull test**

Inject a package-private fetch function returning OCI config plus a flattened tar stream. Assert a tar containing `bin/demo` and `Cmd: ["/bin/demo"]` is published correctly. Add empty-reference, fetch-failure, malformed-tar, absolute-path, traversal, and escaping-symlink cases; every failure must leave no target image.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/registry -v`

Expected: FAIL because the registry package is missing.

- [ ] **Step 3: Implement pull and safe extraction**

Run `go get github.com/google/go-containerregistry@latest`. Production fetch uses `name.ParseReference`, `remote.Image` with:

```go
remote.WithPlatform(v1.Platform{OS: "linux", Architecture: "amd64"})
```

Use `mutate.Extract` so OCI layers and whiteouts become one tar stream. Extract only directories, regular files, and safe links beneath a temporary rootfs. Publish normalized image name and OCI `Config.Cmd` through the store. Do not implement login or credential persistence.

- [ ] **Step 4: Verify pull tests and dependencies**

```bash
go mod tidy
go test ./internal/registry ./internal/image -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/registry
git commit -m "feat: pull public OCI images"
```

---

### Task 4: Restricted Dockerfile Parser

**Files:**
- Create: `internal/dockerfile/parser.go`
- Create: `internal/dockerfile/parser_test.go`

**Interfaces:**
- Produces: `Instruction` plus `From`, `Copy`, `Run`, and `Cmd` values
- Produces: `Parse(reader io.Reader) ([]Instruction, error)`

- [ ] **Step 1: Write failing parser tests**

Assert exact parsing of:

```dockerfile
FROM alpine:3.22
COPY hello.sh /hello.sh
RUN chmod +x /hello.sh
CMD ["/hello.sh"]
```

Reject empty input, instruction before `FROM`, duplicate `FROM`, shell-form `CMD`, JSON `RUN`, multiple COPY sources, relative COPY destination, `ENV`, malformed JSON, and empty CMD arrays.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/dockerfile -v`

Expected: FAIL because `Parse` is missing.

- [ ] **Step 3: Implement strict parsing**

Define:

```go
type Instruction interface{ instruction() }
type From struct{ Image string }
type Copy struct{ Source, Destination string }
type Run struct{ Command string }
type Cmd struct{ Command []string }
```

Use `bufio.Scanner`, ignore blank and full-comment lines, and include source line numbers in errors. Parse CMD with `encoding/json`. Do not support continuations, variables, escapes, or inline comments.

- [ ] **Step 4: Verify parser tests**

Run: `go test ./internal/dockerfile -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dockerfile
git commit -m "feat: parse minimal Dockerfiles"
```

---

### Task 5: Linux Runtime with Hidden Re-exec

**Files:**
- Create: `internal/runtime/runner.go`
- Create: `internal/runtime/runner_linux.go`
- Create: `internal/runtime/runner_unsupported.go`
- Create: `internal/runtime/runner_test.go`
- Create: `internal/runtime/runner_linux_test.go`

**Interfaces:**
- Consumes: `runtime.Config` and `image.Store.Get`
- Produces: `NewRunner(store *image.Store) *Runner`, `Run(ctx, imageName, config) error`, `RunRootFS(ctx, config) error`, and `HandleInternalChild(args) (bool, int)`

- [ ] **Step 1: Write failing portable tests**

Test that explicit `Config.Command` overrides image CMD, image CMD is the fallback, and two empty commands return `image has no CMD and no command override was provided`. Test child-config JSON round-trip and that arguments other than `__child` are not handled internally.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/runtime -run 'TestResolveCommand|TestChildConfig' -v`

Expected: FAIL because runner helpers are missing.

- [ ] **Step 3: Implement parent re-exec and platform split**

The parent copies the immutable image rootfs to a temporary writable rootfs, encodes child config in `MINI_DOCKER_CHILD_CONFIG`, and starts `/proc/self/exe __child` with attached stdio. On Linux set:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
	Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,
}
```

The child makes mount propagation private, sets hostname, calls `chroot`, changes to `/`, creates and mounts `/proc` with `MS_NOSUID|MS_NOEXEC|MS_NODEV`, then `syscall.Exec`s the command. Return the executed process exit code. The `!linux` implementation returns `mini-docker run requires Linux or WSL2`.

- [ ] **Step 4: Add privileged Linux integration coverage**

In a `//go:build linux` test, skip unless `os.Geteuid() == 0`. Use a tiny statically linked helper rootfs and assert the child sees PID 1, configured hostname, mounted `/proc`, and no host root files.

```bash
go test ./internal/runtime -v
sudo -E go test ./internal/runtime -run TestRunnerCreatesIsolatedProcess -v
```

Expected: portable tests PASS; integration PASS as root and SKIP otherwise.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime
git commit -m "feat: run images in Linux namespaces"
```

---

### Task 6: Minimal Image Builder

**Files:**
- Create: `internal/builder/copy.go`
- Create: `internal/builder/builder.go`
- Create: `internal/builder/builder_test.go`

**Interfaces:**
- Consumes: parser, store, and `Executor.Execute(ctx, rootfs, command) error`
- Produces: `New(store *image.Store, executor Executor) *Builder` and `Build(ctx, tag, contextDir) error`

- [ ] **Step 1: Write failing safe-COPY tests**

Test one regular file copied to an absolute image path with executable mode preserved. Reject `../outside`, absolute source, source symlink escaping context, relative destination, directory source, special file, and destination resolving through a rootfs symlink to outside.

- [ ] **Step 2: Verify COPY failure**

Run: `go test ./internal/builder -run TestCopy -v`

Expected: FAIL because the copy helper is missing.

- [ ] **Step 3: Implement safe COPY**

Canonicalize context/rootfs with `EvalSymlinks`, use `filepath.Rel` for containment, remove only the destination's leading slash, and re-check the existing destination parent after resolving symlinks. Copy exactly one regular file.

- [ ] **Step 4: Write failing orchestration tests**

With a temporary base image and fake executor, assert COPY content, `RUN` invocation as `[]string{"/bin/sh", "-c", command}`, stored CMD, missing-local-base guidance, unsupported Dockerfile rejection before mutation, and no publication after executor failure.

- [ ] **Step 5: Implement sequential build**

Parse the complete Dockerfile first, clone the base rootfs to a temporary build directory, apply instructions in order, and publish only after success. Adapt `runtime.RunRootFS` as production executor so build RUN uses the same isolation bootstrap.

Run: `go test ./internal/builder ./internal/dockerfile ./internal/image -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/builder internal/runtime
git commit -m "feat: build images from minimal Dockerfiles"
```

---

### Task 7: Production Wiring

**Files:**
- Modify: `cmd/mini-docker/main.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: store, puller, builder, and runner
- Produces: working public commands and internal child dispatch

- [ ] **Step 1: Add failing error and dispatch tests**

Assert service errors return `1` and print one concise line. Extract a testable process dispatcher and assert `__child` is handled before public parsing, while help lists only the three public commands.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/cli ./cmd/mini-docker -v`

Expected: FAIL because production wiring is incomplete.

- [ ] **Step 3: Wire services**

Use `MINI_DOCKER_HOME` when set; otherwise use `$HOME/.mini-docker/images`. Construct one store, runner, puller, and builder. Call `runtime.HandleInternalChild` before `cli.Run`. Help must show only:

```text
mini-docker pull <image>
mini-docker build -t <image> <context>
mini-docker run [--hostname <name>] <image> [command] [args...]
```

- [ ] **Step 4: Verify tests and Linux build**

```bash
go test ./...
GOOS=linux GOARCH=amd64 go build -o bin/mini-docker ./cmd/mini-docker
```

Expected: tests PASS and binary is created.

- [ ] **Step 5: Commit**

```bash
git add cmd/mini-docker internal/cli
git commit -m "feat: wire pull build and run commands"
```

---

### Task 8: Speech Demo and End-to-End Verification

**Files:**
- Create: `testdata/demo/Dockerfile`
- Create: `testdata/demo/hello.sh`
- Create: `README.md`
- Create: `.gitignore`

**Interfaces:**
- Consumes: compiled CLI
- Produces: repeatable 30-minute demo

- [ ] **Step 1: Add demo files**

```dockerfile
FROM alpine:3.22
COPY hello.sh /hello.sh
RUN chmod +x /hello.sh
CMD ["/hello.sh"]
```

```sh
#!/bin/sh
echo "hello from mini-docker"
echo "hostname: $(hostname)"
echo "pid: $$"
echo "root entries: $(ls / | tr '\n' ' ')"
```

- [ ] **Step 2: Document setup and demo**

README prerequisites are Go 1.24+, Linux/WSL2, root privileges for build RUN and run, and network access for pull. Document:

```bash
go build -o bin/mini-docker ./cmd/mini-docker
sudo -E ./bin/mini-docker pull alpine:3.22
sudo -E ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo -E ./bin/mini-docker run --hostname demo-box hello:latest
```

Explain expected output, store location, namespace/chroot/proc behavior, and all v1 limitations. Ignore `/bin/` and `.mini-docker/`.

- [ ] **Step 3: Run automated verification**

```bash
gofmt -w cmd internal
go vet ./...
go test ./...
GOOS=linux GOARCH=amd64 go build -o bin/mini-docker ./cmd/mini-docker
```

Expected: vet exits 0, tests PASS, build exits 0.

- [ ] **Step 4: Run fresh end-to-end demo on WSL2/Linux**

```bash
export MINI_DOCKER_HOME="$(mktemp -d)"
sudo -E ./bin/mini-docker pull alpine:3.22
sudo -E ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo -E ./bin/mini-docker run --hostname demo-box hello:latest
```

Expected: output contains `hello from mini-docker`, hostname `demo-box`, PID `1`, and image root entries.

- [ ] **Step 5: Commit**

```bash
git add .gitignore README.md testdata/demo
git commit -m "docs: add mini docker speech demo"
```

## Final Verification

- [ ] `go vet ./...` exits `0`.
- [ ] `go test ./...` passes.
- [ ] The privileged runtime integration passes on WSL2/native Linux.
- [ ] A fresh `pull -> build -> run` demo succeeds.
- [ ] `git status --short` contains only intentional changes.