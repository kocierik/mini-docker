# CLI Operation Feedback Design

## Problem

`mini-docker pull` and `mini-docker build` can take several seconds while producing no output. In WSL this makes a healthy command look stalled.

## Behavior

The CLI writes concise lifecycle messages to stdout around long-running operations:

```text
Pulling alpine:3.22...
Pulled alpine:3.22
```

```text
Building hello:latest...
Built hello:latest
```

The start message is written immediately before invoking the service. The completion message is written only when the service succeeds. Existing service errors remain unchanged on stderr and retain exit code 1.

`run` is unchanged because the container owns stdout and stderr; lifecycle text would interfere with command output and scripts.

## Architecture

Feedback belongs in `internal/cli`, where command names, user-provided references, tags, and output streams are already available. Registry, builder, and runtime services remain independent from terminal I/O.

## Testing

CLI tests verify:

- pull prints start and success messages in order
- build prints start and success messages in order
- failed pull/build print the start message but not a false success message
- existing stderr and exit-code behavior remains unchanged

No spinner, byte progress, terminal detection, or new dependency is added.
