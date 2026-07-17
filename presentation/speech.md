# Build Your Own Docker - Speaker Script

## Cue Legend

- `[CLICK]`: advance the current slide fragment.
- `[NEXT SLIDE]`: move to the next slide.
- `[PAUSE]`: leave a short beat for the audience.
- `[POINT TO ...]`: direct attention to a visible element.
- `[SWITCH TO TERMINAL]`: leave the deck for the live demo.
- `[RETURN TO DECK]`: return to the presentation.

## Slide 1 - How does Docker work? (2 minutes)

How does Docker work?

[PAUSE]

That question can lead to a very long list of features and commands. But listing features is not the same as understanding a container. The useful question is more precise: how can an ordinary process start and observe a machine that looks different from the host around it?

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

Now we have the two inputs: a rootfs and a command.

[NEXT SLIDE]