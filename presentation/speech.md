# Build Your Own Docker - Speaker Script

## Cue Legend

- `[CLICK]`: advance the current slide fragment.
- `[NEXT SLIDE]`: move to the next slide.
- `[PAUSE]`: leave a short beat for the audience.
- `[POINT TO ...]`: direct attention to a visible element.
- `[SWITCH TO TERMINAL]`: leave the deck for the live demo.
- `[RETURN TO DECK]`: return to the presentation.
- `[RUN SETUP COMMAND]`: execute the environment setup command.
- `[RUN PULL]`: execute the pull command.
- `[RUN BUILD]`: execute the build command.
- `[RUN - PAUSE BEFORE ENTER]`: prepare the run command, then pause before pressing Enter.

## Slide 1 - How does Docker work? (2 minutes)

How does Docker work?

[PAUSE]

That question can lead to a very long list of features and commands. But listing features is not the same as understanding a container. The useful question is more precise: how can we start an ordinary process that observes an environment that looks different from the host around it?

[POINT TO TERMINAL]

This small result is where we are going. We will start one process. From inside, it will see the hostname `demo-box`. It will report process ID 1. Its files will come from an image rather than from the host filesystem we launched it on.

Those observations are the proof. They are not yet the explanation. I do not want to hide that explanation behind a metaphor about lightweight virtual machines, and I do not want to tour every subsystem in Docker. Instead, we will follow one process through one narrow path: `pull`, then `build`, then `run`.

At each step, we will ask what data exists, what changes it, and what the next step receives. By the end, the terminal output should no longer look surprising. We should be able to connect each observation to a concrete decision in this implementation.

So first, let us separate the material we prepare from the process we eventually start.

[NEXT SLIDE]

## Slide 2 - One lifecycle, three jobs (2 minutes)

The lifecycle has three verbs, but they do three different jobs.

[POINT TO PULL]

`pull` starts with remote image content. Its job is to make that content usable locally as a root filesystem: a directory tree that could become `/` for a process. Nothing is running in that image. We are preparing files and preserving the command metadata that came with them.

[POINT TO BUILD]

`build` starts from that local root filesystem. It makes a derived copy, applies our Dockerfile instructions to the copy, and records the command selected by the Dockerfile. The output is another image: changed files plus metadata. It is still input, not a container.

[POINT TO RUN]

Only `run` crosses from prepared data to a running process. It combines a root filesystem with a command and starts that command so that it observes the environment we asked for.

That gives us the distinction we will use throughout the talk: an image is input; a container is a running process. The same image can be stored without any container, and it can be used to start more than one process. Calling all of these things "Docker" hides the causal boundary we need to understand.

There is also an unresolved question in this pipeline. Pull can prepare files. Build can derive files and choose a command. But how can run make that command see PID 1, a private hostname, and the image filesystem as its root? We will leave that question open for a moment. Before we explain the process, we need to manufacture its two inputs.

Let us manufacture the input first.

[NEXT SLIDE]

## Slide 3 - Pull: from OCI layers to a local image (3 minutes)

We begin with `mini-docker pull alpine:3.22`. The result is a local image, but it is important not to picture an image as a virtual machine disk. There is no complete guest machine waiting to boot inside one large disk file.

An OCI image arrives as an ordered sequence of layers. Each layer describes filesystem changes relative to the layers before it: files added, replaced, or removed. Order matters because a later change is applied on top of the earlier result.

[CLICK]

[POINT TO BLUEPRINT]

This implementation flattens those ordered changes. It extracts the combined archive into a temporary `rootfs` directory. After extraction, the useful result is an ordinary directory tree: the files a future process should see. Alongside those files, the image configuration supplies command metadata. That command is part of image behavior, even though pulling it does not execute anything.

Now let us inspect the publication boundary.

[CLICK]

[POINT TO CODE]

