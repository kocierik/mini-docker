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

test('teaches pull and build with real source references', async ({ page }) => {
  await page.goto('/');

  const pullSlide = page.locator('[data-slide="pull"]');
  const pullCode = pullSlide.locator('code.language-go');
  await expect(pullSlide).toContainText('remote OCI image');
  await expect(pullSlide).toContainText('internal/registry/puller.go');
  await expect.soft(pullCode).toContainText('if err := puller.store.Put');
  await expect.soft(pullCode).toContainText('return fmt.Errorf("publish image: %w", err)');
  await expect.soft(pullCode).toContainText('return nil');
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
