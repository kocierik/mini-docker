# Task 7 Report: Production Wiring

Date: 2026-07-16

## Status

Implemented production wiring for the `mini-docker` entrypoint and CLI: the process now resolves the image store root from `MINI_DOCKER_HOME` or `$HOME/.mini-docker/images`, constructs one store plus puller/builder/runner services, dispatches `__child` before public CLI parsing, and exposes help text listing only the three public commands.

## Files

- `cmd/mini-docker/main.go`
- `cmd/mini-docker/main_test.go`
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

No runtime implementation files, store files, puller files, builder files, plans, or specs were modified for Task 7.

## Implementation

- Added a testable `dispatchProcess` seam in `cmd/mini-docker/main.go` so `runtime.HandleInternalChild` can be verified to run before public CLI parsing.
- Added `resolveStoreRoot` in `cmd/mini-docker/main.go` to prefer `MINI_DOCKER_HOME` and otherwise default to `$HOME/.mini-docker/images`.
- Added `newServices` in `cmd/mini-docker/main.go` to construct exactly one shared image store, one runtime runner, one registry puller, and one builder for the CLI.
- Updated `main()` to fail fast on service-construction errors and otherwise delegate through `dispatchProcess`.
- Added explicit public help handling in `internal/cli/cli.go` for `help`, `-h`, and `--help`.
- Updated CLI usage errors with no command, unknown command, or invalid `pull` arity to print only the three documented public command forms.
- Added tests for public help text, usage help text, service-error exit behavior, store-root resolution, and internal-child dispatch ordering.

## TDD Evidence

### RED: Missing Production Wiring Seams and Help Output

