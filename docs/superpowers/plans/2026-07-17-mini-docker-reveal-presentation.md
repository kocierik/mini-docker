# Mini Docker Reveal.js Presentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an offline Reveal.js slide deck, speaker notes, demo runbook, fallback output, and PDF export for the approved 30-minute “Build Your Own Docker” presentation.

**Architecture:** Vite serves and bundles a single Reveal.js deck under `presentation/`. Slide content stays in semantic HTML so terminal proofs, systems blueprints, and short Go excerpts can use purpose-built layouts; Reveal.js supplies navigation, fragments, speaker notes, and print mode. Playwright treats the built deck as user-facing software by verifying content, keyboard navigation, local-only assets, two projection viewports, and PDF generation.

**Tech Stack:** Node.js 22.12 or newer, npm, Reveal.js 6.0.1, Vite 8.1.5, Playwright 1.61.1, IBM Plex Sans Condensed 5.2.8, IBM Plex Mono 5.2.7, HTML, CSS, JavaScript.

## Global Constraints

- The talk is English and lasts 30 minutes excluding questions.
- The deck contains exactly ten top-level slides in the approved narrative order.
- Slides 5-8 reserve 14 minutes for runtime internals before the demo.
- Use the terminal-first `observe -> explain -> inspect` rhythm.
- Show exactly four static code moments, each limited to the decisive 6-12 visible lines or two fixed fragments for `runInternalChild`.
- Use only locally bundled runtime assets; the deck must work with network access disabled.
- Use IBM Plex Sans Condensed for prose and IBM Plex Mono for commands and code.
- Green means observed proof, blue means image/rootfs transformation, yellow means runtime boundary, and gray-green means context.
- Do not add simulated typing, live source editing, decorative gradients, nested cards, or ornamental illustrations.
- The stable demo assertions are `hello from mini-docker`, `hostname: demo-box`, and `pid: 1`.
- Preserve the user-approved opening question: “How does Docker work?”
- Pass `MINI_DOCKER_HOME` explicitly through `sudo env`; do not rely on broad environment preservation.

---

### Task 1: Reveal.js Foundation and Smoke Test

**Files:**
- Create: `package.json`
- Create: `package-lock.json` through `npm install`
- Create: `presentation/index.html`
- Create: `presentation/main.js`
- Create: `presentation/theme.css`
- Create: `presentation/playwright.config.js`
- Create: `presentation/tests/deck.spec.js`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: no presentation files; the existing Go module remains independent.
- Produces: `window.presentationDeck` as the initialized Reveal instance; npm scripts `slides:dev`, `slides:build`, `slides:test`, and `slides:pdf`; ten top-level `<section data-slide="...">` elements.

- [ ] **Step 1: Add the package manifest and test configuration**

Create `package.json`:

```json
{
  "name": "mini-docker-presentation",
  "private": true,
  "type": "module",
  "engines": {
    "node": ">=22.12.0"
  },
  "scripts": {
    "slides:dev": "vite presentation --host 127.0.0.1",
    "slides:build": "vite build presentation --outDir ../dist/presentation",
    "slides:test": "playwright test -c presentation/playwright.config.js",
    "slides:pdf": "node presentation/scripts/export-pdf.mjs"
  },
  "dependencies": {
    "@fontsource/ibm-plex-mono": "5.2.7",
    "@fontsource/ibm-plex-sans-condensed": "5.2.8",
    "reveal.js": "6.0.1"
  },
  "devDependencies": {
    "@playwright/test": "1.61.1",
    "vite": "8.1.5"
  }
}
```

Create `presentation/playwright.config.js`:

```js
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  use: { baseURL: 'http://127.0.0.1:4173' },
  webServer: {
    command: 'npm run slides:dev -- --port 4173',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: true,
  },
});
```

Append these entries to `.gitignore`:

```gitignore
/node_modules/
/dist/
/test-results/
/playwright-report/
```

Run: `npm install`

Expected: dependencies install and `package-lock.json` is created with no floating package versions in `package.json`.

- [ ] **Step 2: Write the failing deck smoke test**

Create `presentation/tests/deck.spec.js`:

```js
import { expect, test } from '@playwright/test';

test('loads the ten-slide offline deck', async ({ page }) => {
  await page.goto('/');

  await expect(page).toHaveTitle('Build Your Own Docker');
  await expect(page.locator('.reveal .slides > section')).toHaveCount(10);
  await expect(page.locator('[data-slide="question"] h1')).toContainText('How does Docker work?');

  const externalResources = await page.locator('[src^="http"], [href^="http"]').count();
  expect(externalResources).toBe(0);
});

test('supports keyboard navigation', async ({ page }) => {
  await page.goto('/');
  await page.keyboard.press('ArrowRight');
  await expect(page.locator('[data-slide="workflow"]')).toHaveClass(/present/);
});
```

