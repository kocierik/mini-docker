# Mini Docker v1 Design

## Overview

This project builds a minimal Docker-like pipeline with three operations: pull an image, build a new image, and run a container. It is intended for a 30-minute step-by-step technical presentation to developers. The runtime remains the main teaching focus, while pull and build provide the complete path from a remote base image to an isolated process.

The implementation target is Linux native, with WSL2 acceptable as a local development environment. The implementation language is Go.

## Goals

- Build a minimal runtime that launches a process in isolation.
- Pull a public base image into a local image store.
- Build a new local image from a deliberately small Dockerfile subset.
- Demonstrate the complete `pull -> build -> run` workflow.
- Demonstrate real Linux primitives rather than a simulated container experience.
- Keep the command-line surface small enough for a live technical walkthrough.
- Produce a codebase that can evolve incrementally into a more complete container runtime.

## Non-Goals

- No private registry authentication or image push.
- No build cache, layer cache, or multi-stage builds.
- No complete Dockerfile compatibility.
- No platform support beyond `linux/amd64` in v1.
- No overlay filesystem.
- No volume management.
- No network namespace in v1.
- No cgroups in v1.
- No attempt to provide full Docker CLI compatibility.

## Recommended Approach

Three scope levels were considered:

1. A runtime-only CLI with `run`.
2. A complete but constrained `pull -> build -> run` pipeline.
3. Broad Docker compatibility including full Dockerfile support and production image management.

The approved approach is option 2. It provides a recognizable Docker workflow while keeping each operation narrow. Registry protocol details are delegated to `go-containerregistry`; the educational implementation focuses on the local image store, minimal builder, and Linux runtime primitives.

## Architecture

The project exposes three public commands:

```bash
mini-docker pull alpine:3.22
mini-docker build -t hello:latest .
mini-docker run hello:latest
```

Suggested layout:

```text
cmd/mini-docker
internal/registry
internal/image
internal/dockerfile
internal/builder
internal/runtime
internal/rootfs
internal/ns
```

Responsibilities:

- `cmd/mini-docker`: CLI entry point, argument parsing, validation, and dispatch for `pull`, `build`, and `run`.
- `internal/registry`: public OCI image retrieval through `go-containerregistry`.
- `internal/image`: local image storage, lookup, rootfs snapshots, and command metadata.
- `internal/dockerfile`: parser for the supported `FROM`, `COPY`, `RUN`, and `CMD` instructions.
- `internal/builder`: sequential application of Dockerfile instructions to a temporary rootfs.
- `internal/runtime`: orchestration of the `run` lifecycle.
- `internal/rootfs`: root filesystem validation, `/proc` mount setup, and cleanup.
- `internal/ns`: namespace configuration and isolation-related helpers.

## v1 Scope

The first version includes exactly these features:

- Three public subcommands: `pull`, `build`, and `run`.
- Pull of public `linux/amd64` images from Docker Hub.
- A local image store containing a flattened rootfs and command metadata.
- Dockerfile support limited to `FROM`, `COPY`, `RUN`, and JSON-array `CMD`.
- Single-stage, sequential image builds without caching.
- A base image that must already exist in the local store before `build`.
- A temporary writable rootfs for build and container execution.
- PID namespace isolation.
- UTS namespace isolation.
- Mount namespace isolation.
- Hostname configuration inside the isolated environment.
- `chroot` into the selected root filesystem.
- Mounting `/proc` inside the isolated environment.
- Execution of image `CMD` or a user-selected command override.

The first version explicitly excludes:

- Image push.
- Private registry authentication.
- Additional Dockerfile instructions.
- Build cache and multi-stage builds.
- Multi-platform image selection.
- Network isolation.
- Resource limits.
- Process management commands such as `exec`, `ps`, or `kill`.

## CLI Design

Public syntax:

```bash
mini-docker pull <image-reference>
mini-docker build -t <image-name> <context>
mini-docker run [--hostname <name>] <image-name> [command] [args...]
```

Rules:

- `pull` accepts public Docker Hub references and stores only the `linux/amd64` result.
- `build` requires `-t`, a build context, and a `Dockerfile` in that context.
- `FROM` must reference an image already present in the local store.
- `run` requires a local image name.
- `--hostname` is optional.
- `[command] [args...]` overrides the image `CMD`; otherwise `CMD` is required in image metadata.

Valid examples:

```bash
mini-docker pull alpine:3.22
mini-docker build -t hello:latest .
mini-docker run hello:latest
mini-docker run --hostname demo-box alpine:3.22 /bin/sh
```

## Dockerfile Subset

The v1 parser accepts these forms only:

- `FROM <local-image>` exactly once as the first instruction.
- `COPY <source> <absolute-destination>` with one source inside the build context.
- `RUN <shell-command>` executed as `/bin/sh -c <shell-command>` in the temporary build rootfs.
- `CMD ["executable", "arg", ...]` in JSON-array form.

Any other form or instruction fails before the image is published. `COPY` must reject source path traversal and destination paths that escape the temporary build rootfs.

## Local Image Store

