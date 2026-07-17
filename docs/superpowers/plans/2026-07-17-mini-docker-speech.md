# Mini-Docker Speech Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a complete, natural English speaker script for the approved 30-minute mini-Docker presentation.

**Architecture:** Keep one canonical script in `presentation/speech.md`, organized by the deck's ten slides. Draft the lifecycle and runtime halves separately so each can be reviewed for narrative and technical fidelity, then run structural and content checks across the complete artifact.

**Tech Stack:** Markdown, Reveal.js deck content, Go source excerpts, PowerShell validation commands.

## Global Constraints

- Audience: programmers familiar with Docker as users, but not necessarily with Linux container internals.
- Duration: 30 minutes, followed by questions.
- Language: clear international English.
- Tone: conversational and technical, with short sentences, rhetorical questions, and no forced jokes.
- Form: complete slide-by-slide spoken text with unobtrusive stage cues.
- Target length: approximately 3,500 words, allowing for stage actions and terminal output.
- Preserve the approved ten-slide order and timing split.
- Do not modify the deck, demo commands, or implementation while writing the script.
- End with: "That's the core of a container. Questions?"

---

### Task 1: Opening, Pull, and Build

**Files:**
- Create: `presentation/speech.md`
- Reference: `presentation/index.html`
- Reference: `presentation/notes.md`

**Interfaces:**
- Consumes: approved slide order, fragment order, code excerpts, and timing from the deck and notes.
- Produces: the canonical script header and complete sections for slides 1-4.

- [ ] **Step 1: Create the script structure**

Create `presentation/speech.md` with a title, a short cue legend, and headings in this exact form:

```markdown
# Build Your Own Docker - Speaker Script

## Cue Legend

- `[CLICK]`: advance the current slide fragment.
- `[NEXT SLIDE]`: move to the next slide.
- `[PAUSE]`: leave a short beat for the audience.
- `[POINT TO ...]`: direct attention to a visible element.
- `[SWITCH TO TERMINAL]`: leave the deck for the live demo.
- `[RETURN TO DECK]`: return to the presentation.

## Slide 1 - How does Docker work? (2 minutes)
## Slide 2 - One lifecycle, three jobs (2 minutes)
## Slide 3 - Pull: from OCI layers to a local image (3 minutes)
## Slide 4 - Build: turn instructions into an image (3 minutes)
```

- [ ] **Step 2: Write slides 1-2**

Write complete spoken prose that:

- opens with the exact question "How does Docker work?";
- distinguishes understanding a container from listing Docker features;
- narrows the talk to one process and one `pull -> build -> run` path;
- reveals and explains each lifecycle stage in deck order;
- makes `run` the unresolved question that leads into the rest of the talk;
- includes every required fragment cue and a spoken transition between slides.

- [ ] **Step 3: Write slide 3**

Explain, in deck order:

- why an image is not a virtual machine disk;
- OCI layers as ordered filesystem changes;
- extraction into a temporary root filesystem;
- command metadata as part of image behavior;
- atomic publication through `store.Put` only after extraction succeeds;
- the exact code excerpt shown by the slide, without inventing APIs or skipping error handling.

End by asking how a local image is changed by a Dockerfile.

- [ ] **Step 4: Write slide 4**

Explain the demo Dockerfile and the guarded `COPY`, `RUN`, and `CMD` branches in the builder switch. Emphasize that `RUN` executes against the build rootfs while `CMD` records future runtime behavior. End by moving from image construction to the runtime contract.

- [ ] **Step 5: Review the first four slides**

Read the sections aloud. Confirm that the prose takes about 10 minutes, all visible fragments have cues, all code claims match `presentation/index.html`, and no production Docker subsystem is introduced prematurely.

- [ ] **Step 6: Commit the first half**

```powershell
git add presentation/speech.md
git commit -m "docs: draft mini-docker speech opening"
```

Expected: one commit containing the script through slide 4.

---

### Task 2: Runtime, Demo, and Closing

**Files:**
- Modify: `presentation/speech.md`
- Reference: `presentation/index.html`
- Reference: `presentation/notes.md`
- Reference: `presentation/demo.md`
- Reference: `presentation/assets/demo-output.txt`
- Reference: `internal/runtime/runner.go`
- Reference: `internal/runtime/runner_linux.go`

**Interfaces:**
- Consumes: the canonical script and transition produced by Task 1.
- Produces: complete sections for slides 5-10 and a complete end-to-end talk.

- [ ] **Step 1: Add the remaining slide headings**

Append headings in this exact form:

```markdown
## Slide 5 - What must run change? (3 minutes)
## Slide 6 - Create the boundary before entering it (4 minutes)
## Slide 7 - Assemble the child's world (4 minutes)
## Slide 8 - Exec makes the payload PID 1 (3 minutes)
## Slide 9 - Live demo: pull, build, run (4 minutes)
## Slide 10 - The minimal answer (2 minutes)
```

- [ ] **Step 2: Write slide 5**

Turn hostname, PID, rootfs, and `/proc` into four observable runtime promises. Explain that these are different views for one ordinary Linux process, not evidence of a guest kernel. Use the four promises to define what the following code must accomplish.

- [ ] **Step 3: Write slide 6**