- [ ] **Step 3: Run the smoke test to verify it fails**

Run: `npx playwright install chromium && npm run slides:test -- --grep "ten-slide offline deck"`

Expected: FAIL because `presentation/index.html` does not exist.

- [ ] **Step 4: Implement the minimal Reveal.js shell**

Create `presentation/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Build Your Own Docker</title>
  </head>
  <body>
    <div class="reveal">
      <div class="slides">
        <section data-slide="question"><h1>How does Docker work?</h1></section>
        <section data-slide="workflow"><h2>Pull. Build. Run.</h2></section>
        <section data-slide="pull"><h2>Pull</h2></section>
        <section data-slide="build"><h2>Build</h2></section>
        <section data-slide="run-contract"><h2>What must run do?</h2></section>
        <section data-slide="namespaces"><h2>Re-exec + namespaces</h2></section>
        <section data-slide="rootfs"><h2>Rootfs + /proc</h2></section>
        <section data-slide="pid-one"><h2>PID 1 + exec</h2></section>
        <section data-slide="demo"><h2>Live demo</h2></section>
        <section data-slide="docker-adds"><h2>What Docker adds</h2></section>
      </div>
    </div>
    <script type="module" src="/main.js"></script>
  </body>
</html>
```

Create `presentation/main.js`:

```js
import Reveal from 'reveal.js';
import RevealHighlight from 'reveal.js/plugin/highlight/highlight.esm.js';
import RevealNotes from 'reveal.js/plugin/notes/notes.esm.js';
import 'reveal.js/dist/reveal.css';
import 'reveal.js/plugin/highlight/monokai.css';
import '@fontsource/ibm-plex-sans-condensed/400.css';
import '@fontsource/ibm-plex-sans-condensed/600.css';
import '@fontsource/ibm-plex-sans-condensed/700.css';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/600.css';
import './theme.css';

const deck = new Reveal(document.querySelector('.reveal'), {
  width: 1600,
  height: 900,
  margin: 0.04,
  hash: true,
  controls: true,
  progress: true,
  center: false,
  transition: 'none',
  backgroundTransition: 'none',
  plugins: [RevealHighlight, RevealNotes],
});

window.presentationDeck = deck;
await deck.initialize();
```

Create `presentation/theme.css`:

```css
:root {
  --ink: #edf4ef;
  --muted: #9caaa1;
  --surface: #101713;
  --surface-raised: #18211c;
  --proof: #79dfa5;
  --data: #86b7d9;
  --runtime: #e7cf78;
}

body { background: var(--surface); }
.reveal { color: var(--ink); font-family: "IBM Plex Sans Condensed", sans-serif; }
.reveal .slides { text-align: left; }
.reveal .slides > section { box-sizing: border-box; height: 100%; padding: 56px 64px; }
.reveal h1, .reveal h2, .reveal h3 { color: var(--ink); font-family: inherit; letter-spacing: 0; text-transform: none; }
.reveal code, .reveal pre, .terminal { font-family: "IBM Plex Mono", monospace; }
```

- [ ] **Step 5: Run foundation checks**

Run: `npm run slides:test && npm run slides:build`

Expected: 2 tests pass; Vite builds ten modules/assets into `dist/presentation` without external CDN references.

- [ ] **Step 6: Commit the foundation**

```bash
git add .gitignore package.json package-lock.json presentation/index.html presentation/main.js presentation/theme.css presentation/playwright.config.js presentation/tests/deck.spec.js
git commit -m "feat: scaffold reveal presentation"
```

---

### Task 2: Lifecycle Slides and First Two Code Moments

**Files:**
- Modify: `presentation/index.html`
- Modify: `presentation/tests/deck.spec.js`

**Interfaces:**
- Consumes: the ten-slide Reveal shell and semantic color classes from Task 1.
- Produces: complete slides 1-4; source labels for `(*Puller).Pull` and `(*Builder).Build`; fragments whose order follows observe, explain, inspect.

- [ ] **Step 1: Add failing lifecycle content assertions**

Append to `presentation/tests/deck.spec.js`:

```js
test('teaches pull and build with real source references', async ({ page }) => {
  await page.goto('/');

  await expect(page.locator('[data-slide="pull"]')).toContainText('remote OCI image');
  await expect(page.locator('[data-slide="pull"]')).toContainText('internal/registry/puller.go');
  await expect(page.locator('[data-slide="pull"] code')).toContainText('puller.store.Put');

  await expect(page.locator('[data-slide="build"]')).toContainText('FROM alpine:3.22');
  await expect(page.locator('[data-slide="build"]')).toContainText('internal/builder/builder.go');
  await expect(page.locator('[data-slide="build"] code')).toContainText('case dockerfile.Run');
});
```

- [ ] **Step 2: Run the lifecycle test to verify it fails**

Run: `npm run slides:test -- --grep "pull and build"`

Expected: FAIL because slides 3 and 4 contain headings only.

- [ ] **Step 3: Replace slides 1-4 with final semantic markup**

In `presentation/index.html`, replace the first four placeholder sections with:

```html
<section data-slide="question" class="title-slide">
  <p class="eyebrow proof">BUILD YOUR OWN DOCKER</p>
  <h1>How does Docker work?</h1>
  <div class="terminal proof-terminal">
    <span class="prompt">$</span> mini-docker run --hostname demo-box hello:latest<br>
    <span class="muted">hostname:</span> <strong>demo-box</strong>
    <span class="muted">pid:</span> <strong>1</strong>
  </div>
  <p class="thesis">Follow one process from remote image to isolated PID 1.</p>
  <aside class="notes">0:00-2:00. Open exactly: “How does Docker work?” Establish that the answer will be executable, not metaphorical. Transition: “First, separate what we prepare from what we run.”</aside>
</section>

<section data-slide="workflow">
  <p class="eyebrow">THE LIFECYCLE</p>
  <h2>Prepare a filesystem. Start a process.</h2>
  <div class="pipeline" aria-label="Pull, build, run pipeline">
    <div class="stage data"><b>pull</b><span>remote image → local rootfs</span></div>
    <span class="arrow">→</span>
    <div class="stage data"><b>build</b><span>rootfs → derived rootfs + CMD</span></div>
    <span class="arrow">→</span>
    <div class="stage runtime"><b>run</b><span>rootfs + CMD → PID 1</span></div>
  </div>
  <p class="takeaway">An image is input. A container is a running process.</p>
  <aside class="notes">2:00-4:00. Define image versus container. Do not discuss namespaces yet. Transition: “Let us manufacture the input first.”</aside>
</section>

<section data-slide="pull">
  <p class="eyebrow data">CODE MOMENT 01 · PULL</p>
  <h2>A remote OCI image becomes a local rootfs.</h2>
  <div class="two-column">
    <div class="terminal"><span class="prompt">$</span> mini-docker pull alpine:3.22<br><span class="muted">Pulling...</span><br><span class="proof">Pulled alpine:3.22</span></div>
    <div class="blueprint fragment"><span>OCI layers</span><b>flatten</b><span>rootfs + CMD</span></div>
  </div>
  <pre class="fragment"><code class="language-go" data-trim>if err := extractRootFS(rootfs, archive); err != nil {
    return fmt.Errorf("extract image: %w", err)
}
metadata := image.Metadata{
    Name: parsedReference.Name(),
    Command: slices.Clone(configFile.Config.Cmd),
}
return puller.store.Put(ctx, parsedReference.Name(), rootfs, metadata)</code></pre>
  <p class="source">internal/registry/puller.go · (*Puller).Pull</p>
  <aside class="notes">4:00-7:00. Observe the command, explain flattening, then reveal publication. Avoid registry protocol details. Transition: “Build starts from that local result.”</aside>
</section>

<section data-slide="build">
  <p class="eyebrow data">CODE MOMENT 02 · BUILD</p>
  <h2>Build derives an image; it does not start the final container.</h2>
  <div class="two-column">
    <pre><code>FROM alpine:3.22
COPY hello.sh /hello.sh
RUN chmod +x /hello.sh
CMD ["/hello.sh"]</code></pre>
    <pre class="fragment"><code class="language-go" data-trim>case dockerfile.Copy:
    err = copyFileIntoRootFS(contextDir, buildRootFS,
        instruction.Source, instruction.Destination)
case dockerfile.Run:
    err = builder.executor.Execute(ctx, buildRootFS,
        []string{"/bin/sh", "-c", instruction.Command})
case dockerfile.Cmd:
    metadata.Command = append([]string(nil), instruction.Command...)</code></pre>
  </div>
  <p class="source">internal/builder/builder.go · (*Builder).Build</p>
  <p class="takeaway fragment">RUN mutates now. CMD records what should run later.</p>
  <aside class="notes">7:00-10:00. Read the four-line Dockerfile top to bottom. Emphasize that RUN uses the runtime path during build while CMD is metadata. Transition: “Now we have the two inputs: a rootfs and a command.”</aside>
</section>
```

