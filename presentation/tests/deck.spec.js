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

  await expect(page.locator('[data-slide="namespaces"] code')).toContainText('CLONE_NEWPID');
  await expect(page.locator('[data-slide="namespaces"]')).toContainText('internal/runtime/runner_linux.go');
  await expect(page.locator('[data-slide="rootfs"] code').first()).toContainText('syscall.Chroot');
  await expect(page.locator('[data-slide="rootfs"] code').last()).toContainText('syscall.Mount("proc"');
  await expect(page.locator('[data-slide="pid-one"]')).toContainText('exec replaces bootstrap');
  await expect(page.locator('[data-slide="pid-one"]')).toContainText('pid: 1');
});