Explain why the parent re-executes `/proc/self/exe`, how the internal child mode carries configuration, and why `CLONE_NEWPID`, `CLONE_NEWUTS`, and `CLONE_NEWNS` are selected when the child is created. State precisely that PID 1 is inside the new PID namespace. Follow the deck's fragment order and source excerpt.

- [ ] **Step 4: Write slide 7**

Explain the child setup in exact execution order:

1. make mount propagation private;
2. set the hostname in the UTS namespace;
3. call `chroot` and change to `/`;
4. create and mount `/proc` from the new namespace view;
5. call `syscall.Exec`.

State that `chroot` changes pathname resolution but is not a complete security boundary. Keep every claim aligned with `internal/runtime/runner_linux.go`.

- [ ] **Step 5: Write slide 8**

Explain process replacement rather than process creation: after `exec`, the bootstrap code is gone and the payload inherits its namespace membership and becomes PID 1 inside the namespace. Connect each result back to the four promises from slide 5.

- [ ] **Step 6: Write slide 9**

Use the exact commands from `presentation/demo.md`. Include cues to switch to the terminal, wait without filling every pause, identify these four proofs in output, and return to the deck:

```text
hello from mini-docker
hostname: demo-box
pid: 1
root entries: bin dev etc hello.sh lib proc tmp usr ...
```

Add a clearly marked fallback cue: if the live command fails or runs long, show `presentation/assets/demo-output.txt` and continue with the same explanation instead of debugging on stage.

- [ ] **Step 7: Write slide 10**

Answer the opening question directly: the demonstrated container is a normal Linux process launched with namespace isolation, a prepared root filesystem, a mounted `/proc`, and an `exec`-replaced payload. Name the omitted production capabilities shown by the deck, repeat that the project is educational rather than production-ready, and end exactly with:

```text
That's the core of a container. Questions?
```

- [ ] **Step 8: Review runtime and demo fidelity**

Compare slides 5-10 against both runtime source files, the deck, and the demo runbook. Confirm the runtime sections take about 14 minutes, the demo about 4 minutes, and the closing about 2 minutes.

- [ ] **Step 9: Commit the complete draft**

```powershell
git add presentation/speech.md
git commit -m "docs: complete mini-docker speaker script"
```

Expected: one commit adding slides 5-10 to the script.

---

### Task 3: Validate the Complete Script

**Files:**
- Modify if needed: `presentation/speech.md`
- Validate: `presentation/speech.md`

**Interfaces:**
- Consumes: the complete script from Tasks 1-2.
- Produces: a structurally complete, technically checked, rehearsal-ready script.

- [ ] **Step 1: Verify all slide sections and total timing**

Run:

```powershell
$speech = Get-Content presentation/speech.md -Raw
$headings = [regex]::Matches($speech, '(?m)^## Slide (\d+) - .+ \((\d+) minutes?\)$')
$numbers = $headings | ForEach-Object { [int]$_.Groups[1].Value }
$minutes = ($headings | ForEach-Object { [int]$_.Groups[2].Value } | Measure-Object -Sum).Sum
if (($numbers -join ',') -ne '1,2,3,4,5,6,7,8,9,10') { throw "Unexpected slide order: $($numbers -join ',')" }
if ($minutes -ne 30) { throw "Unexpected duration: $minutes minutes" }
"Slides: $($headings.Count); duration: $minutes minutes"
```

Expected:

```text
Slides: 10; duration: 30 minutes
```

- [ ] **Step 2: Verify required cues and closing**

Run:

```powershell
$speech = Get-Content presentation/speech.md -Raw
$required = '[CLICK]', '[NEXT SLIDE]', '[PAUSE]', '[POINT TO ', '[SWITCH TO TERMINAL]', '[RETURN TO DECK]', "That's the core of a container. Questions?"
foreach ($item in $required) {
    if (-not $speech.Contains($item)) { throw "Missing required text: $item" }
}
'Required cues and closing are present'
```

Expected: `Required cues and closing are present`.

- [ ] **Step 3: Verify the canonical demo commands**

Run:

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

Expected: `Canonical demo sequence is present`.

- [ ] **Step 4: Measure script length**

Run:

```powershell
$speech = Get-Content presentation/speech.md -Raw
$spoken = $speech -replace '(?s)```.*?```', '' -replace '(?m)^#.*$', '' -replace '\[[A-Z][^\]]*\]', '' -replace '[`*_>#-]', ' '
$words = ([regex]::Matches($spoken, "\b[\p{L}\p{N}][\p{L}\p{N}'/-]*\b")).Count
"Estimated spoken words: $words"
if ($words -lt 3000 -or $words -gt 3900) { throw 'Spoken word count is outside the rehearsal range' }
```

Expected: an estimate between 3,000 and 3,900 spoken words. Trim repetition or add missing causal explanation if outside the range.

- [ ] **Step 5: Run the presentation regression suite**

Run:

```powershell
npm run slides:test
```

Expected: all 9 Playwright tests pass, confirming no presentation asset was unintentionally changed.

- [ ] **Step 6: Check the final diff**

Run:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors; only the speech, design specification, and plan are changed unless earlier user work is already present.

- [ ] **Step 7: Commit validation adjustments**

```powershell
git add presentation/speech.md docs/superpowers/specs/2026-07-17-mini-docker-speech-design.md docs/superpowers/plans/2026-07-17-mini-docker-speech.md
git commit -m "docs: finalize mini-docker talk script"
```

Expected: the final script and its approved planning documents are committed.