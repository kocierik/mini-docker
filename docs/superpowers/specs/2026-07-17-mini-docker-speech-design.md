# Mini-Docker Speech Design

## Goal

Create a complete English speaker script for the approved ten-slide
"Build Your Own Docker" presentation. The script must explain the causal path
from `pull` through `build` to `run`, with runtime internals as the technical
center of the talk, while remaining natural enough to deliver without sounding
read aloud.

## Audience and Delivery

- Audience: programmers familiar with Docker as users, but not necessarily with
  Linux container internals.
- Duration: 30 minutes, followed by questions.
- Language: clear international English.
- Tone: conversational and technical, with short sentences, rhetorical
  questions, and no forced jokes.
- Form: a complete slide-by-slide speaker script rather than bullet notes.

## Structure

Each slide section includes:

- the slide number, title, and target duration;
- the complete spoken text;
- unobtrusive stage cues such as `[CLICK]`, `[PAUSE]`, `[POINT TO CODE]`, and
  `[SWITCH TO TERMINAL]`;
- an explicit transition into the next slide.

The timing follows the approved deck:

- Slides 1-2: question and lifecycle, 4 minutes.
- Slides 3-4: pull and build, 6 minutes.
- Slides 5-8: runtime internals, 14 minutes.
- Slide 9: live demo, 4 minutes.
- Slide 10: conclusion, 2 minutes.

The target length is approximately 3,500 words, adjusted where stage actions
and terminal output consume speaking time.

## Narrative

The script follows the deck's `observe -> explain -> inspect` rhythm. It starts
with the question "How does Docker work?", narrows the problem to one process,
and then builds the answer in three stages:

1. `pull` turns OCI layers and metadata into a local image.
2. `build` transforms that image according to a Dockerfile.
3. `run` creates a process with different namespace views, a different root,
   its own `/proc`, and the requested command as PID 1 inside the new PID
   namespace.

The speech must explain why each mechanism is needed before inspecting its code.
It must not expand into a survey of every production Docker subsystem.

## Technical Guardrails

The script must preserve these distinctions:

- namespaces are selected when the child process is created;
- PID 1 means PID 1 inside the new PID namespace;
- mount propagation is made private before changing the child's mount view;
- `chroot` changes pathname resolution but is not a complete security boundary;
- `/proc` is mounted after entering the namespace and root filesystem views;
- `exec` replaces the bootstrap process rather than starting another child;
- this project demonstrates the core mechanism and is not production-ready.

Code explanations must match the source excerpts already shown by the deck.

## Demo Choreography

Slide 9 uses the canonical command sequence from the slide, README, and demo
runbook. The script includes cues for switching to the terminal, waiting for
commands, highlighting the four observable proofs, and returning to the deck.
If the live command fails or runs long, the speaker switches to the captured
fallback output without diagnosing the environment on stage.

## Closing

The conclusion answers the opening question in concrete terms: a container is a
normal Linux process launched with constrained views and a prepared filesystem.
It then names the production capabilities intentionally omitted by the project,
states the security limitation, and ends with:

> That's the core of a container. Questions?

## Deliverable and Validation

The full script will live in `presentation/speech.md`. Validation consists of:

- checking that all ten slides appear in order;
- checking the timing totals 30 minutes;
- checking that every fragment, code inspection, terminal switch, and slide
  transition has an appropriate cue;
- checking the demo commands against the canonical runbook;
- checking all runtime claims against the implementation and deck;
- estimating spoken duration and trimming prose if necessary.