The excerpt begins by joining the temporary directory with `rootfs`. `os.Mkdir` creates that destination with mode `0755`. If creation fails, Pull returns an error wrapped as `create temporary rootfs`; it cannot continue with nowhere safe to extract.

Next, `extractRootFS` applies the archive to that directory. Again, failure returns immediately, this time as `extract image`. The code does not publish whatever partial tree happens to exist.

Only after extraction succeeds does it construct the metadata. The name is the parsed OCI reference, and `slices.Clone(configFile.Config.Cmd)` copies the configured command rather than retaining shared slice storage. So the local image has both the flattened filesystem and its default command.

Finally, the code calls `puller.store.Put` with the context, the parsed image name, the completed rootfs, and that metadata. `store.Put` stages the stored object and publishes its reference with atomic filesystem renames. That ordering is the important guarantee here: callers cannot resolve a half-extracted image under the requested name. If publication fails, Pull returns `publish image` instead of reporting success.

Notice what the excerpt deliberately does not need to explain. Registry transport has already produced the archive and configuration. For our lifecycle, the decisive step is turning remote, layered content into complete local filesystem content and metadata, then making that result visible only after it is ready.

Now we have a local image. How do we change it with a Dockerfile?

[NEXT SLIDE]

## Slide 4 - Build: turn instructions into an image (3 minutes)

Build starts from the local result of Pull. On the left is the entire Dockerfile for the demo, so we can follow it in order.

[POINT TO DOCKERFILE]

`FROM alpine:3.22` selects the base image and therefore the starting root filesystem and inherited metadata. `COPY hello.sh /hello.sh` adds our script to that filesystem. `RUN chmod +x /hello.sh` changes the copied file during the build. `CMD ["/hello.sh"]` selects the command that a future Run should start.

Those four lines mix two kinds of instruction, and the timing difference matters.

[CLICK]

[POINT TO BLUEPRINT]

`COPY` and `RUN` mutate the build rootfs now. `CMD` does not run the script now. It records future behavior in the derived image's metadata. So Build produces another root filesystem plus a command; it does not start the final container.

The implementation makes that sequence explicit.

[CLICK]

[POINT TO CODE]

The builder walks the parsed instructions in order and switches on their concrete type. In the `COPY` branch, `copyFileIntoRootFS` receives both the context directory and the build rootfs. That boundary matters: the source comes from the build context, while the destination is resolved inside the filesystem being built. If the guarded copy returns an error, Build returns that error immediately. It does not continue to `RUN` or publish an incomplete image.

In the `RUN` branch, the executor receives the build rootfs and the command `[/bin/sh, -c, instruction.Command]`. For this Dockerfile, that means executing `chmod +x /hello.sh` against the build rootfs. The command runs now because its filesystem effects must be captured in the image we are constructing. If execution fails, the guard returns the error and the later instructions do not run.

Then the `CMD` branch does something intentionally quieter. It copies the command into `metadata.Command` with `append([]string(nil), instruction.Command...)`. There is no executor call in this branch. `/hello.sh` is recorded for later; it is not launched during Build.

That distinction prevents a common misunderstanding. `RUN` may use the same execution path that can run a root filesystem, but during Build it is a tool for changing the build result. The final application process has not started. `CMD` is the hand-off from image construction to that future process.

At this point the preparation side is complete. We have the two inputs shown at the start: a root filesystem and a command. The remaining question is the runtime contract: what must Run do so one process observes that filesystem as its root, the requested hostname, and PID 1?

[NEXT SLIDE]

## Slide 5 - What must run change? (3 minutes)

Before reading the runtime code, let us define what success means from the process's point of view.

[POINT TO HOSTNAME]

First, the process must observe the hostname `demo-box`, while the host keeps its own hostname. That is a promise about the system identity visible to this process.

[POINT TO PROCESS ID]

Second, the process must observe its own process ID as 1. That does not mean it has host PID 1. It means PID 1 inside a different process-ID view.

[POINT TO FILESYSTEM ROOT]

