import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

const sharedDemoSequence = [
  'export MINI_DOCKER_HOME="$(mktemp -d)"',
  './bin/mini-docker pull alpine:3.22',
  'sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker build -t hello:latest ./testdata/demo',
  'sudo env MINI_DOCKER_HOME="$MINI_DOCKER_HOME" ./bin/mini-docker run --hostname demo-box hello:latest',
];

function normalizeCommandLines(lines) {
  return lines.map((line) => line.trim()).filter(Boolean).map((line) => line.replace(/^\$\s*/, ''));
}

function extractMarkdownBashBlock(document, heading) {
  const escapedHeading = heading.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const pattern = `## ${escapedHeading}[\\s\\S]*?\`\`\`bash\\r?\\n([\\s\\S]*?)\\r?\\n\`\`\``;
  const match = document.match(new RegExp(pattern));
  expect(match).not.toBeNull();

  return normalizeCommandLines(match[1].split('\n'));
}

function extractPromptCommands(document) {
  return normalizeCommandLines(document.split('\n').filter((line) => line.startsWith('$ ')));
}

function extractSlideCommands(document) {
  const match = document.match(/<section data-slide="demo">[\s\S]*?<div class="terminal demo-commands">([\s\S]*?)<\/div>/);
  expect(match).not.toBeNull();

  return normalizeCommandLines(
    match[1]
      .replace(/<span class="prompt">\$<\/span>/g, '$ ')
      .replace(/<br\s*\/?>/g, '\n')
      .split('\n'),
  );
}

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

test('uses a dark reveal surface with readable heading contrast', async ({ page }) => {
  await page.goto('/');

  const styles = await page.evaluate(() => {
    const section = document.querySelector('.reveal .slides > section.present');
    const heading = document.querySelector('[data-slide="question"] h1');

    return {
      viewportBackground: getComputedStyle(document.querySelector('.reveal-viewport')).backgroundColor,
      revealBackground: getComputedStyle(document.querySelector('.reveal')).backgroundColor,
      slideBackground: getComputedStyle(section).backgroundColor,
      headingColor: getComputedStyle(heading).color,
    };
  });

  expect(styles).toEqual({
    viewportBackground: 'rgb(16, 23, 19)',
    revealBackground: 'rgb(16, 23, 19)',
    slideBackground: 'rgb(16, 23, 19)',
    headingColor: 'rgb(237, 244, 239)',
  });
});

test('teaches pull and build with real source references', async ({ page }) => {
  await page.goto('/');

  const pullSlide = page.locator('[data-slide="pull"]');
  const pullCode = pullSlide.locator('code.language-go');
  await expect(pullSlide).toContainText('remote OCI image');
  await expect(pullSlide).toContainText('internal/registry/puller.go');
  await expect.soft(pullCode).toContainText('rootfs := filepath.Join(temporaryRoot, "rootfs")');
  await expect.soft(pullCode).toContainText('if err := os.Mkdir(rootfs, 0o755); err != nil');
  await expect.soft(pullCode).toContainText('if err := extractRootFS(rootfs, archive); err != nil');
  await expect.soft(pullCode).toContainText('return fmt.Errorf("extract image: %w", err)');
  await expect.soft(pullCode).toContainText('metadata := image.Metadata{');
  await expect.soft(pullCode).not.toContainText('return puller.store.Put');

  const buildSlide = page.locator('[data-slide="build"]');
  const buildCode = buildSlide.locator('code.language-go');
  await expect(buildSlide).toContainText('FROM alpine:3.22');
  await expect(buildSlide).toContainText('internal/builder/builder.go');
  await expect.soft(buildCode).toContainText('if err := copyFileIntoRootFS');
  await expect.soft(buildCode).toContainText('if err := builder.executor.Execute');
  await expect.soft(buildCode).toContainText('return err');
  await expect.soft(buildCode).toContainText('metadata.Command = append([]string(nil), instruction.Command...)');
  await expect.soft(buildCode).not.toContainText('err = copyFileIntoRootFS');
  await expect.soft(buildCode).not.toContainText('err = builder.executor.Execute');
});

