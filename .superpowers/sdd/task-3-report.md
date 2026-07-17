# Task 3 Report: Validate the Complete Script

## Status

Complete. The final script is committed as `e70dd861b5a1f73bfe69d59fe58152a44ed7df44` (`docs: finalize mini-docker talk script`). All required post-commit validations pass.

## Files Changed

- `presentation/speech.md`
  - Reworded the opening so that we start a process which observes a different environment; the process is no longer described as starting itself.
  - Narrowed the slide 8 demo handoff from proving every claim to demonstrating observable results.
  - Preserved the already nonrepetitive slide 4 handoff.
  - Normalized the working copy to LF so the brief's exact end-anchored PowerShell heading regex succeeds.
- `.superpowers/sdd/task-3-report.md`
  - Added this ignored task report after the final commit.

No deck, source code, demo command, or other presentation asset was modified. The design specification and implementation plan were already committed and unchanged, so they were not included in the final commit command.

## Command Log

All commands ran from `C:\Users\ekoci\docker-speach\.worktrees\reveal-presentation` through Windows PowerShell unless noted.

### Initial State and Invalid Parallel Attempt

```powershell
git status --short
```

Result: no output; the worktree was initially clean.

The first attempt launched the following three brief checks in parallel in the shared persistent terminal:

```powershell
$speech = Get-Content presentation/speech.md -Raw
$headings = [regex]::Matches($speech, '(?m)^## Slide (\d+) - .+ \((\d+) minutes?\)$')
$numbers = $headings | ForEach-Object { [int]$_.Groups[1].Value }
$minutes = ($headings | ForEach-Object { [int]$_.Groups[2].Value } | Measure-Object -Sum).Sum
if (($numbers -join ',') -ne '1,2,3,4,5,6,7,8,9,10') { throw "Unexpected slide order: $($numbers -join ',')" }
if ($minutes -ne 30) { throw "Unexpected duration: $minutes minutes" }
"Slides: $($headings.Count); duration: $minutes minutes"
```

```powershell
$speech = Get-Content presentation/speech.md -Raw
$required = '[CLICK]', '[NEXT SLIDE]', '[PAUSE]', '[POINT TO ', '[SWITCH TO TERMINAL]', '[RETURN TO DECK]', "That's the core of a container. Questions?"
foreach ($item in $required) {
    if (-not $speech.Contains($item)) { throw "Missing required text: $item" }
}
'Required cues and closing are present'
```

```powershell
$speech = Get-Content presentation/speech.md -Raw
$commands = @(
    'export MINI_DOCKER_HOME="$(mktemp -d)"',
    './bin/mini-docker pull alpine:3.22',
    'sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo',
    'sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest'
)
foreach ($command in $commands) {
    if (-not $speech.Contains($command)) { throw "Missing demo command: $command" }
}
'Canonical demo sequence is present'
```

Result: invalid due to command interleaving in the shared persistent terminal. The structural output included `Unexpected slide order:` and `Slides: 0; duration: 0 minutes`; no result from this batch was accepted as validation evidence.

A sequential multiline rerun of the structural command executed only its first line in the persistent terminal and produced no validation output. It was also discarded.

### Line-Ending Diagnosis

The structural command was then issued atomically as one PowerShell statement. It reproducibly failed with `Unexpected slide order:` because the heading regex found zero complete matches.

```powershell
git ls-files --eol presentation/speech.md
```

Output:

```text
i/lf    w/crlf  attr/                   presentation/speech.md
```

```powershell
$speech = Get-Content presentation/speech.md -Raw; "Heading lines: $(([regex]::Matches($speech, '(?m)^## Slide ')).Count)"; "CRLF sequences: $(([regex]::Matches($speech, "`r`n")).Count)"; "LF-only sequences: $(([regex]::Matches($speech, '(?<!\r)\n')).Count)"
```

Output:

```text
Heading lines: 10
CRLF sequences: 319
LF-only sequences: 0
```

This confirmed that the brief's `$` anchor could not match before the carriage return in CRLF headings.

After the two wording edits, the structural command was run immediately and failed for the same confirmed CRLF reason. The file was then normalized with:

```powershell
$path = (Resolve-Path 'presentation/speech.md'); $text = [System.IO.File]::ReadAllText($path); $text = $text.Replace("`r`n", "`n"); [System.IO.File]::WriteAllText($path, $text, [System.Text.UTF8Encoding]::new($false))
```

The exact structural validation then passed:

```text
Slides: 10; duration: 30 minutes
```

### Pre-Commit Validation

The exact required-cue and closing command produced:

```text
Required cues and closing are present
```

The exact canonical-command command produced:

```text
Canonical demo sequence is present
```

The exact word-count command produced:

```text
Estimated spoken words: 3354
```

```powershell
npm run slides:test
```

Result: all 9 Playwright tests passed in 13.2 seconds.

```powershell
git diff --check
```

Result: exit code 0 with no whitespace error. Before staging, Git warned that LF would be replaced by CRLF the next time it touched `presentation/speech.md`.

```powershell
git status --short
```

Output:

```text
 M presentation/speech.md
