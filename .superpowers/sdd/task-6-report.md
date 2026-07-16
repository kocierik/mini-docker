# Task 6 Report: Minimal Image Builder

Date: 2026-07-16

## Status

Implemented the minimal image builder with safe single-file `COPY`, parse-before-mutation `Build` orchestration, sequential `COPY`/`RUN`/`CMD` application, exact missing-base guidance, and success-only image publication.

Implementation commit: `a306977` (`feat: build images from minimal Dockerfiles`)

## Files

- `internal/builder/builder.go`
- `internal/builder/builder_test.go`
- `internal/builder/copy.go`

No runtime package files, parser files, image-store files, plans, or specs were modified for Task 6.

## Implementation

- Added `internal/builder.Executor` with `Execute(ctx, rootfs, command) error` and `Builder` with `New(store *image.Store, executor Executor) *Builder` plus `Build(ctx, tag, contextDir) error`.
- `Build` opens `Dockerfile`, parses the complete file before any rootfs clone or store publication, resolves the local base image, clones the base rootfs into a temporary build root, applies instructions in order, and publishes through the existing image store only after all steps succeed.
- Missing local `FROM` images now return the exact guidance required by the design: `base image is not local; run: mini-docker pull <image>`.
- `RUN` is executed strictly as `[]string{"/bin/sh", "-c", command}` through the executor seam.
- The default production executor path reuses `runtime.RunRootFS`.
- Added a safe `COPY` helper that canonicalizes both the build context and rootfs with `EvalSymlinks`, rejects context escape and rootfs escape, requires an absolute destination path, resolves the nearest existing destination parent to block symlink traversal, and copies exactly one regular file while preserving executable mode bits.
- Added builder tests covering the required `COPY` rejection cases, sequential Dockerfile application, missing-base guidance, parser rejection before mutation, and no publication after executor failure.

## TDD Evidence

### RED 1: Missing COPY Helper

Focused `COPY` tests were written before any builder production code existed.

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 && go test ./internal/builder -run TestCopy -v"
```

Output:

```text
# github.com/ekoci/mini-docker/internal/builder [github.com/ekoci/mini-docker/internal/builder.test]
internal/builder/builder_test.go:20:12: undefined: copyFileIntoRootFS
internal/builder/builder_test.go:88:11: undefined: copyFileIntoRootFS
FAIL    github.com/ekoci/mini-docker/internal/builder [build failed]
FAIL

Command exited with code 1
```

This was the intended RED result: the tests failed because the safe `COPY` helper did not exist yet.

### GREEN 1: Safe COPY

After implementing `copyFileIntoRootFS`, the same focused test slice was rerun.

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 && go test ./internal/builder -run TestCopy -v"
```

Output:

```text
=== RUN   TestCopyRegularFileIntoAbsoluteDestination
--- PASS: TestCopyRegularFileIntoAbsoluteDestination (0.00s)
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/absolute_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/relative_destination
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/directory_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/special_file_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape
--- PASS: TestCopyRejectsUnsafePathsAndFileTypes (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/absolute_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/relative_destination (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/directory_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/special_file_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/builder   0.018s
```

### RED 2: Missing Builder Orchestration

The sequential build tests were then added before `New` and `Build` existed.

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 && go test ./internal/builder -v"
```

Output:

```text
# github.com/ekoci/mini-docker/internal/builder [github.com/ekoci/mini-docker/internal/builder.test]
internal/builder/builder_test.go:48:13: undefined: New
internal/builder/builder_test.go:98:13: undefined: New
internal/builder/builder_test.go:127:13: undefined: New
internal/builder/builder_test.go:157:13: undefined: New
FAIL    github.com/ekoci/mini-docker/internal/builder [build failed]
FAIL

Command exited with code 1
```

This RED result matched the intended slice: the orchestration API had not been implemented yet.

### GREEN 2: Builder Package

After implementing `Builder`, `Build`, and the default runtime-backed executor path, the builder package passed.

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 && go test ./internal/builder -v"
```

Output:

```text
=== RUN   TestBuildAppliesDockerfileSequentially
--- PASS: TestBuildAppliesDockerfileSequentially (0.02s)
=== RUN   TestBuildMissingBaseImageGuidance
--- PASS: TestBuildMissingBaseImageGuidance (0.00s)
=== RUN   TestBuildRejectsUnsupportedDockerfileBeforeMutation
--- PASS: TestBuildRejectsUnsupportedDockerfileBeforeMutation (0.00s)
=== RUN   TestBuildDoesNotPublishImageWhenRunFails
--- PASS: TestBuildDoesNotPublishImageWhenRunFails (0.00s)
=== RUN   TestCopyRegularFileIntoAbsoluteDestination
--- PASS: TestCopyRegularFileIntoAbsoluteDestination (0.00s)
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/absolute_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/relative_destination
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/directory_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/special_file_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape
--- PASS: TestCopyRejectsUnsafePathsAndFileTypes (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/absolute_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/relative_destination (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/directory_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/special_file_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/builder   0.059s
```

