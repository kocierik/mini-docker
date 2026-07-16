# CLI Operation Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Print immediate start and successful completion messages for long-running `pull` and `build` commands.

**Architecture:** Keep terminal feedback in `internal/cli`, which owns command names and output streams. Service packages remain independent from terminal I/O, and `run` output remains unchanged.

**Tech Stack:** Go standard library and the existing Go test suite.

## Global Constraints

- Write lifecycle feedback to stdout.
- Preserve service errors on stderr and exit code 1.
- Never print a completion message after failure.
- Do not change `run` output.
- Add no spinner, byte progress, terminal detection, or dependency.

---

### Task 1: Pull And Build Lifecycle Feedback

**Files:**
- Modify: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `Puller.Pull(context.Context, string) error` and `Builder.Build(context.Context, string, string) error`
- Produces: stdout messages around successful or failed pull/build service calls; no public Go API changes

- [ ] **Step 1: Write failing CLI tests**

Add tests that call `Run` with `strings.Builder` stdout/stderr and assert:

```go
stdout.String() == "Pulling alpine:3.22...\nPulled alpine:3.22\n"
stdout.String() == "Building hello:latest...\nBuilt hello:latest\n"
```

For failing services, assert stdout contains only the corresponding start line, stderr contains the existing error, and the exit code is 1.

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/cli -run 'TestRunReports(Pull|Build)' -count=1
```

Expected: FAIL because pull/build currently write no lifecycle output.

- [ ] **Step 3: Implement minimal CLI feedback**

Capture `stdout` in the closures created by `Prepare`. Immediately before invoking each long-running service, write the start line. Return the service error unchanged. Write the completion line only after a nil service error.

- [ ] **Step 4: Document expected lifecycle output**

Update `README.md` demo commands with the concise pull/build output users should see. Clarify that `pull` does not require root, while build with `RUN` and `run` do.

- [ ] **Step 5: Verify GREEN and full suite**

Run:

```bash
gofmt -w internal/cli/cli.go internal/cli/cli_test.go
go vet ./...
go test ./... -count=1
GOOS=linux GOARCH=amd64 go build -o bin/mini-docker ./cmd/mini-docker
```

Expected: all commands exit 0.

- [ ] **Step 6: Verify the WSL demo**

Run pull without root using a fresh store, then build/run with root:

```bash
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo -E ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo -E ./bin/mini-docker run --hostname demo-box hello:latest
```

Expected: pull/build lifecycle messages appear immediately; run prints the demo payload with hostname `demo-box` and PID 1.