```

```powershell
git diff -- presentation/speech.md
```

Result: exactly two wording replacements, one on slide 1 and one on slide 8; no other content change.

### Commit

```powershell
git add presentation/speech.md; git commit -m "docs: finalize mini-docker talk script"
```

Output:

```text
[feature/reveal-presentation e70dd86] docs: finalize mini-docker talk script
 1 file changed, 2 insertions(+), 2 deletions(-)
```

The unchanged planning files were intentionally omitted as required by the task instructions.

### Fresh Post-Commit Validation

The brief's exact structural, cue, canonical-command, and word-count checks were rerun after commit. Outputs:

```text
Slides: 10; duration: 30 minutes
Required cues and closing are present
Canonical demo sequence is present
Estimated spoken words: 3354
```

```powershell
npm run slides:test
```

Result: all 9 Playwright tests passed in 17.0 seconds.

```powershell
git diff --check
```

Result: no output, exit code 0.

```powershell
git status --short
```

Result: no output; tracked worktree state clean.

```powershell
git show -s --format='%H%n%s' HEAD
```

Output:

```text
e70dd861b5a1f73bfe69d59fe58152a44ed7df44
docs: finalize mini-docker talk script
```

## Self-Review

- Natural spoken English: the opening now assigns agency correctly to the presenter/runtime, and the replacement sentence preserves the existing cadence. The slide transitions read naturally; slide 3 hands into Build, slide 4 hands into the runtime contract, and there is no repetitive slide 4 handoff.
- Technical guardrails: PID 1 is consistently qualified as PID 1 inside the new PID namespace; the script says the host kernel is shared and no guest kernel starts; namespace creation is distinguished from `chroot`; `chroot` is explicitly not presented as a complete security boundary; the proc directory listing is explicitly not claimed as proof that procfs is mounted; and production omissions are stated.
- Demo claims: the slide 8 handoff now promises only observable results. Slide 9 separately says source inspection establishes the procfs mount, while output demonstrates hostname, PID, command, and rootfs observations.
- Timing: all ten declared slide allocations sum to exactly 30 minutes. The 3,354-word estimate is within the specified rehearsal range and leaves the planned live-demo and pause time intact.
- Closing: the script ends exactly with `That's the core of a container. Questions?`.

## Concerns

- The repository's Windows Git configuration checks this file out as CRLF (`i/lf w/crlf`). A future Git operation that rewrites the working file may restore CRLF and make the brief's exact heading regex fail again even though the committed blob remains LF. No `.gitattributes` change was made because this task permits only necessary speech adjustments and forbids unrelated presentation changes.
- No full timed spoken rehearsal was performed; timing evidence is the declared 30-minute allocation and the required 3,354-word estimate.

## Final Review Fix

### Changes

- Tightened slide 8 and slide 9 so terminal output confirms expected hostname, PID, and image-derived root observations while the preceding source inspection supplies causal mechanism evidence.
- Added `[RUN SETUP COMMAND]`, `[RUN PULL]`, `[RUN BUILD]`, and `[RUN - PAUSE BEFORE ENTER]` to both the cue legend and demo choreography without changing the canonical command strings.
- Moved the existing ten-second fallback instruction to immediately after Pull, where it explicitly applies to Pull and each remaining live command and preserves the no-live-debug rule.

### Commit

```text
19d9658299548ee433647a5727c4889ed6f40cbd
docs: tighten mini-docker demo script
```

### Fresh Post-Commit Validation

The combined PowerShell content check validated slide headings and timing, canonical commands, the exact closing, execution cue coverage, and the spoken-word estimate.

```text
Slides: 10; duration: 30 minutes
Canonical demo sequence is present
Exact closing is present
[RUN SETUP COMMAND] count=2
[RUN PULL] count=2
[RUN BUILD] count=2
[RUN - PAUSE BEFORE ENTER] count=2
Estimated spoken words: 3441
```

```powershell
git diff --check
git diff --check HEAD^ HEAD
git status --short
git show -s --format='%H%n%s' HEAD
```

Output: both diff checks and `git status --short` produced no output. The commit query produced:

```text
19d9658299548ee433647a5727c4889ed6f40cbd
docs: tighten mini-docker demo script
```

```powershell
npm run slides:test
```

Output:

```text
Running 9 tests using 1 worker
9 passed (9.4s)
```

### Concerns

- No full timed spoken rehearsal was performed; timing evidence remains the ten headings totaling 30 minutes and the 3,441-word estimate.
- The existing Windows Git LF-to-CRLF warning remains; no line-ending policy file was changed because it was outside the permitted scope.

