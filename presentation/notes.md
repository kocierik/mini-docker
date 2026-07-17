# Build Your Own Docker Speaker Notes

## Slide 1 · The Question · 0:00-2:00
**Job:** Promise an executable answer to “How does Docker work?”

Open exactly: “How does Docker work?” Explain that the terminal proof is the destination, not yet the explanation. A process will report a private hostname, PID 1, and an image-derived filesystem.

**Transition:** “First, separate what we prepare from what we run.”

## Slide 2 · The Workflow · 2:00-4:00
**Job:** Separate images from containers.

Pull prepares a local rootfs. Build derives another rootfs and records a command. Run combines those inputs to start an isolated process. An image is input; a container is a process.

**Transition:** “Let us manufacture the input first.”

## Slide 3 · Pull · 4:00-7:00
**Job:** Show remote OCI content becoming local filesystem content and metadata.

Run the command visually, reveal the flattening blueprint, then reveal `extractRootFS` and `store.Put`. Do not explain registry authentication, manifests, or transport.

**Transition:** “Build starts from that local result.”

## Slide 4 · Build · 7:00-10:00
**Job:** Distinguish immediate filesystem mutation from recorded runtime metadata.

Read the four Dockerfile lines in order. `COPY` changes the rootfs, `RUN` executes now through the runtime path, and `CMD` records what executes later.

**Transition:** “Now we have the two inputs: a rootfs and a command.”

## Slide 5 · Runtime Contract · 10:00-13:00
**Job:** Define what the process must observe before introducing syscalls.

Walk through hostname, PID, filesystem root, and process view. Ask which Linux mechanism could change each observation. State that there is no single container syscall.

**Transition:** “Those views begin at process creation.”

## Slide 6 · Re-exec and Namespaces · 13:00-17:00
**Job:** Explain the parent/child boundary and namespace creation.

Reveal the process flow before the code. Explain `/proc/self/exe`, encoded child configuration, and the clone flags. PID changes process IDs, UTS changes hostname, and mount isolates mount operations. Namespace membership exists before `chroot`.

**Transition:** “The child exists; now it must construct its view.”

## Slide 7 · Rootfs and /proc · 17:00-21:00
**Job:** Follow child setup in execution order.

First make mount propagation private, set the hostname, change the root, and `chdir` to `/`. Then create and mount `/proc` inside the mount/PID view. `chroot` changes path resolution but is not a complete security boundary.

**Transition:** “The final call removes mini-docker from the process tree.”

## Slide 8 · PID 1 and exec · 21:00-24:00
**Job:** Connect `exec` to the observable PID 1 result.

`exec` replaces the child program without creating a new process. The payload keeps the identity that is PID 1 inside the namespace. Connect `demo-box` to UTS, `pid: 1` to PID, and `/proc` plus root entries to mount/rootfs setup.

**Transition:** “Now let the real program prove every claim.”

## Slide 9 · Live Demo · 24:00-28:00
**Job:** Prove the complete lifecycle.

Switch to the prepared WSL terminal. Pull at 24:20, build at 25:10, and type run at 26:20. Pause before Enter and ask what the process should observe. Explain the stable output from 27:05. After ten seconds without useful progress, switch to captured output and do not debug.

**Transition:** “That is the minimal mechanism. Now draw the production boundary.”

## Slide 10 · What Docker Adds · 28:00-30:00
**Job:** Give the minimal answer without claiming production equivalence.

Close exactly: “An image and rootfs, Linux namespaces, a changed root, a private `/proc`, and `exec` are enough to demonstrate the core runtime idea. Docker adds the production system around it.” Name cgroups, networking, storage, volumes, hardening, distribution, and orchestration. Invite questions after the close.

## Rehearsal Clock

| Slides | Duration |
| --- | ---: |
| 1-2 | 4:00 |
| 3-4 | 6:00 |
| 5-8 | 14:00 |
| 9 | 4:00 |
| 10 | 2:00 |
| **Total** | **30:00** |