## Focused Integration Verification

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 && go test ./internal/builder ./internal/dockerfile ./internal/image -v"
```

Output:

```text
=== RUN   TestBuildAppliesDockerfileSequentially
--- PASS: TestBuildAppliesDockerfileSequentially (0.02s)
=== RUN   TestBuildMissingBaseImageGuidance
--- PASS: TestBuildMissingBaseImageGuidance (0.00s)
=== RUN   TestBuildRejectsUnsupportedDockerfileBeforeMutation
--- PASS: TestBuildRejectsUnsupportedDockerfileBeforeMutation (0.00s)
=== RUN   TestBuildDoesNotPublishImageWhenRunFails
--- PASS: TestBuildDoesNotPublishImageWhenRunFails (0.00s)
=== RUN   TestCopyRegularFileIntoAbsoluteDestination
--- PASS: TestCopyRegularFileIntoAbsoluteDestination (0.00s)
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/absolute_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/relative_destination
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/directory_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/special_file_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape
--- PASS: TestCopyRejectsUnsafePathsAndFileTypes (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/builder   (cached)
=== RUN   TestParseValidDockerfile
--- PASS: TestParseValidDockerfile (0.00s)
=== RUN   TestParseSkipsBlankAndCommentLines
--- PASS: TestParseSkipsBlankAndCommentLines (0.00s)
=== RUN   TestParseRejectsInvalidDockerfiles
--- PASS: TestParseRejectsInvalidDockerfiles (0.00s)
=== RUN   TestParseWrapsScannerFailuresWithLineNumber
--- PASS: TestParseWrapsScannerFailuresWithLineNumber (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/dockerfile        (cached)
=== RUN   TestStorePutAndGet
--- PASS: TestStorePutAndGet (0.01s)
=== RUN   TestStoreGetUnknownImage
--- PASS: TestStoreGetUnknownImage (0.00s)
=== RUN   TestStorePutReplacesTagWithoutReplacingObjectDirectory
--- PASS: TestStorePutReplacesTagWithoutReplacingObjectDirectory (0.01s)
=== RUN   TestStorePutHonorsCanceledContext
--- PASS: TestStorePutHonorsCanceledContext (0.00s)
=== RUN   TestStorePutCancelsDuringTraversal
--- PASS: TestStorePutCancelsDuringTraversal (0.00s)
=== RUN   TestStorePutCancelsAfterTemporaryObjectCreation
--- PASS: TestStorePutCancelsAfterTemporaryObjectCreation (0.00s)
=== RUN   TestStoreHashesImageNames
--- PASS: TestStoreHashesImageNames (0.00s)
=== RUN   TestStorePutPublishesReferenceWithoutTemporaryFiles
--- PASS: TestStorePutPublishesReferenceWithoutTemporaryFiles (0.00s)
=== RUN   TestStorePutReferencePublicationUsesSingleDirectoryRename
--- PASS: TestStorePutReferencePublicationUsesSingleDirectoryRename (0.00s)
=== RUN   TestStorePutReferencePublishFailureCleansTemporaryRefs
--- PASS: TestStorePutReferencePublishFailureCleansTemporaryRefs (0.00s)
=== RUN   TestStorePutRejectsUnsafeSymlinks
--- PASS: TestStorePutRejectsUnsafeSymlinks (0.00s)
=== RUN   TestStorePutRejectsUnsupportedFileTypes
--- PASS: TestStorePutRejectsUnsupportedFileTypes (0.00s)
=== RUN   TestStorePutRejectsSockets
--- PASS: TestStorePutRejectsSockets (0.00s)
=== RUN   TestFileModeFromStatMarksSpecialFilesUnsupported
--- PASS: TestFileModeFromStatMarksSpecialFilesUnsupported (0.00s)
=== RUN   TestStorePutRejectsSourcePathSwapOutsideRoot
--- PASS: TestStorePutRejectsSourcePathSwapOutsideRoot (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/image     (cached)
```

## Final Verification

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 && gofmt -w internal/builder/*.go && go test ./... && go vet ./..."
```

Output:

```text
?       github.com/ekoci/mini-docker/cmd/mini-docker    [no test files]
ok      github.com/ekoci/mini-docker/internal/builder   (cached)
ok      github.com/ekoci/mini-docker/internal/cli       (cached)
ok      github.com/ekoci/mini-docker/internal/dockerfile        (cached)
ok      github.com/ekoci/mini-docker/internal/image     (cached)
ok      github.com/ekoci/mini-docker/internal/registry  (cached)
ok      github.com/ekoci/mini-docker/internal/runtime   (cached)
```

`go vet ./...` completed with exit code `0` and no output.

## Self-Review

- The builder public surface matches the task brief exactly: `New(store *image.Store, executor Executor) *Builder` and `Build(ctx, tag, contextDir) error`.
- Dockerfiles are fully parsed before the base image is fetched, before the base rootfs is cloned, and before any new image is published.
- `COPY` enforces both context containment and rootfs containment, including the required destination-parent symlink check and a strict regular-file-only source rule.
- `RUN` goes through the executor seam only, and the default production path is a thin adapter over `runtime.RunRootFS`.
- Publication still delegates to the existing atomic image store, so executor failures and parser failures leave no published target tag.
- Base-image `CMD` is inherited unless a later `CMD` overrides it; this aligns with the sequential `FROM` then `CMD` model and does not widen the feature set.

## Concerns

- The default production `RUN` path through `runtime.RunRootFS` is covered indirectly by existing runtime tests and directly by builder seam tests, but not by a privileged end-to-end builder execution test in this task because the current environment is not running privileged Linux integration as root.

## Review Follow-up: Preserve Base RootFS Special Bits

Review requirement: preserve sticky/setuid/setgid bits when cloning the `FROM` image rootfs into the temporary build root, and cover the regression with focused tests.

### RED: Focused Clone Regression

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 ; go test ./internal/builder -run TestCloneIntoBuildRootPreservesSpecialModeBits -v"
```

Output:

```text
=== RUN   TestCloneIntoBuildRootPreservesSpecialModeBits
    builder_test.go:45: mode(/tmp/mini-docker-build-4011545908/rootfs/tmp) = 755, want 4000755
--- FAIL: TestCloneIntoBuildRootPreservesSpecialModeBits (0.00s)
FAIL
FAIL    github.com/ekoci/mini-docker/internal/builder   0.018s
FAIL

Command exited with code 1
```

This confirmed the review finding before the fix: the temporary build root clone dropped special permission bits.

### GREEN: Focused Clone Regression

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 ; go test ./internal/builder -run TestCloneIntoBuildRootPreservesSpecialModeBits -v"
```

Output:

```text
=== RUN   TestCloneIntoBuildRootPreservesSpecialModeBits
--- PASS: TestCloneIntoBuildRootPreservesSpecialModeBits (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/builder   0.016s
```

The focused regression now verifies a cloned sticky+setgid directory and a cloned setuid+setgid regular file retain their raw Unix mode bits.

### Required Verification

Command:

```powershell
wsl.exe -d Ubuntu -- bash -lc "export PATH=/home/erik/.local/go/bin:/usr/bin:/bin; cd /mnt/c/Users/ekoci/docker-speach/.worktrees/mini-docker-v1 ; gofmt -w internal/builder/*.go ; go test ./internal/builder ./internal/dockerfile ./internal/image -v ; go test ./... ; go vet ./..."
```

Output:

```text
=== RUN   TestCloneIntoBuildRootPreservesSpecialModeBits
--- PASS: TestCloneIntoBuildRootPreservesSpecialModeBits (0.00s)
=== RUN   TestBuildAppliesDockerfileSequentially
--- PASS: TestBuildAppliesDockerfileSequentially (0.01s)
=== RUN   TestBuildMissingBaseImageGuidance
--- PASS: TestBuildMissingBaseImageGuidance (0.00s)
=== RUN   TestBuildRejectsUnsupportedDockerfileBeforeMutation
--- PASS: TestBuildRejectsUnsupportedDockerfileBeforeMutation (0.00s)
=== RUN   TestBuildDoesNotPublishImageWhenRunFails
--- PASS: TestBuildDoesNotPublishImageWhenRunFails (0.00s)
=== RUN   TestCopyRegularFileIntoAbsoluteDestination
--- PASS: TestCopyRegularFileIntoAbsoluteDestination (0.00s)
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/absolute_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/relative_destination
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/directory_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/special_file_source
=== RUN   TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape
--- PASS: TestCopyRejectsUnsafePathsAndFileTypes (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/parent_traversal_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/absolute_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/symlink_escaping_context (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/relative_destination (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/directory_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/special_file_source (0.00s)
    --- PASS: TestCopyRejectsUnsafePathsAndFileTypes/rootfs_symlink_escape (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/builder   0.038s
=== RUN   TestParseValidDockerfile
--- PASS: TestParseValidDockerfile (0.00s)
=== RUN   TestParseSkipsBlankAndCommentLines
--- PASS: TestParseSkipsBlankAndCommentLines (0.00s)
=== RUN   TestParseRejectsInvalidDockerfiles
=== RUN   TestParseRejectsInvalidDockerfiles/empty_input
=== RUN   TestParseRejectsInvalidDockerfiles/instruction_before_from
=== RUN   TestParseRejectsInvalidDockerfiles/instruction_before_from_after_skipped_lines
=== RUN   TestParseRejectsInvalidDockerfiles/duplicate_from
=== RUN   TestParseRejectsInvalidDockerfiles/shell_form_cmd
=== RUN   TestParseRejectsInvalidDockerfiles/json_run
=== RUN   TestParseRejectsInvalidDockerfiles/multiple_copy_sources
=== RUN   TestParseRejectsInvalidDockerfiles/relative_copy_destination
=== RUN   TestParseRejectsInvalidDockerfiles/env_unsupported
=== RUN   TestParseRejectsInvalidDockerfiles/malformed_json_cmd
=== RUN   TestParseRejectsInvalidDockerfiles/empty_cmd_array
--- PASS: TestParseRejectsInvalidDockerfiles (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/empty_input (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/instruction_before_from (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/instruction_before_from_after_skipped_lines (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/duplicate_from (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/shell_form_cmd (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/json_run (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/multiple_copy_sources (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/relative_copy_destination (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/env_unsupported (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/malformed_json_cmd (0.00s)
    --- PASS: TestParseRejectsInvalidDockerfiles/empty_cmd_array (0.00s)
=== RUN   TestParseWrapsScannerFailuresWithLineNumber
--- PASS: TestParseWrapsScannerFailuresWithLineNumber (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/dockerfile        (cached)
=== RUN   TestStorePutAndGet
--- PASS: TestStorePutAndGet (0.01s)
=== RUN   TestStoreGetUnknownImage
--- PASS: TestStoreGetUnknownImage (0.00s)
=== RUN   TestStorePutReplacesTagWithoutReplacingObjectDirectory
--- PASS: TestStorePutReplacesTagWithoutReplacingObjectDirectory (0.01s)
=== RUN   TestStorePutHonorsCanceledContext
--- PASS: TestStorePutHonorsCanceledContext (0.00s)
=== RUN   TestStorePutCancelsDuringTraversal
--- PASS: TestStorePutCancelsDuringTraversal (0.00s)
=== RUN   TestStorePutCancelsAfterTemporaryObjectCreation
--- PASS: TestStorePutCancelsAfterTemporaryObjectCreation (0.00s)
=== RUN   TestStoreHashesImageNames
--- PASS: TestStoreHashesImageNames (0.00s)
=== RUN   TestStorePutPublishesReferenceWithoutTemporaryFiles
--- PASS: TestStorePutPublishesReferenceWithoutTemporaryFiles (0.00s)
=== RUN   TestStorePutReferencePublicationUsesSingleDirectoryRename
--- PASS: TestStorePutReferencePublicationUsesSingleDirectoryRename (0.00s)
=== RUN   TestStorePutReferencePublishFailureCleansTemporaryRefs
--- PASS: TestStorePutReferencePublishFailureCleansTemporaryRefs (0.00s)
=== RUN   TestStorePutRejectsUnsafeSymlinks
=== RUN   TestStorePutRejectsUnsafeSymlinks/absolute
=== RUN   TestStorePutRejectsUnsafeSymlinks/escaping
--- PASS: TestStorePutRejectsUnsafeSymlinks (0.00s)
    --- PASS: TestStorePutRejectsUnsafeSymlinks/absolute (0.00s)
    --- PASS: TestStorePutRejectsUnsafeSymlinks/escaping (0.00s)
=== RUN   TestStorePutRejectsUnsupportedFileTypes
--- PASS: TestStorePutRejectsUnsupportedFileTypes (0.00s)
=== RUN   TestStorePutRejectsSockets
--- PASS: TestStorePutRejectsSockets (0.00s)
=== RUN   TestFileModeFromStatMarksSpecialFilesUnsupported
--- PASS: TestFileModeFromStatMarksSpecialFilesUnsupported (0.00s)
=== RUN   TestStorePutRejectsSourcePathSwapOutsideRoot
--- PASS: TestStorePutRejectsSourcePathSwapOutsideRoot (0.00s)
PASS
ok      github.com/ekoci/mini-docker/internal/image     (cached)
?       github.com/ekoci/mini-docker/cmd/mini-docker    [no test files]
ok      github.com/ekoci/mini-docker/internal/builder   0.036s
ok      github.com/ekoci/mini-docker/internal/cli       (cached)
ok      github.com/ekoci/mini-docker/internal/dockerfile        (cached)
ok      github.com/ekoci/mini-docker/internal/image     (cached)
ok      github.com/ekoci/mini-docker/internal/registry  (cached)
ok      github.com/ekoci/mini-docker/internal/runtime   (cached)
```

`go vet ./...` completed with exit code `0` and no output.