- [ ] **Step 4: Run lifecycle and smoke tests**

Run: `npm run slides:test -- --grep "offline deck|keyboard navigation|pull and build"`

Expected: all 3 selected tests pass.

- [ ] **Step 5: Commit lifecycle slides**

```bash
git add presentation/index.html presentation/tests/deck.spec.js
git commit -m "feat: add pull and build slides"
```

---

### Task 3: Runtime Slides and Final Two Code Moments

**Files:**
- Modify: `presentation/index.html`
- Modify: `presentation/tests/deck.spec.js`

**Interfaces:**
- Consumes: Reveal fragments and semantic classes from Tasks 1-2.
- Produces: complete slides 5-8; namespace, rootfs, `/proc`, PID 1, and `exec` explanations tied to `runRootFS`, `configureChildCommand`, and `runInternalChild`.

- [ ] **Step 1: Add failing runtime assertions**

Append to `presentation/tests/deck.spec.js`:

```js
test('connects runtime primitives to observable proof', async ({ page }) => {
  await page.goto('/');

  await expect(page.locator('[data-slide="namespaces"] code')).toContainText('CLONE_NEWPID');
  await expect(page.locator('[data-slide="namespaces"]')).toContainText('internal/runtime/runner_linux.go');
  await expect(page.locator('[data-slide="rootfs"] code')).toContainText('syscall.Chroot');
  await expect(page.locator('[data-slide="rootfs"] code')).toContainText('syscall.Mount("proc"');
  await expect(page.locator('[data-slide="pid-one"]')).toContainText('exec replaces bootstrap');
  await expect(page.locator('[data-slide="pid-one"]')).toContainText('pid: 1');
});
```

- [ ] **Step 2: Run the runtime test to verify it fails**

Run: `npm run slides:test -- --grep "runtime primitives"`

Expected: FAIL because slides 5-8 are placeholders.

- [ ] **Step 3: Replace slides 5-8 with final semantic markup**

In `presentation/index.html`, replace the runtime placeholder sections with:

```html
<section data-slide="run-contract">
  <p class="eyebrow runtime">THE RUNTIME CONTRACT</p>
  <h2>Make one process observe a different machine.</h2>
  <div class="contract-grid">
    <div><span class="muted">hostname</span><strong>demo-box</strong></div>
    <div><span class="muted">process id</span><strong>1</strong></div>
    <div><span class="muted">filesystem root</span><strong>image rootfs</strong></div>
    <div><span class="muted">process view</span><strong>private /proc</strong></div>
  </div>
  <p class="takeaway">Isolation is a collection of Linux views, not one container syscall.</p>
  <aside class="notes">10:00-13:00. Define the desired observations before implementation. Ask which Linux mechanism could change each view. Transition: “Those views begin at process creation.”</aside>
</section>

<section data-slide="namespaces">
  <p class="eyebrow runtime">CODE MOMENT 03 · PARENT</p>
  <h2>Re-exec creates the namespace boundary.</h2>
  <div class="process-flow">
    <div>host CLI</div><span>re-exec</span><div class="runtime-box">child mini-docker</div><span>exec</span><div class="proof-box">payload</div>
  </div>
  <pre class="fragment"><code class="language-go" data-trim>command := exec.CommandContext(ctx,
    "/proc/self/exe", internalChildArgument)
command.Env = append(os.Environ(),
    childConfigEnvName+"="+encodedConfig)
command.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWPID |
        syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,
}</code></pre>
  <div class="namespace-key fragment"><span>PID · process IDs</span><span>UTS · hostname</span><span>MOUNT · mounts</span></div>
  <p class="source">internal/runtime/runner.go · runRootFS<br>internal/runtime/runner_linux.go · configureChildCommand</p>
  <aside class="notes">13:00-17:00. Explain why the program starts itself again. Reveal PID, UTS, and mount meanings only after the clone flags. State that chroot does not create namespace membership. Transition: “The child exists; now it must construct its view.”</aside>
</section>

<section data-slide="rootfs">
  <p class="eyebrow runtime">CODE MOMENT 04 · CHILD</p>
  <h2>Construct the view from the outside in.</h2>
  <div class="two-column code-pair">
    <pre><code class="language-go" data-trim>syscall.Mount("", "/", "",
    syscall.MS_REC|syscall.MS_PRIVATE, "")
syscall.Sethostname([]byte(config.Hostname))
syscall.Chroot(config.RootFS)
os.Chdir("/")</code></pre>
    <pre class="fragment"><code class="language-go" data-trim>os.MkdirAll("/proc", 0o555)
syscall.Mount("proc", "/proc", "proc",
    syscall.MS_NOSUID |
        syscall.MS_NOEXEC |
        syscall.MS_NODEV, "")
syscall.Exec(config.Command[0],
    config.Command, os.Environ())</code></pre>
  </div>
  <p class="source">internal/runtime/runner_linux.go · runInternalChild</p>
  <aside class="notes">17:00-21:00. Follow execution order. Define chroot as path-resolution change, not a security boundary. Mount /proc after namespace entry so it reports the child view. Transition: “The final call removes mini-docker from the process tree.”</aside>
</section>

<section data-slide="pid-one">
  <p class="eyebrow proof">THE OBSERVABLE RESULT</p>
  <h2><code>exec</code> replaces bootstrap. The payload becomes PID 1.</h2>
  <div class="before-after">
    <div><span class="muted">before exec</span><b>PID 1 · mini-docker child</b></div>
    <span class="arrow">→</span>
    <div class="proof-box"><span>after exec</span><b>PID 1 · /hello.sh</b></div>
  </div>
  <div class="terminal fragment">hostname: <strong>demo-box</strong><br>pid: <strong>1</strong><br>root entries: bin dev etc hello.sh lib proc tmp usr ...</div>
  <p class="takeaway">Same process identity. New program image.</p>
  <aside class="notes">21:00-24:00. Explain replacement, not spawning. Connect hostname to UTS, pid 1 to PID namespace, and /proc/root entries to mount namespace plus chroot. Transition: “Now let the real program prove every claim.”</aside>
</section>
```

