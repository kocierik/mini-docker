import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { chromium } from '@playwright/test';
import { createServer } from 'vite';

const repoRoot = resolve(fileURLToPath(new URL('../..', import.meta.url)));
const outputDir = resolve(repoRoot, 'dist/presentation');
const printStyles = `
  html, body {
    width: auto !important;
    height: auto !important;
    overflow: visible !important;
  }

  body {
    margin: 0 !important;
    padding: 0 !important;
  }

  .reveal {
    width: auto !important;
    height: auto !important;
    overflow: visible !important;
  }

  .reveal .controls,
  .reveal .progress,
  .reveal .playback,
  .reveal .backgrounds,
  .reveal .slide-number,
  .state-background,
  .aria-status {
    display: none !important;
  }

  .reveal .slides {
    position: static !important;
    width: auto !important;
    height: auto !important;
    left: 0 !important;
    top: 0 !important;
    margin: 0 !important;
    padding: 0 !important;
    display: block !important;
    overflow: visible !important;
    transform: none !important;
    perspective: none !important;
  }

  .reveal .slides > section {
    position: static !important;
    width: auto !important;
    min-height: 7.5in !important;
    height: auto !important;
    display: block !important;
    overflow: visible !important;
    opacity: 1 !important;
    visibility: visible !important;
    transform: none !important;
    transition: none !important;
    page-break-after: always !important;
    break-after: page !important;
  }

  .reveal .slides > section:last-of-type {
    page-break-after: avoid !important;
    break-after: auto !important;
  }

  .reveal .slides .fragment {
    opacity: 1 !important;
    visibility: visible !important;
    transform: none !important;
  }

  .reveal pre code {
    overflow: hidden !important;
  }
`;

await mkdir(outputDir, { recursive: true });

const server = await createServer({
  root: resolve(repoRoot, 'presentation'),
  server: { host: '127.0.0.1', port: 4174 },
});

await server.listen();
const browser = await chromium.launch();

try {
  const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });
  await page.goto('http://127.0.0.1:4174/?print-pdf', { waitUntil: 'networkidle' });
  await page.waitForFunction(() => window.presentationDeck?.isReady?.() === true);
  await page.addStyleTag({ content: printStyles });
  await page.evaluate(() => {
    window.presentationDeck.configure({ pdfSeparateFragments: false });
    document.documentElement.classList.add('print-pdf', 'reveal-print');
  });
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