test('connects runtime primitives to observable proof', async ({ page }) => {
  await page.goto('/');

  const namespaceSlide = page.locator('[data-slide="namespaces"]');
  const namespaceCode = namespaceSlide.locator('code.language-go');
  const namespaceKey = namespaceSlide.locator('.namespace-key');
  await expect(namespaceSlide.locator('.process-flow')).toContainText('host CLI');
  await expect(namespaceKey).toContainText('PID · process IDs');
  await expect(namespaceKey).toContainText('UTS · hostname');
  await expect(namespaceKey).toContainText('MOUNT · mounts');
  await expect(namespaceCode).toHaveCount(2);
  await expect(namespaceCode.first()).toContainText('childExecutablePath()');
  await expect(namespaceCode.last()).toContainText('CLONE_NEWPID');
  await expect(namespaceCode.last()).toContainText('CLONE_NEWUTS');
  await expect(namespaceCode.last()).toContainText('CLONE_NEWNS');
  await expect(namespaceSlide).toContainText('/proc/self/exe resolves to the running mini-docker executable');
  await expect(namespaceSlide).toContainText('internal/runtime/runner.go · runRootFS');
  await expect(namespaceSlide).toContainText('internal/runtime/runner_linux.go · configureChildCommand');

  const namespaceOrder = await namespaceSlide.locator('.process-flow, .namespace-key, .code-inspection').evaluateAll(
    (elements) => elements.map((element) => element.className),
  );
  expect(namespaceOrder).toEqual(['process-flow', 'namespace-key fragment', 'code-inspection fragment']);

  const rootfsSlide = page.locator('[data-slide="rootfs"]');
  const rootfsCode = rootfsSlide.locator('code.language-go');
  await expect(rootfsCode).toHaveCount(2);
  await expect(rootfsCode.first()).toContainText('syscall.MS_REC|syscall.MS_PRIVATE');
  await expect(rootfsCode.first()).toContainText('failed to make mount propagation private');
  await expect(rootfsCode.first()).toContainText('if config.Hostname != ""');
  await expect(rootfsCode.first()).toContainText('syscall.Sethostname');
  await expect(rootfsCode.first()).toContainText('syscall.Chroot');
  await expect(rootfsCode.last()).toContainText('syscall.Mount("proc"');
  await expect(rootfsCode.last()).toContainText('syscall.Exec');
  await expect(rootfsSlide).toContainText('chroot changes path resolution; it is not a complete security boundary');

  const childOperations = await rootfsCode.allTextContents();
  const orderedChildSource = childOperations.join('\n');
  const operationOffsets = [
    'syscall.MS_REC|syscall.MS_PRIVATE',
    'if config.Hostname != ""',
    'syscall.Sethostname',
    'syscall.Chroot',
    'syscall.Mount("proc"',
    'syscall.Exec',
  ].map((operation) => orderedChildSource.indexOf(operation));
  expect(operationOffsets).not.toContain(-1);
  expect(operationOffsets).toEqual([...operationOffsets].sort((left, right) => left - right));

  const contractSlide = page.locator('[data-slide="run-contract"]');
  await expect(contractSlide).toContainText('demo-box');

  const pidOneSlide = page.locator('[data-slide="pid-one"]');
  await expect(pidOneSlide).toContainText('exec replaces bootstrap');
  await expect(pidOneSlide).toContainText('pid: 1');
  await expect(pidOneSlide).toContainText('PID 1 inside the new PID namespace');

  await expect(page.getByText(/CODE MOMENT \d{2}/)).toHaveCount(4);
});

test('packages notes and a consistent demo fallback', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('.reveal .slides > section aside.notes')).toHaveCount(10);

  const readme = await readFile('README.md', 'utf8');
  const slideMarkup = await readFile('presentation/index.html', 'utf8');
  const notes = await readFile('presentation/notes.md', 'utf8');
  const demo = await readFile('presentation/demo.md', 'utf8');
  const fallback = await readFile('presentation/assets/demo-output.txt', 'utf8');
  const slideCommands = extractSlideCommands(slideMarkup);

  expect(notes.match(/^## Slide /gm)).toHaveLength(10);
  expect(extractMarkdownBashBlock(readme, 'Exact Demo Commands')).toEqual(sharedDemoSequence);
  expect(extractMarkdownBashBlock(readme, 'Fresh End-To-End Demo')).toEqual(sharedDemoSequence);
  expect(slideCommands).toEqual(sharedDemoSequence);
  expect(extractMarkdownBashBlock(demo, 'Shared Demo Sequence')).toEqual(sharedDemoSequence);
  expect(extractPromptCommands(fallback)).toEqual(sharedDemoSequence);
  expect(fallback).toContain('hostname: demo-box');
  expect(fallback).toContain('pid: 1');
});

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
