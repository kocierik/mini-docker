# Build Your Own Docker Presentation Design

## Purpose

Create an English, 30-minute technical presentation for programmers that explains how this repository implements a constrained Docker-like lifecycle. The talk starts with the familiar `pull -> build -> run` workflow, then spends most of its time on the Linux runtime primitives that turn a root filesystem and a command into an isolated PID 1 process.

Questions happen after the 30-minute presentation. The talk is guided rather than workshop-based: the audience watches a coherent argument, sees four short excerpts from the real codebase, and finishes with a live end-to-end demonstration.

## Central Promise

The opening question is:

> How much Docker do we need to isolate one process?

The closing answer is:

> An image and rootfs, Linux namespaces, a changed root, a private `/proc`, and `exec` are enough to demonstrate the core runtime idea. Docker adds the production safety, resource control, networking, storage, distribution, and operational ergonomics around it.

The presentation must not imply that this project is a secure or production-ready container engine. It demonstrates the mechanism, not full Docker equivalence.

## Audience and Learning Outcomes

The audience consists of programmers who know command-line tools and processes but may not know container internals. Go familiarity is helpful but not required.

By the end, the audience should be able to:

- distinguish an image from a running container;
- describe `pull`, `build`, and `run` as rootfs transformations followed by process creation;
- explain why the runtime re-executes itself into a child process;
- identify what the PID, UTS, and mount namespaces isolate;
- explain the roles of `chroot`, `/proc`, and `syscall.Exec`;
- connect the observed `hostname: demo-box` and `pid: 1` output to specific Linux primitives;
- name major capabilities deliberately absent from this v1.

## Delivery Package

The presentation package will contain:

- a self-contained, offline-capable, 16:9 HTML slide deck;
- English speaker notes with timing, transitions, and demo cues;
- a short demo runbook containing preparation, exact commands, expected output, and fallback procedure;
- static code excerpts copied from the repository and labeled with their source file and symbol;
- captured terminal output for the demo fallback;
- a print stylesheet used to generate a readable PDF backup through the browser's print-to-PDF support.

The deck is operated with keyboard navigation in a browser. It does not depend on network-loaded fonts, scripts, images, or syntax highlighting during the talk.

## Narrative Structure

| Time | Slide | Purpose | Primary visual |
| --- | --- | --- | --- |
| 0:00-2:00 | 1. The question | Establish the promise and define the destination as an isolated process | Title plus terminal proof teaser |
| 2:00-4:00 | 2. The workflow | Separate image preparation from runtime process creation | `pull -> build -> run` pipeline |
| 4:00-7:00 | 3. Pull | Show a remote OCI image becoming a flattened local rootfs and metadata | Terminal, rootfs blueprint, code moment 1 |
| 7:00-10:00 | 4. Build | Show a base rootfs being copied, changed, and published with a recorded command | Dockerfile, transformation blueprint, code moment 2 |
| 10:00-13:00 | 5. What `run` must do | Define the runtime contract before showing implementation | Host process versus desired container view |
| 13:00-17:00 | 6. Re-exec and namespaces | Explain the parent/child boundary and namespace creation | Process blueprint, code moment 3 |
| 17:00-21:00 | 7. Rootfs and `/proc` | Explain mount propagation, hostname, changed root, and process view | Progressive child setup, code moment 4 |
| 21:00-24:00 | 8. PID 1 and `exec` | Show why the payload replaces the bootstrap process and becomes PID 1 | Before/after process tree plus expected observations |
| 24:00-28:00 | 9. Live demo | Prove the complete lifecycle with the real CLI | Full-screen terminal |
| 28:00-30:00 | 10. What Docker adds | State the minimal answer, limitations, and production gap | Minimal core surrounded by omitted capabilities |

Runtime content receives 14 minutes before the demo, from slides 5 through 8. Pull and build establish the lifecycle but remain intentionally high-level.

## Slide System

The visual language is terminal-first, with systems blueprints as the explanatory layer. The deck repeatedly uses an `observe -> explain -> inspect` rhythm:

1. **Observe:** show one command or one visible effect.
2. **Explain:** introduce only the Linux primitive or filesystem transformation needed to explain that effect.
3. **Inspect:** reveal the decisive 6-12 lines of real code.

Each slide has one job. Slides do not combine a full terminal, a full architecture diagram, and a large code listing at the same time. Code is static rather than scrolled live.

Three recurring layouts provide visual continuity:

- **Proof:** a large terminal command and its relevant output, followed by one question or conclusion.
- **Blueprint:** a left-to-right process or filesystem transformation, with only the current primitive highlighted.
- **Code:** a short excerpt with surrounding syntax muted and one semantic decision emphasized.

## Visual Direction

The deck uses a restrained dark terminal surface rather than a generic dark-blue presentation theme. Color carries stable meaning:

- green identifies observed proof and completed outcomes;
- blue identifies image, rootfs, and data transformations;
- yellow identifies runtime boundaries and isolation primitives;
- muted gray-green identifies context that is not currently being discussed.

Typography pairs locally bundled IBM Plex Sans Condensed for titles and explanations with locally bundled IBM Plex Mono for commands and code. Type sizes are fixed by semantic role, not viewport width. The smallest projected code remains readable at 1366x768.

Animations are limited to progressive blueprint reveals and must never be required to understand a slide. There is no simulated terminal typing, decorative gradient, nested-card layout, or ornamental illustration. Every diagram explains a real transition in this implementation.

## Four Code Moments

### 1. Pull: Publish a Flattened Image

Source: `internal/registry/puller.go`, centered on `(*Puller).Pull`.

The excerpt shows the semantic path from a remote OCI reference to a temporary rootfs, extracted contents, copied command metadata, and atomic publication in the local store. Registry protocol details from `go-containerregistry` remain off-slide because they do not advance the runtime argument.

Teaching point: an image is prepared filesystem content plus metadata; it is not a running container.

### 2. Build: Apply a Small Dockerfile Sequentially

Source: `internal/builder/builder.go`, centered on `(*Builder).Build`.

The excerpt shows the instruction switch for `COPY`, `RUN`, and `CMD`. The slide pairs it with the four-line demo Dockerfile. The audience sees that `RUN` uses the same runtime path later used by the public `run` command, while `CMD` records metadata rather than executing immediately.

Teaching point: build derives another image; it does not create the final long-lived container process.

### 3. Parent: Re-exec with New Namespaces

Sources: `internal/runtime/runner.go` and `internal/runtime/runner_linux.go`, centered on `runRootFS` and `configureChildCommand`.

The excerpt shows the executable re-launch and these clone flags:

```go
syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS
```

The blueprint distinguishes the original host process from the child created inside new namespace views.

Teaching point: namespace membership is established when the child is created; it is not added later by `chroot`.

### 4. Child: Construct the Container View and Replace Bootstrap

Source: `internal/runtime/runner_linux.go`, centered on `runInternalChild`.

The excerpt uses two fixed fragments on the same slide and reveals them in execution order. The first shows private mount propagation, hostname, `chroot`, and the directory change to `/`. The second shows the `/proc` mount and `syscall.Exec`. Together they remain one code moment and one conceptual sequence.

Teaching point: the bootstrap process prepares the isolated view, and `exec` replaces it with the configured payload so that payload becomes PID 1 in the new PID namespace.

## Runtime Explanation Model

The runtime is presented as a parent/child sequence rather than as a list of syscalls:

```text
host CLI
  -> resolve image, rootfs, and command
  -> clone a writable runtime rootfs
  -> re-exec mini-docker with PID + UTS + mount namespaces
       -> make mount propagation private
       -> set hostname
       -> chroot and chdir /
       -> mount a namespace-aware /proc
       -> exec the payload as PID 1
```

The explanation explicitly distinguishes:

- PID namespace: changes process IDs and makes the payload PID 1;
- UTS namespace: gives the child its own hostname;
- mount namespace: isolates mount operations such as the new `/proc`;
- `chroot`: changes filesystem path resolution but is not a namespace or a complete security boundary;
- `/proc`: must be mounted after entering the relevant namespaces so it reports the container process view;
- `exec`: replaces the bootstrap program instead of leaving it as an unnecessary parent.

## Live Demo Choreography

The demo occupies four minutes and uses a full-screen WSL terminal at a projection-safe font size. The editor and slide deck are not switched into during execution.

Before the audience enters:

- build `bin/mini-docker`;
- verify WSL networking and the Alpine reference;
- warm `sudo` credentials;
- verify that `sudo -E` preserves `MINI_DOCKER_HOME`;
- keep a known-good prepared image store separate from the fresh demo store;
- record the expected terminal session as the fallback asset.

At the start of the demo, create a fresh store and run the repository's documented commands:

```bash
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo -E ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo -E ./bin/mini-docker run --hostname demo-box hello:latest
```

Timing and narration:

- **0:00-0:20:** establish the fresh image store.
- **0:20-1:10:** pull Alpine; describe remote OCI content becoming a local rootfs.
- **1:10-2:20:** build `hello:latest`; describe the derived filesystem and persisted `CMD`.
- **2:20-3:05:** type the run command, pause before Enter, and ask what the process should observe.
- **3:05-4:00:** connect `hostname: demo-box`, `pid: 1`, and the root entries to UTS, PID, mount, rootfs, `/proc`, and `exec`.

Expected proof:

```text
hello from mini-docker
hostname: demo-box
pid: 1
root entries: bin dev etc hello.sh lib proc tmp usr ...
```

The root entry list may vary slightly. The stable assertions are the greeting, hostname, and PID.

If any command fails to make useful progress for ten seconds, switch immediately to the captured terminal output and explain the same evidence. Do not debug networking, privileges, namespaces, or line endings on stage.

## Speaker Notes

Speaker notes are written in English and organized per slide. Each slide entry includes:

- target start and end time;
- one sentence stating the slide's job;
- the spoken explanation, sized to the available time;
- the exact transition into the next slide;
- reveal or code-highlight cues where applicable;
- the demo pause and fallback cues for slide 9.

Notes do not duplicate every visible word. They explain causality, define terms, and prevent omissions. The opening and closing lines are scripted exactly; the middle remains conversational enough to deliver naturally.

## Scope Boundaries

The presentation does not include:

- a workshop or audience coding exercise;
- live source editing or scrolling through large files;
- a detailed OCI registry protocol explanation;
- a complete Dockerfile parser walkthrough;
- claims of production security or Docker compatibility;
- networking, cgroups, overlay filesystems, volumes, image layer caching, or orchestration implementation details;
- live debugging as part of the planned demo.

Those omitted capabilities appear only on the closing comparison slide to clarify what real Docker adds.

## Validation

The finished package is accepted when all of the following are true:

- the deck runs without network access;
- keyboard navigation reaches every slide and reveal in order;
- no text, diagram, terminal output, or code excerpt overflows at 1920x1080 or 1366x768;
- commands and code are readable from a projected 1366x768 viewport;
- every code excerpt matches the named repository symbol;
- the PDF backup preserves layout, contrast, and code readability;
- a timed rehearsal finishes between 28 and 30 minutes without questions;
- at least 14 minutes before the demo are spent on slides 5-8;
- the exact demo commands succeed in WSL2 with a fresh store;
- the captured-output fallback can replace the live terminal within ten seconds;
- the closing slide explicitly distinguishes this educational runtime from production Docker.

## Approved Decisions

- Format: 30-minute guided English technical talk, with questions afterward.
- Narrative: proof and workflow first, runtime internals second, live demo after the mechanisms are understood.
- Visual direction: terminal-first with systems blueprints as supporting explanation.
- Slide rhythm: `observe -> explain -> inspect`.
- Code density: four short, static code moments rather than editor browsing.
- Demo: complete `pull -> build -> run` lifecycle live, with a prepared environment and captured-output fallback after ten seconds.