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

  await expect(page.locator('[data-slide="pull"]')).toContainText('remote OCI image');
  await expect(page.locator('[data-slide="pull"]')).toContainText('internal/registry/puller.go');
  await expect(page.locator('[data-slide="pull"] code')).toContainText('puller.store.Put');

  await expect(page.locator('[data-slide="build"]')).toContainText('FROM alpine:3.22');
  await expect(page.locator('[data-slide="build"]')).toContainText('internal/builder/builder.go');
  await expect(page.locator('[data-slide="build"] code.language-go')).toContainText('case dockerfile.Run');
});