Third, when the process opens `/`, it must reach the image rootfs rather than the host's root directory. `/hello.sh` must name the file added during Build, and ordinary absolute paths must resolve inside that prepared tree.

[POINT TO PROCESS VIEW]

Fourth, `/proc` must describe processes from the new PID view. Merely carrying the host's existing `/proc` into another directory tree would contradict the PID promise, because the files under `/proc` would still expose the wrong process view.

These are four observable promises: a hostname, a process ID, a filesystem root, and a process view. They are different views presented to one ordinary Linux process. They are not evidence of a guest kernel booting somewhere. The process still uses the host Linux kernel; the kernel changes what that process can observe.

There is no single `container` syscall that produces all four results. We need separate mechanisms with separate jobs. A UTS namespace can isolate the hostname. A PID namespace can isolate process IDs. A mount namespace can isolate mount operations. `chroot` can change pathname resolution. A new proc filesystem can report the process view from inside those namespaces.

That list is not yet an execution plan. In particular, some views must exist when the child is created, while the filesystem setup must happen inside that child afterward. The code therefore needs a boundary: create a child with the namespace memberships first, then let that child assemble the world the payload will inherit.

Those views begin at process creation.

[NEXT SLIDE]

## Slide 6 - Create the boundary before entering it (4 minutes)

[POINT TO PROCESS FLOW]

Start with the process flow, before the code. The host CLI does not turn itself directly into `/hello.sh`. It starts another copy of the mini-docker executable. That child performs the privileged bootstrap work, and only after the environment is ready does it become the payload.

Why launch the same executable again? The namespace flags are applied when this child process is created. Re-executing mini-docker gives the new child a known entry point that can run the setup code from inside those new namespace views. It also leaves the original CLI outside, where it can wait for the child and return its exit status.

On Linux, `childExecutablePath()` returns `/proc/self/exe`. That path resolves to the executable for the currently running mini-docker process. The parent can therefore start the same binary without depending on the command name or the path used to invoke it.

The argument `__child` selects an internal child mode. The rootfs path, requested hostname, and command are encoded as JSON and carried in the `MINI_DOCKER_CHILD_CONFIG` environment variable. When the new mini-docker process starts, `HandleInternalChild` recognizes that private argument, decodes the configuration, and calls the child setup path. This is configuration transport between the parent and its bootstrap child, not image metadata and not an extra container process.

[CLICK]

[POINT TO NAMESPACE KEY]

Now reveal the three namespace choices. `CLONE_NEWPID` gives the child a new process-ID namespace. The child is the first process in that namespace, so it observes itself as PID 1 there; it still has a different PID when viewed from the host. `CLONE_NEWUTS` gives it a separate UTS namespace, where changing the hostname does not rename the host. `CLONE_NEWNS` creates a separate mount namespace, so the child's mount operations can change its mount view without changing the host's mount namespace.

These flags do not set the hostname, change the root, or mount `/proc` by themselves. They establish the boundaries in which the child can do those things. Namespace membership therefore exists before `chroot`; `chroot` is not what creates a namespace.

[CLICK]

[POINT TO LEFT CODE EXCERPT]

The first source excerpt is the parent side in `runRootFS`. `exec.CommandContext` prepares a child command using `/proc/self/exe` and the internal argument. Standard input, output, and error are connected to the current terminal, which is why the payload can behave like the command we launched. The environment carries the encoded child configuration.

[POINT TO RIGHT CODE EXCERPT]

The second excerpt is the Linux-specific creation policy. `SysProcAttr.Cloneflags` combines exactly the PID, UTS, and mount namespace flags we just named. When `command.Run` starts the child, the kernel creates it with those namespace memberships. The parent has now created the boundary, but the child's filesystem view is not ready yet.

The child exists; now it must construct its view.

[NEXT SLIDE]

## Slide 7 - Assemble the child's world (4 minutes)

The child setup is order-sensitive. Each operation prepares an assumption used by the next one, so let us follow the first fragment from top to bottom.