The default store is `$HOME/.mini-docker/images`. Each normalized image name maps to:

- a flattened `rootfs` directory
- a metadata file containing the image name and default command

Pull and build publish through a temporary directory followed by an atomic rename, so a failed operation does not leave a partially usable image. The store deliberately omits layer history, cache metadata, and content-addressed deduplication in v1.

## Execution Flow

### Pull

1. Parse and normalize the public image reference.
2. Request the `linux/amd64` image through `go-containerregistry`.
3. Verify content using registry-provided digests.
4. Apply image layers in order, including OCI whiteout semantics.
5. Store a flattened rootfs plus command metadata in the local image store.

### Build

1. Read the Dockerfile from the supplied build context.
2. Reject unsupported instructions before changing the store.
3. Copy the `FROM` image rootfs into a temporary build directory.
4. Apply each supported instruction in order.
5. Execute `RUN` through the same isolated runtime bootstrap used by containers.
6. Record JSON-array `CMD` as image metadata.
7. Publish the completed rootfs and metadata under the requested tag.

### Run

When the user runs `mini-docker run --hostname demo-box alpine:3.22 /bin/sh`, the runtime performs the following steps:

1. Resolve the image and command from the local store.
2. Create a temporary writable copy of the image rootfs.
3. Create a child process with new PID, UTS, and mount namespaces.
4. Set the hostname inside the isolated UTS namespace.
5. Enter the temporary rootfs using `chroot`.
6. Change working directory to `/`.
7. Mount `/proc` inside the isolated rootfs.
8. Execute the command override or image `CMD`.
9. Propagate the exit code and perform best-effort cleanup.

## Error Handling

The runtime should prefer explicit, short, actionable error messages.

Errors that must be handled clearly:

- invalid or unavailable image reference
- registry download or digest verification failure
- malformed Dockerfile or unsupported instruction
- missing local `FROM` image
- unsafe `COPY` source or destination escaping the allowed roots
- missing local image
- missing both command override and image `CMD`
- insufficient privileges
- `chroot` failure
- `/proc` mount failure
- command not found inside the root filesystem

Representative messages:

```text
image not found in local store: alpine:3.22
unsupported Dockerfile instruction: ENV
base image is not local; run: mini-docker pull alpine:3.22
image has no CMD and no command override was provided
failed to chroot into rootfs: ...
failed to mount /proc: ...
```

Behavioral requirements:

- Return a non-zero exit code on every failure.
- Propagate the child process exit code when execution succeeds.
- Attempt cleanup if bootstrap fails after `/proc` has been mounted.

## Testing Strategy

The first milestone combines lightweight automated checks with manual runtime verification.

Automated checks:

- CLI validation tests.
- Configuration parsing tests.
- Dockerfile parser tests for accepted and rejected instructions.
- Image store round-trip tests.
- Build-context path traversal tests.
- Small unit tests for registry and runtime helper logic that is not kernel-dependent.

Manual verification on Linux or WSL2:

- `hostname` returns the configured container hostname.
- `ps` reflects the isolated PID namespace view.
- The visible filesystem comes from the selected local image.
- A pulled Alpine image can be used as `FROM`.
- A built image runs its recorded `CMD`.

The project should avoid over-investing in heavy end-to-end automation in v1 because namespace-heavy tests add setup complexity that does not materially improve the initial learning objective.

## Demo Flow

Recommended presentation sequence:

1. Introduce containers as images plus isolated processes.
2. Pull Alpine and briefly show the local image store.
3. Build a small image from `FROM`, `COPY`, `RUN`, and `CMD`.
4. Run the new image.
5. Explain the selected namespaces and what each one isolates.
6. Show `chroot` and `/proc` setup in the runtime code.
7. Verify hostname, process view, and filesystem isolation live.

For a 30-minute presentation, pull and build are covered at a high level. Most code-level explanation is reserved for `run`, namespace creation, rootfs setup, and process lifecycle.

## Roadmap

### v2

- Add build and layer caching.
- Add more Dockerfile instructions such as `WORKDIR`, `ENV`, and `ENTRYPOINT`.
- Add support for additional OCI registries and authenticated pulls.

### v3

- Add cgroups for memory and CPU limits.
- Add optional flags such as `--memory` and `--cpu`.
- Improve cleanup and lifecycle robustness.

### v4

- Add network namespace support.
- Add simple isolated connectivity.

## Final Decision

The approved design for implementation is:

- Go implementation.
- Linux-native target, with WSL2 acceptable for development.
- Three public CLI commands: `pull`, `build`, and `run`.
- Public `linux/amd64` image pulls implemented with `go-containerregistry`.
- Minimal single-stage builds supporting `FROM`, `COPY`, `RUN`, and JSON-array `CMD`.
- Local flattened-rootfs image storage without cache or layer history.
- Runtime centered on PID, UTS, and mount namespaces plus `chroot` and `/proc`.
- A 30-minute presentation that treats the runtime as the primary technical subject.

This gives the project a complete, recognizable Docker workflow while keeping the implementation and presentation focused enough to explain the runtime clearly.