- [ ] **Step 4: Run runtime and full content tests**

Run: `npm run slides:test -- --grep "runtime primitives|pull and build|keyboard navigation"`

Expected: all 3 selected tests pass.

- [ ] **Step 5: Commit runtime slides**

```bash
git add presentation/index.html presentation/tests/deck.spec.js
git commit -m "feat: explain mini-docker runtime"
```

---

### Task 4: Demo, Closing Slide, Notes, and Fallback

**Files:**
- Modify: `presentation/index.html`
- Modify: `presentation/tests/deck.spec.js`
- Create: `presentation/notes.md`
- Create: `presentation/demo.md`
- Create: `presentation/assets/demo-output.txt`
- Modify: `README.md`

**Interfaces:**
- Consumes: exact CLI behavior documented in `README.md` and the ten-slide deck.
- Produces: complete slides 9-10; one notes entry per slide; exact demo commands shared by README and runbook; stable captured-output fallback.

- [ ] **Step 1: Add failing package-completeness tests**

Add this import at the top of `presentation/tests/deck.spec.js`:

```js
import { readFile } from 'node:fs/promises';
```

Then append:

```js

test('packages notes and a consistent demo fallback', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('.reveal .slides > section aside.notes')).toHaveCount(10);

  const notes = await readFile('presentation/notes.md', 'utf8');
  const demo = await readFile('presentation/demo.md', 'utf8');
  const fallback = await readFile('presentation/assets/demo-output.txt', 'utf8');

  expect(notes.match(/^## Slide /gm)).toHaveLength(10);
  expect(demo).toContain('sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME"');
  expect(fallback).toContain('hostname: demo-box');
  expect(fallback).toContain('pid: 1');
});
```

- [ ] **Step 2: Run the package test to verify it fails**

Run: `npm run slides:test -- --grep "consistent demo fallback"`

Expected: FAIL because slide notes 9-10 and the three companion files do not exist.

- [ ] **Step 3: Complete slides 9-10**

Replace the final two placeholder sections in `presentation/index.html`:

```html
<section data-slide="demo">
  <p class="eyebrow proof">LIVE · 24:00-28:00</p>
  <h2>From Alpine to isolated PID 1.</h2>
  <div class="terminal demo-commands"><span class="prompt">$</span> export MINI_DOCKER_HOME="$(mktemp -d)"<br><span class="prompt">$</span> ./bin/mini-docker pull alpine:3.22<br><span class="prompt">$</span> sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo<br><span class="prompt">$</span> sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest</div>
  <p class="takeaway">Switch to the terminal now. Return only for fallback or closing.</p>
  <aside class="notes">24:00-28:00. Use the runbook. Pause before Enter on run and ask what the process should observe. After ten seconds without useful progress, show the captured output and continue; do not debug.</aside>
</section>

<section data-slide="docker-adds">
  <p class="eyebrow proof">THE MINIMAL ANSWER</p>
  <h2>Images prepare files. Linux isolates the process.</h2>
  <div class="core-answer"><b>image + rootfs</b><b>namespaces + chroot</b><b>private /proc + exec</b></div>
  <div class="docker-adds fragment"><span>cgroups</span><span>networking</span><span>layer storage</span><span>volumes</span><span>security hardening</span><span>distribution</span><span>orchestration</span></div>
  <p class="warning">This project demonstrates the mechanism. It is not a secure, production-ready container engine.</p>
  <aside class="notes">28:00-30:00. Close exactly: “An image and rootfs, Linux namespaces, a changed root, a private /proc, and exec are enough to demonstrate the core runtime idea. Docker adds the production system around it.” Invite questions only after the close.</aside>
</section>
```

- [ ] **Step 4: Create the notes, runbook, and fallback assets**

Create `presentation/notes.md`:

```markdown
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
```

Create `presentation/demo.md`:

```markdown
# Live Demo Runbook

## Before the Talk

1. Open WSL at the repository root and verify network access.
2. Build the binary and warm sudo credentials.
3. Keep `presentation/assets/demo-output.txt` ready in a second terminal tab.
4. Use a terminal font size readable from the back of the room.

```bash
go build -o bin/mini-docker ./cmd/mini-docker
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo -v
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
```

## Four-Minute Cues

- 0:00-0:20: export the fresh store.
- 0:20-1:10: pull; say “remote OCI content becomes a local rootfs.”
- 1:10-2:20: build; say “derive a filesystem and persist CMD.”
- 2:20-3:05: type run, pause before Enter, and ask what the process should observe.
- 3:05-4:00: connect hostname, PID 1, and root entries to the explained primitives.

## Fallback

After ten seconds without useful progress, display `presentation/assets/demo-output.txt` and explain the same evidence. Do not debug network, sudo, namespaces, or line endings on stage.
```

Create `presentation/assets/demo-output.txt`:

```text
$ ./bin/mini-docker pull alpine:3.22
Pulling alpine:3.22...
Pulled alpine:3.22
$ sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
Building hello:latest...
Built hello:latest
$ sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
hello from mini-docker
hostname: demo-box
pid: 1
root entries: bin dev etc hello.sh lib proc tmp usr ...
```

In `README.md`, replace the `Exact Demo Commands` block with:

```bash
go build -o bin/mini-docker ./cmd/mini-docker
export MINI_DOCKER_HOME="$(mktemp -d)"
./bin/mini-docker pull alpine:3.22
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo
sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest
```

Keep the existing export and pull lines in the `Fresh End-To-End Demo` block, but replace its build and run lines with the same two `sudo env` commands above. Replace the fresh-demo explanation with:

```markdown
`pull` does not require root. Dockerfile `RUN` during `build` and every `run` require root. Passing `MINI_DOCKER_HOME` through `sudo env` ensures that all three commands use the same image store without preserving unrelated environment variables.
```

- [ ] **Step 5: Run package and Go documentation checks**

Run: `npm run slides:test -- --grep "consistent demo fallback" && go test ./...`

Expected: package test passes; all Go tests pass.

- [ ] **Step 6: Commit demo package**

```bash
git add README.md presentation/index.html presentation/tests/deck.spec.js presentation/notes.md presentation/demo.md presentation/assets/demo-output.txt
git commit -m "docs: add presentation demo package"
```

---

### Task 5: Projection Theme, Responsive Checks, and PDF Export

**Files:**
- Modify: `presentation/theme.css`
- Modify: `presentation/tests/deck.spec.js`
- Create: `presentation/scripts/export-pdf.mjs`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: all ten semantic slides and `window.presentationDeck`.
- Produces: projection-safe 1600x900 design; viewport checks at 1920x1080 and 1366x768; `npm run slides:pdf` generating `dist/presentation/build-your-own-docker.pdf`.

- [ ] **Step 1: Add failing viewport and fragment tests**

Append to `presentation/tests/deck.spec.js`:

```js
for (const viewport of [
  { name: 'full-hd', width: 1920, height: 1080 },
  { name: 'projector', width: 1366, height: 768 },
]) {
  test(`fits every slide at ${viewport.name}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await page.goto('/');

    const slideCount = await page.locator('.reveal .slides > section').count();
    for (let index = 0; index < slideCount; index += 1) {
      await page.evaluate((slideIndex) => window.presentationDeck.slide(slideIndex), index);
      const overflow = await page.locator('.reveal .slides > section.present').evaluate((slide) => ({
        horizontal: slide.scrollWidth > slide.clientWidth + 2,
        vertical: slide.scrollHeight > slide.clientHeight + 2,
      }));
      expect(overflow, `slide ${index + 1}`).toEqual({ horizontal: false, vertical: false });
    }
  });
}