[POINT TO EXACT FRAGMENT 1]

First, the child recursively changes mount propagation at `/` to private with `MS_REC | MS_PRIVATE`. A mount namespace gives this child a separate mount view, and private propagation prevents later mount events in this tree from propagating back through shared mount relationships. That must happen before the child mounts its own `/proc`.

Second, if a hostname was requested, `syscall.Sethostname` sets it. Because the child already belongs to the new UTS namespace, this changes the hostname observed in that namespace, not the host's hostname. If no hostname was configured, the code skips this call.

Third, `syscall.Chroot(config.RootFS)` changes how this process resolves `/` and absolute paths. The immediately following `os.Chdir("/")` moves its current working directory into that new root. Both steps matter: changing the root does not by itself move an existing working directory.

This is where we must not overclaim security. `chroot` changes pathname resolution for the process, but it is not a complete security boundary. This educational runtime combines it with namespaces to demonstrate the filesystem view; production isolation needs substantially more hardening.

At this point, `/hello.sh` resolves inside the prepared image rootfs. But the process view is still incomplete. A rootfs directory named `/proc` is only a mount point. It does not automatically become a proc filesystem, and reusing a proc mount created from the host view would expose the wrong information.

[CLICK]

[POINT TO EXACT FRAGMENT 2]

The second fragment continues in exact execution order. `os.MkdirAll` ensures that `/proc` exists inside the changed root. Then `syscall.Mount` mounts a new filesystem of type `proc` there. This call happens after creation of the PID and mount namespaces and after `chroot`, so the mount is placed in the child's mount view at the path the payload will call `/proc`, and proc reports the new PID namespace's process view. The `MS_NOSUID`, `MS_NOEXEC`, and `MS_NODEV` flags restrict how that proc mount can be used.

Only now are all four runtime promises assembled. The child has its namespace memberships, optional hostname, changed root, current directory, and namespace-aware `/proc`.

The final operation is `syscall.Exec`. Its first argument is the configured executable path. The full command slice becomes the argument vector, and the current environment is passed to the new program. If the executable does not exist inside the changed root, the code reports that it was not found inside the root filesystem. Other failures are reported as an exec failure.

On success, `syscall.Exec` does not return. The next slide is about why that detail produces the result we want.

The final call preserves the process, its PID, its parent relationship, and its position in the process tree while replacing the mini-docker program image with the payload.

[NEXT SLIDE]

## Slide 8 - Exec makes the payload PID 1 (3 minutes)

[POINT TO BEFORE EXEC]

Immediately before `exec`, the mini-docker child is already PID 1 inside the new PID namespace. It has done the bootstrap work, but it is still running the mini-docker program.

[POINT TO AFTER EXEC]

`exec` is process replacement, not process creation. The kernel keeps the process and replaces its program image with `/hello.sh`. There is no extra payload child and no bootstrap parent left inside the namespace. The Go bootstrap code is gone, and a successful `syscall.Exec` never returns to run the lines after it.

Because the process identity continues across `exec`, the payload inherits the namespace memberships and filesystem context that the bootstrap prepared. The same process that was PID 1 while running mini-docker is now PID 1 while running `/hello.sh`. From the host it remains an ordinary process with a host-visible PID. From inside its PID namespace it is PID 1.

That is why the phrase "the payload becomes PID 1" is precise only when we include "inside the new PID namespace." We have not replaced the host's init process, and we have not started a guest kernel.

[CLICK]

[POINT TO TERMINAL OUTPUT]

Now connect the visible results back to the four promises.

`hostname: demo-box` confirms the expected hostname observation. The preceding source inspection provides the causal mechanism evidence: the child is created in a new UTS namespace, then `Sethostname` changes the hostname observed there.

`pid: 1` confirms the expected process-ID observation. The source inspection showed the mechanism: the child is created first in a new PID namespace, then `exec` preserves that process identity while replacing the bootstrap program with the payload. It is not a PID assigned by `chroot` or by the script.