Focused tests were added before the production wiring existed.

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; go test ./internal/cli ./cmd/mini-docker -v'
```

Output:

```text
# github.com/ekoci/mini-docker/cmd/mini-docker [github.com/ekoci/mini-docker/cmd/mini-docker.test]
cmd/mini-docker/main_test.go:12:20: undefined: resolveStoreRoot
cmd/mini-docker/main_test.go:25:20: undefined: resolveStoreRoot
cmd/mini-docker/main_test.go:36:14: undefined: dispatchProcess
cmd/mini-docker/main_test.go:66:14: undefined: dispatchProcess
=== RUN   TestRunDispatchesPull
--- PASS: TestRunDispatchesPull (0.00s)
=== RUN   TestRunDispatchesBuild
--- PASS: TestRunDispatchesBuild (0.00s)
=== RUN   TestRunDispatchesRun
--- PASS: TestRunDispatchesRun (0.00s)
=== RUN   TestRunReturnsUsageError
=== RUN   TestRunReturnsUsageError/no_command
=== RUN   TestRunReturnsUsageError/unknown_command
=== RUN   TestRunReturnsUsageError/missing_image
=== RUN   TestRunReturnsUsageError/missing_build_tag
--- PASS: TestRunReturnsUsageError (0.00s)
    --- PASS: TestRunReturnsUsageError/no_command (0.00s)
    --- PASS: TestRunReturnsUsageError/unknown_command (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_image (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_build_tag (0.00s)
=== RUN   TestRunWritesPublicHelp
    cli_test.go:136: Run() exit code = 2, want 0
--- FAIL: TestRunWritesPublicHelp (0.00s)
=== RUN   TestRunWritesUsageHelpForUsageErrors
    cli_test.go:158: stderr = "", want "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
--- FAIL: TestRunWritesUsageHelpForUsageErrors (0.00s)
=== RUN   TestRunReportsServiceError
--- PASS: TestRunReportsServiceError (0.00s)
=== RUN   TestRunReportsServiceErrorsForAllCommands
=== RUN   TestRunReportsServiceErrorsForAllCommands/build
=== RUN   TestRunReportsServiceErrorsForAllCommands/run
--- PASS: TestRunReportsServiceErrorsForAllCommands (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/build (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/run (0.00s)
=== RUN   TestRunPropagatesContainerExitStatus
--- PASS: TestRunPropagatesContainerExitStatus (0.00s)
FAIL
FAIL    github.com/ekoci/mini-docker/internal/cli   0.015s
FAIL    github.com/ekoci/mini-docker/cmd/mini-docker [build failed]
FAIL

Command exited with code 1
```

This was the intended RED result: the tests failed because the main entrypoint wiring seams and CLI help behavior did not exist yet.

### GREEN: Focused CLI and Entrypoint Wiring

After implementing the production wiring seams and help output, the same focused test slice passed.

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; go test ./internal/cli ./cmd/mini-docker -v'
```

Output:

```text
=== RUN   TestRunDispatchesPull
--- PASS: TestRunDispatchesPull (0.00s)
=== RUN   TestRunDispatchesBuild
--- PASS: TestRunDispatchesBuild (0.00s)
=== RUN   TestRunDispatchesRun
--- PASS: TestRunDispatchesRun (0.00s)
=== RUN   TestRunReturnsUsageError
=== RUN   TestRunReturnsUsageError/no_command
=== RUN   TestRunReturnsUsageError/unknown_command
=== RUN   TestRunReturnsUsageError/missing_image
=== RUN   TestRunReturnsUsageError/missing_build_tag
--- PASS: TestRunReturnsUsageError (0.00s)
    --- PASS: TestRunReturnsUsageError/no_command (0.00s)
    --- PASS: TestRunReturnsUsageError/unknown_command (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_image (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_build_tag (0.00s)
=== RUN   TestRunWritesPublicHelp
--- PASS: TestRunWritesPublicHelp (0.00s)
=== RUN   TestRunWritesUsageHelpForUsageErrors
--- PASS: TestRunWritesUsageHelpForUsageErrors (0.00s)
=== RUN   TestRunReportsServiceError
--- PASS: TestRunReportsServiceError (0.00s)
=== RUN   TestRunReportsServiceErrorsForAllCommands
=== RUN   TestRunReportsServiceErrorsForAllCommands/build
=== RUN   TestRunReportsServiceErrorsForAllCommands/run
--- PASS: TestRunReportsServiceErrorsForAllCommands (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/build (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/run (0.00s)
=== RUN   TestRunPropagatesContainerExitStatus
--- PASS: TestRunPropagatesContainerExitStatus (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/cli   0.010s
=== RUN   TestResolveStoreRootUsesMiniDockerHome
--- PASS: TestResolveStoreRootUsesMiniDockerHome (0.00s)
=== RUN   TestResolveStoreRootDefaultsUnderHome
--- PASS: TestResolveStoreRootDefaultsUnderHome (0.00s)
=== RUN   TestDispatchProcessHandlesInternalChildBeforeCLI
--- PASS: TestDispatchProcessHandlesInternalChildBeforeCLI (0.00s)
=== RUN   TestDispatchProcessForwardsPublicArgsToCLI
--- PASS: TestDispatchProcessForwardsPublicArgsToCLI (0.00s)
PASS
ok      github.com/ekoci/mini-docker/cmd/mini-docker        0.011s
```

## Full Verification

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; go test ./...; go vet ./...; GOOS=linux GOARCH=amd64 go build -o bin/mini-docker ./cmd/mini-docker'
```

Output:

```text
ok      github.com/ekoci/mini-docker/cmd/mini-docker        0.011s
ok      github.com/ekoci/mini-docker/internal/builder       (cached)
ok      github.com/ekoci/mini-docker/internal/cli   0.012s
ok      github.com/ekoci/mini-docker/internal/dockerfile    (cached)
ok      github.com/ekoci/mini-docker/internal/image (cached)
ok      github.com/ekoci/mini-docker/internal/registry      (cached)
ok      github.com/ekoci/mini-docker/internal/runtime       (cached)
```

`go vet ./...` completed with no output and the Linux build succeeded, producing `bin/mini-docker` during verification.

## Self-Review

- Checked the entrypoint contract against the brief: `runtime.HandleInternalChild` is still the first behavioral decision, ahead of public CLI parsing.
- Checked the store-root requirement against the brief: `MINI_DOCKER_HOME` wins when set, otherwise the default path is derived from `HOME` with no extra fallback behavior added.
- Checked help scope against the plan: the help text lists only the three public command forms and does not expose `__child`.
- Kept the change narrow: no demo files, no extra commands, and no behavior changes in store, puller, builder, or runtime beyond the required production wiring.

## Review Fixes

Addressed the two Task 7 review findings with a narrow follow-up:

1. Moved the internal-child dispatch ahead of all service construction and store-root resolution by adding `runProcess`, which checks `runtime.HandleInternalChild` before calling `newServices`.
2. Suppressed raw Go flag parser output for invalid `build` and `run` usage so the CLI now prints only the documented public help text on usage errors.

## Review Fix TDD Evidence

### RED: Review Regressions Reproduced

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; mkdir -p /tmp/mini-docker-gopath /tmp/mini-docker-gomodcache /tmp/mini-docker-gocache; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache go test ./internal/cli ./cmd/mini-docker -v'
```

Output:

```text
# github.com/ekoci/mini-docker/cmd/mini-docker [github.com/ekoci/mini-docker/cmd/mini-docker.test]
cmd/mini-docker/main_test.go:99:14: undefined: runProcess
cmd/mini-docker/main_test.go:135:14: undefined: runProcess
=== RUN   TestRunDispatchesPull
--- PASS: TestRunDispatchesPull (0.00s)
=== RUN   TestRunDispatchesBuild
--- PASS: TestRunDispatchesBuild (0.00s)
=== RUN   TestRunDispatchesRun
--- PASS: TestRunDispatchesRun (0.00s)
=== RUN   TestRunReturnsUsageError
=== RUN   TestRunReturnsUsageError/no_command
=== RUN   TestRunReturnsUsageError/unknown_command
=== RUN   TestRunReturnsUsageError/missing_image
=== RUN   TestRunReturnsUsageError/missing_build_tag
--- PASS: TestRunReturnsUsageError (0.00s)
    --- PASS: TestRunReturnsUsageError/no_command (0.00s)
    --- PASS: TestRunReturnsUsageError/unknown_command (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_image (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_build_tag (0.00s)
=== RUN   TestRunWritesPublicHelp
--- PASS: TestRunWritesPublicHelp (0.00s)
=== RUN   TestRunWritesUsageHelpForUsageErrors
--- PASS: TestRunWritesUsageHelpForUsageErrors (0.00s)
=== RUN   TestRunBuildUsageErrorsWriteOnlyPublicHelp
    cli_test.go:176: stderr = "flag provided but not defined: -bad-flag\nUsage of build:\n  -t string\n    \timage tag\n", want "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
--- FAIL: TestRunBuildUsageErrorsWriteOnlyPublicHelp (0.00s)
=== RUN   TestRunContainerUsageErrorsWriteOnlyPublicHelp
    cli_test.go:194: stderr = "flag provided but not defined: -bad-flag\nUsage of run:\n  -hostname string\n    \tcontainer hostname\n", want "mini-docker pull <image>\nmini-docker build -t <image> <context>\nmini-docker run [--hostname <name>] <image> [command] [args...]\n"
--- FAIL: TestRunContainerUsageErrorsWriteOnlyPublicHelp (0.00s)
=== RUN   TestRunReportsServiceError
--- PASS: TestRunReportsServiceError (0.00s)
=== RUN   TestRunReportsServiceErrorsForAllCommands
=== RUN   TestRunReportsServiceErrorsForAllCommands/build
=== RUN   TestRunReportsServiceErrorsForAllCommands/run
--- PASS: TestRunReportsServiceErrorsForAllCommands (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/build (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/run (0.00s)
=== RUN   TestRunPropagatesContainerExitStatus
--- PASS: TestRunPropagatesContainerExitStatus (0.00s)
FAIL
FAIL    github.com/ekoci/mini-docker/internal/cli   0.012s
FAIL    github.com/ekoci/mini-docker/cmd/mini-docker [build failed]
FAIL
```

This was the intended RED result: the new regression tests failed because the entrypoint still built services before the `__child` fast path and the CLI still leaked raw flag parser output for invalid `build` and `run` usage.

### GREEN: Focused Review Fix Tests

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; mkdir -p /tmp/mini-docker-gopath /tmp/mini-docker-gomodcache /tmp/mini-docker-gocache; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache gofmt -w cmd/mini-docker/main.go cmd/mini-docker/main_test.go internal/cli/cli.go internal/cli/cli_test.go; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache go test ./internal/cli ./cmd/mini-docker -v'
```

Output:

```text
=== RUN   TestRunDispatchesPull
--- PASS: TestRunDispatchesPull (0.00s)
=== RUN   TestRunDispatchesBuild
--- PASS: TestRunDispatchesBuild (0.00s)
=== RUN   TestRunDispatchesRun
--- PASS: TestRunDispatchesRun (0.00s)
=== RUN   TestRunReturnsUsageError
=== RUN   TestRunReturnsUsageError/no_command
=== RUN   TestRunReturnsUsageError/unknown_command
=== RUN   TestRunReturnsUsageError/missing_image
=== RUN   TestRunReturnsUsageError/missing_build_tag
--- PASS: TestRunReturnsUsageError (0.00s)
    --- PASS: TestRunReturnsUsageError/no_command (0.00s)
    --- PASS: TestRunReturnsUsageError/unknown_command (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_image (0.00s)
    --- PASS: TestRunReturnsUsageError/missing_build_tag (0.00s)
=== RUN   TestRunWritesPublicHelp
--- PASS: TestRunWritesPublicHelp (0.00s)
=== RUN   TestRunWritesUsageHelpForUsageErrors
--- PASS: TestRunWritesUsageHelpForUsageErrors (0.00s)
=== RUN   TestRunBuildUsageErrorsWriteOnlyPublicHelp
--- PASS: TestRunBuildUsageErrorsWriteOnlyPublicHelp (0.00s)
=== RUN   TestRunContainerUsageErrorsWriteOnlyPublicHelp
--- PASS: TestRunContainerUsageErrorsWriteOnlyPublicHelp (0.00s)
=== RUN   TestRunReportsServiceError
--- PASS: TestRunReportsServiceError (0.00s)
=== RUN   TestRunReportsServiceErrorsForAllCommands
=== RUN   TestRunReportsServiceErrorsForAllCommands/build
=== RUN   TestRunReportsServiceErrorsForAllCommands/run
--- PASS: TestRunReportsServiceErrorsForAllCommands (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/build (0.00s)
    --- PASS: TestRunReportsServiceErrorsForAllCommands/run (0.00s)
=== RUN   TestRunPropagatesContainerExitStatus
--- PASS: TestRunPropagatesContainerExitStatus (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/cli   0.010s
=== RUN   TestResolveStoreRootUsesMiniDockerHome
--- PASS: TestResolveStoreRootUsesMiniDockerHome (0.00s)
=== RUN   TestResolveStoreRootDefaultsUnderHome
--- PASS: TestResolveStoreRootDefaultsUnderHome (0.00s)
=== RUN   TestDispatchProcessHandlesInternalChildBeforeCLI
--- PASS: TestDispatchProcessHandlesInternalChildBeforeCLI (0.00s)
=== RUN   TestDispatchProcessForwardsPublicArgsToCLI
--- PASS: TestDispatchProcessForwardsPublicArgsToCLI (0.00s)
=== RUN   TestRunProcessHandlesInternalChildBeforeBuildingServices
--- PASS: TestRunProcessHandlesInternalChildBeforeBuildingServices (0.00s)
=== RUN   TestRunProcessReportsServiceFactoryError
--- PASS: TestRunProcessReportsServiceFactoryError (0.00s)
PASS
ok      github.com/ekoci/mini-docker/cmd/mini-docker        0.045s
```

### Full Verification After Review Fixes

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; mkdir -p /tmp/mini-docker-gopath /tmp/mini-docker-gomodcache /tmp/mini-docker-gocache; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache go test ./...; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache go vet ./...; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache GOOS=linux GOARCH=amd64 go build -o bin/mini-docker ./cmd/mini-docker'
```

Output:

```text
ok      github.com/ekoci/mini-docker/cmd/mini-docker        0.014s
ok      github.com/ekoci/mini-docker/internal/builder       0.036s
ok      github.com/ekoci/mini-docker/internal/cli   0.014s
ok      github.com/ekoci/mini-docker/internal/dockerfile    0.013s
ok      github.com/ekoci/mini-docker/internal/image 0.047s
ok      github.com/ekoci/mini-docker/internal/registry      0.018s
ok      github.com/ekoci/mini-docker/internal/runtime       0.010s
```

`go vet ./...` completed with no output and the Linux build succeeded, producing `bin/mini-docker`.

### Internal Child Smoke Check

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc 'export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; mkdir -p /tmp/mini-docker-gopath /tmp/mini-docker-gomodcache /tmp/mini-docker-gocache; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1; env -u HOME -u MINI_DOCKER_HOME GOPATH=/tmp/mini-docker-gopath GOMODCACHE=/tmp/mini-docker-gomodcache GOCACHE=/tmp/mini-docker-gocache go run ./cmd/mini-docker __child'
```

Output:

```text
MINI_DOCKER_CHILD_CONFIG is not set
exit status 1
```

This confirms the `__child` path now reaches the internal-child handler without failing on `HOME` or `MINI_DOCKER_HOME` resolution first.