test('reveals all fragments without changing slide', async ({ page }) => {
  await page.goto('/');
  await page.evaluate(() => window.presentationDeck.slide(2));
  const slideBefore = await page.evaluate(() => window.presentationDeck.getIndices().h);
  await page.keyboard.press('ArrowRight');
  await expect(page.locator('[data-slide="pull"] .fragment.visible')).toHaveCount(1);
  expect(await page.evaluate(() => window.presentationDeck.getIndices().h)).toBe(slideBefore);
});
```

- [ ] **Step 2: Run visual behavior tests to verify they fail**

Run: `npm run slides:test -- --grep "fits every slide|reveals all fragments"`

Expected: at least the 1366x768 overflow test fails with the foundation-only CSS.

- [ ] **Step 3: Complete the custom theme**

Replace `presentation/theme.css` with:

```css
:root {
  --ink: #edf4ef;
  --muted: #9caaa1;
  --surface: #101713;
  --surface-raised: #18211c;
  --line: #34443b;
  --proof: #79dfa5;
  --data: #86b7d9;
  --runtime: #e7cf78;
}

* { box-sizing: border-box; }
body { background: var(--surface); }

.reveal {
  color: var(--ink);
  font-family: "IBM Plex Sans Condensed", sans-serif;
  font-size: 34px;
}

.reveal .slides { text-align: left; }
.reveal .slides > section { height: 100%; padding: 56px 64px; }
.reveal h1, .reveal h2, .reveal h3 {
  color: var(--ink);
  font-family: inherit;
  letter-spacing: 0;
  text-transform: none;
}
.reveal h1 { font-size: 2.65em; line-height: 0.98; max-width: 1150px; }
.reveal h2 { font-size: 1.72em; line-height: 1.05; max-width: 1280px; }
.reveal p { line-height: 1.35; }
.reveal code, .reveal pre, .terminal { font-family: "IBM Plex Mono", monospace; }
.reveal pre { width: 100%; margin: 22px 0; box-shadow: none; font-size: 0.48em; }
.reveal pre code { max-height: none; padding: 22px 24px; line-height: 1.45; }

.title-slide { display: flex !important; flex-direction: column; justify-content: center; }
.eyebrow { margin: 0 0 20px; color: var(--muted); font: 600 0.48em/1 "IBM Plex Mono", monospace; text-transform: uppercase; }
.thesis { max-width: 900px; color: var(--muted); }
.proof-terminal { max-width: 1120px; margin: 42px 0 22px; }
.terminal {
  background: #0b100d;
  border-left: 5px solid var(--proof);
  padding: 24px 28px;
  font-size: 0.64em;
  line-height: 1.65;
}
.terminal strong { color: var(--proof); }
.prompt, .proof { color: var(--proof); }
.data { color: var(--data); }
.runtime { color: var(--runtime); }
.muted { color: var(--muted); }

.pipeline, .process-flow, .before-after {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 24px;
  margin: 70px 0 54px;
}
.arrow { color: var(--muted); font-family: "IBM Plex Mono", monospace; }
.stage {
  display: flex;
  min-height: 170px;
  flex: 1;
  flex-direction: column;
  justify-content: space-between;
  border: 1px solid var(--line);
  padding: 24px;
}
.stage b { font: 600 1.25em/1 "IBM Plex Mono", monospace; }
.stage span { color: var(--ink); font-size: 0.62em; }

.two-column { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 34px; align-items: center; }
.blueprint { display: flex; align-items: center; justify-content: space-between; gap: 16px; color: var(--data); font-family: "IBM Plex Mono", monospace; font-size: 0.58em; }
.blueprint b { border: 1px solid var(--data); padding: 18px; color: var(--ink); }
.source { margin-top: 12px; color: var(--muted); font: 400 0.42em/1.45 "IBM Plex Mono", monospace; }
.takeaway { border-top: 1px solid var(--line); margin-top: 28px; padding-top: 18px; font-weight: 600; }

.contract-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; margin: 52px 0; }
.contract-grid > div { display: flex; min-height: 145px; flex-direction: column; justify-content: space-between; border: 1px solid var(--line); padding: 24px; }
.contract-grid span { font: 400 0.55em/1 "IBM Plex Mono", monospace; }
.contract-grid strong { color: var(--runtime); font-size: 1.18em; }