## Final Review Evidentiary Wording Fix

### Changes

- Replaced the slide 1 claim that observations are proof with wording that presents them as results requiring explanation.
- Replaced the slide 8 claim that output is proof with wording that presents output as observations to check.
- Preserved the source-backed causal qualifications on slides 7 and 8 unchanged.

### Pre-Commit Validation

```powershell
Set-Location 'C:\Users\ekoci\docker-speach\.worktrees\reveal-presentation'; $speech = Get-Content 'presentation/speech.md' -Raw; $rejected = @('Those observations are the proof','the output is the proof'); $foundRejected = @($rejected | Where-Object { $speech.Contains($_) }); if ($foundRejected.Count -ne 0) { throw "Rejected phrases remain: $($foundRejected -join '; ')" }; 'Rejected phrases absent: 2/2'; $mechanismClaims = Select-String -Path 'presentation/speech.md' -Pattern '\b(proof|prove|proves|proved|proven)\b' -CaseSensitive:$false; 'Remaining proof/prove references:'; $mechanismClaims | ForEach-Object { "  line $($_.LineNumber): $($_.Line.Trim())" }; $headings = [regex]::Matches($speech, '(?m)^## Slide (\d+) - .+ \((\d+) minutes?\)\r?$'); $numbers = ($headings | ForEach-Object { $_.Groups[1].Value }) -join ','; $minutes = ($headings | ForEach-Object { [int]$_.Groups[2].Value } | Measure-Object -Sum).Sum; if ($headings.Count -ne 10 -or $numbers -ne '1,2,3,4,5,6,7,8,9,10' -or $minutes -ne 30) { throw "Structural check failed: slides=$($headings.Count), order=$numbers, minutes=$minutes" }; "Structure: $($headings.Count) slides; order $numbers; $minutes minutes"; $commands = @('export MINI_DOCKER_HOME="$(mktemp -d)"','./bin/mini-docker pull alpine:3.22','sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo','sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest'); $missingCommands = @($commands | Where-Object { -not $speech.Contains($_) }); if ($missingCommands.Count -ne 0) { throw "Missing canonical commands: $($missingCommands -join '; ')" }; $closing = "That's the core of a container. Questions?"; if (-not $speech.TrimEnd().EndsWith($closing)) { throw 'Exact closing check failed' }; 'Canonical commands: 4/4; exact closing: present'; $spoken = $speech -replace '(?s)```.*?```', '' -replace '(?m)^#.*$', '' -replace '\[[A-Z][^\]]*\]', '' -replace '[`*_>#-]', ' '; $words = ([regex]::Matches($spoken, "\b[\p{L}\p{N}][\p{L}\p{N}'/-]*\b")).Count; if ($words -lt 3000 -or $words -gt 3900) { throw "Spoken word count out of range: $words" }; "Spoken words: $words (required 3000-3900)"; git diff --check; if ($LASTEXITCODE -ne 0) { throw 'git diff --check failed' }; 'git diff --check: passed'; npm run slides:test; if ($LASTEXITCODE -ne 0) { throw 'npm run slides:test failed' }
```

Result:

```text
Rejected phrases absent: 2/2
Remaining proof/prove references:
  line 248: The `proc` entry marks the path prepared for `/proc`. The listing alone cannot prove that a filesystem is mounted there; the source we just inspected establishes that it is a new proc filesystem mounted after namespace creation. Programs reading it therefore see the PID namespace's process view rather than a proc tree inherited from the host.
  line 309: There are four useful checks here. `hello from mini-docker` confirms that the recorded command ran as the payload. `hostname: demo-box` and `pid: 1` confirm the expected hostname and process-ID observations. The root entries show the expected image-derived root view because they include our script and the image directories. They also show the `/proc` mount point, but neither the listing nor the other output independently proves how those views were produced. The runtime source we inspected supplies that causal evidence: namespace creation, child setup, `chroot`, the procfs mount, and `exec`. The exact directory list can vary slightly, but the expected observations and the inspected mechanisms remain connected.
Structure: 10 slides; order 1,2,3,4,5,6,7,8,9,10; 30 minutes
Canonical commands: 4/4; exact closing: present
Spoken words: 3448 (required 3000-3900)
warning: in the working copy of 'presentation/speech.md', LF will be replaced by CRLF the next time Git touches it
git diff --check: passed

> slides:test
> playwright test -c presentation/playwright.config.js

Running 9 tests using 1 worker
9 passed (9.3s)
```

### Concerns

- No full timed spoken rehearsal was performed; timing evidence remains the ten headings totaling 30 minutes and the 3,448-word estimate.
- The existing Windows Git LF-to-CRLF warning remains; no line-ending policy file was changed because it is outside this fix's scope.