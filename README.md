# Mini Docker v1

Mini Docker v1 is a small Go CLI for a live `pull -> build -> run` walkthrough. It pulls a public `linux/amd64` image into a local store, builds a new image from a narrow Dockerfile subset, and runs a command inside real Linux namespaces with `chroot` and a private `/proc` mount.

## Prerequisites

- Go 1.24 or newer
- Linux or WSL2
- Root privileges for `mini-docker build` when the Dockerfile contains `RUN`, and for every `mini-docker run`
- Network access for `mini-docker pull alpine:3.22`

## Build The CLI

```bash
go build -o bin/mini-docker ./cmd/mini-docker
```

Add `bin/` to `.gitignore`; the built binary is a local artifact and not part of source control.

## Demo Fixture

The demo build context lives in `testdata/demo`:

- `Dockerfile` uses exactly `FROM`, `COPY`, `RUN`, and JSON-array `CMD`
- `hello.sh` prints a stable demo payload for the live run

## Exact Demo Commands

Run these commands from the repository root:

```bash
go build -o bin/mini-docker ./cmd/mini-docker
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
```

The long-running operations provide immediate lifecycle feedback:

```text
Pulling alpine:3.22...
Pulled alpine:3.22
Building hello:latest...
Built hello:latest
```

Expected run output includes these lines:

```text
hello from mini-docker
hostname: demo-box
pid: 1
root entries: bin dev etc hello.sh lib proc tmp usr ...
```

`root entries:` will reflect the pulled Alpine root filesystem plus `/hello.sh` and `/proc`, so the exact directory list can vary slightly with the base image contents.

## Fresh End-To-End Demo

For a clean store during a presentation, set `MINI_DOCKER_HOME` to a temporary directory:

```bash
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
```

`pull` does not require root. Dockerfile `RUN` during `build` and every `run` require root. Passing `MINI_DOCKER_HOME` through `sudo env` ensures that all three commands use the same image store without preserving unrelated environment variables.

## Local Image Store

By default, Mini Docker stores images under `$HOME/.mini-docker/images`. If `MINI_DOCKER_HOME` is set, that value is used instead.

The store layout is:

```text
$HOME/.mini-docker/images/
  objects/<object-id>/rootfs
  objects/<object-id>/metadata.json
  refs/<sha256(image-name)>
```

Pull and build publish through a temporary directory and atomic rename so failed operations do not leave a partially published image tag.

## Runtime Behavior

`mini-docker run` and Dockerfile `RUN` both execute through the same Linux runtime path:

- clone into new PID, UTS, and mount namespaces
- optionally set the container hostname
- `chroot` into a temporary writable copy of the selected root filesystem
- mount a private `/proc` inside that isolated rootfs
- `exec` the selected command

This is intentionally small and explicit so the runtime can be explained live in about 30 minutes.

## v1 Limitations

- Only `linux/amd64` images are supported
- Only public image pulls are supported
- No private registry authentication or image push
- Dockerfile support is limited to one leading `FROM`, plus `COPY`, shell-form `RUN`, and JSON-array `CMD`
- Builds are single-stage and have no cache
- `FROM` must reference an image that has already been pulled into the local store; short Docker Hub names like `alpine:3.22` are resolved to the normalized pulled reference
- No network namespace, cgroups, volumes, or overlay filesystem
- No multi-platform support, process management commands, or full Docker CLI compatibility
- Runtime verification requires Linux or WSL2 with sufficient privileges