The root entries show the expected image-derived root view, including `hello.sh` and the image directories. The source inspection established the mechanism: `chroot` changed absolute pathname resolution to the prepared rootfs, and `chdir` moved into it.

The `proc` entry marks the path prepared for `/proc`. The listing alone cannot prove that a filesystem is mounted there; the source we just inspected establishes that it is a new proc filesystem mounted after namespace creation. Programs reading it therefore see the PID namespace's process view rather than a proc tree inherited from the host.

One ordinary Linux process now observes all four views we promised. Namespace creation established the isolation boundary, child setup constructed the view, and `exec` installed the real payload without adding another process.

Now let the real program demonstrate those observable results.

[NEXT SLIDE]

## Slide 9 - Live demo: pull, build, run (4 minutes)

[SWITCH TO TERMINAL]

We will run the complete path with a fresh local store.

[RUN SETUP COMMAND]

```bash
export MINI_DOCKER_HOME="$(mktemp -d)"
```

First, this environment variable gives the demo an empty store, so the lifecycle starts without relying on an earlier pull or build.

[RUN PULL]

```bash
./bin/mini-docker pull alpine:3.22
```

[FALLBACK - AFTER TEN SECONDS WITHOUT USEFUL PROGRESS]

For this command and each remaining live command, if it fails or runs long, show `presentation/assets/demo-output.txt` and continue with the captured output. Do not debug networking, sudo, namespaces, or line endings on stage.

Pull turns remote OCI content into the local Alpine rootfs and its command metadata.

[RUN BUILD]

```bash
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
```

Build copies that rootfs, adds `hello.sh`, runs `chmod` against the build filesystem, and persists `/hello.sh` as the command for the derived image.

[RUN - PAUSE BEFORE ENTER]

```bash
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
```

Before I press Enter on Run, what should this one process observe?

[PAUSE]

Let the command run. We do not need to fill every pause; the output is the proof.

```text
hello from mini-docker
hostname: demo-box
pid: 1
root entries: bin dev etc hello.sh lib proc tmp usr ...
```

There are four useful checks here. `hello from mini-docker` confirms that the recorded command ran as the payload. `hostname: demo-box` and `pid: 1` confirm the expected hostname and process-ID observations. The root entries show the expected image-derived root view because they include our script and the image directories. They also show the `/proc` mount point, but neither the listing nor the other output independently proves how those views were produced. The runtime source we inspected supplies that causal evidence: namespace creation, child setup, `chroot`, the procfs mount, and `exec`. The exact directory list can vary slightly, but the expected observations and the inspected mechanisms remain connected.

[RETURN TO DECK]

That is the minimal mechanism. Now draw the production boundary.

[NEXT SLIDE]

## Slide 10 - The minimal answer (2 minutes)

So, how does Docker work at its core?

In the mechanism we have just demonstrated, a container is a normal Linux process launched with namespace isolation, a prepared root filesystem, a mounted `/proc`, and a payload installed by replacing the bootstrap program with `exec`.

The image side prepares files and command metadata. The runtime side creates PID, UTS, and mount namespace views. The child changes pathname resolution to the rootfs, mounts proc from inside the new view, and replaces itself with the configured command. The result is not a small virtual machine. It is a process using the host kernel while observing a deliberately constructed environment.

[CLICK]

[POINT TO OMITTED CAPABILITIES]

That explains the core mechanism, not the complete Docker product. Production container systems add cgroups for resource control, networking, layer storage, volumes, security hardening, distribution, and orchestration. They also need robust lifecycle management and operational behavior around those capabilities.

This project is educational. It demonstrates how a rootfs and Linux process primitives can produce the observations we saw, but it is not a secure, production-ready container engine and it does not claim Docker compatibility.

An image and rootfs, Linux namespaces, a changed root, a private `/proc`, and `exec` are enough to demonstrate the core runtime idea. Docker adds the production system around it.

[PAUSE]

That's the core of a container. Questions?