.process-flow > div, .runtime-box, .proof-box { border: 1px solid var(--line); padding: 24px; }
.process-flow > span { color: var(--muted); font: 400 0.48em/1 "IBM Plex Mono", monospace; }
.runtime-box { color: var(--runtime); }
.proof-box { color: var(--proof); }
.namespace-key { display: flex; gap: 18px; margin: 18px 0; }
.namespace-key span { flex: 1; border-top: 3px solid var(--runtime); padding-top: 12px; font: 400 0.48em/1.3 "IBM Plex Mono", monospace; }

.code-pair { align-items: start; }
.code-pair pre { margin-top: 0; }
.before-after > div { display: flex; min-width: 390px; flex-direction: column; gap: 18px; }
.before-after span { color: var(--muted); font: 400 0.48em/1 "IBM Plex Mono", monospace; }
.before-after b { font-family: "IBM Plex Mono", monospace; }

.demo-commands { margin-top: 46px; font-size: 0.43em; overflow-wrap: anywhere; }
.core-answer { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 18px; margin: 52px 0 34px; }
.core-answer b { border-top: 4px solid var(--proof); padding: 20px 12px; font: 600 0.68em/1.25 "IBM Plex Mono", monospace; }
.docker-adds { display: flex; flex-wrap: wrap; gap: 12px; }
.docker-adds span { border: 1px solid var(--line); padding: 10px 14px; color: var(--muted); font-size: 0.48em; }
.warning { margin-top: 38px; border-left: 5px solid var(--runtime); padding-left: 20px; color: var(--runtime); font-weight: 600; }

@media (max-aspect-ratio: 16 / 9) {
  .reveal .slides > section { padding: 44px 52px; }
  .reveal pre { font-size: 0.45em; }
  .demo-commands { font-size: 0.4em; }
}

@media print {
  .reveal .slides > section { break-after: page; }
  .reveal .fragment { opacity: 1 !important; visibility: visible !important; }
}
```

- [ ] **Step 4: Add deterministic PDF export**

Create `presentation/scripts/export-pdf.mjs`:

```js
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { chromium } from '@playwright/test';
import { createServer } from 'vite';

const outputDir = resolve('dist/presentation');
await mkdir(outputDir, { recursive: true });

const server = await createServer({
  root: resolve('presentation'),
  server: { host: '127.0.0.1', port: 4174 },
});

await server.listen();
const browser = await chromium.launch();

try {
  const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });
  await page.goto('http://127.0.0.1:4174/?print-pdf', { waitUntil: 'networkidle' });
  await page.emulateMedia({ media: 'print' });
  await page.pdf({
    path: resolve(outputDir, 'build-your-own-docker.pdf'),
    width: '13.333in',
    height: '7.5in',
    printBackground: true,
    preferCSSPageSize: true,
  });
} finally {
  await browser.close();
  await server.close();
}
```

- [ ] **Step 5: Run complete automated verification**

Run: `npm run slides:test && npm run slides:build && npm run slides:pdf && go test ./... && git diff --check`

Expected: all Playwright and Go tests pass; Vite builds successfully; `dist/presentation/build-your-own-docker.pdf` exists and is non-empty; `git diff --check` prints nothing.

- [ ] **Step 6: Perform visual and rehearsal verification**

Run: `npm run slides:dev -- --port 4173`

Verify in Chromium at 1920x1080 and 1366x768:

- every slide and fragment is nonblank and fully framed;
- code remains readable and no text overlaps;
- right-arrow navigation follows the ten-slide narrative;
- speaker view opens and contains the matching English notes;
- network-disabled reload still succeeds after the initial local build;
- the generated PDF contains ten pages with all fragments visible;
- a spoken rehearsal lasts 28-30 minutes, with slides 5-8 totaling 14 minutes.

Stop the dev server after verification.

- [ ] **Step 7: Commit final presentation polish**

```bash
git add .gitignore presentation/theme.css presentation/tests/deck.spec.js presentation/scripts/export-pdf.mjs
git commit -m "test: verify presentation delivery"
```

---

## Final Acceptance Run

From the repository root:

```bash
npm ci
npx playwright install chromium
npm run slides:test
npm run slides:build
npm run slides:pdf
go test ./...
git status --short
```

Expected:

- all Playwright tests pass at both viewports;
- Vite emits an offline deck under `dist/presentation`;
- PDF export creates `dist/presentation/build-your-own-docker.pdf`;
- all Go tests pass;
- only intentionally generated ignored output remains outside Git status.