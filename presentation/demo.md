# Live Demo Runbook

## Before the Talk

1. Open WSL at the repository root and verify network access.
2. Build the binary and warm sudo credentials.
3. Keep `presentation/assets/demo-output.txt` ready in a second terminal tab.
4. Use a terminal font size readable from the back of the room.

```bash
go build -o bin/mini-docker ./cmd/mini-docker
sudo -v
```

## Shared Demo Sequence

```bash
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
```

## Four-Minute Cues

- 0:00-0:20: export the fresh store.
- 0:20-1:10: pull; say “remote OCI content becomes a local rootfs.”
- 1:10-2:20: build; say “derive a filesystem and persist CMD.”
- 2:20-3:05: type run, pause before Enter, and ask what the process should observe.
- 3:05-4:00: connect hostname, PID 1, and root entries to the explained primitives.

## Fallback

After ten seconds without useful progress, display `presentation/assets/demo-output.txt` and explain the same evidence. Do not debug network, sudo, namespaces, or line endings on stage.