import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { chromium } from '@playwright/test';
import { createServer } from 'vite';

const repoRoot = resolve(fileURLToPath(new URL('../..', import.meta.url)));
const outputDir = resolve(repoRoot, 'dist/presentation');
const printStyles = `
  @page {
    size: 13.333in 7.5in;
    margin: 0;
  }

  html, body {
    margin: 0 !important;
    padding: 0 !important;
    background: #101713 !important;
  }

  *, *::before, *::after {
    box-sizing: border-box;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }

  body {
    width: 1600px !important;
    min-width: 1600px !important;
  }

  .pdf-export-root {
    display: block;
  }

  .pdf-export-page {
    width: 1600px !important;
    height: 900px !important;
    min-height: 900px !important;
    margin: 0 !important;
    padding: 0 !important;
    overflow: hidden !important;
    background: #101713 !important;
    display: block !important;
    position: relative !important;
    page-break-after: always !important;
    break-after: page !important;
  }

  .pdf-export-page:last-of-type {
    page-break-after: avoid !important;
    break-after: auto !important;
  }

  .pdf-export-page > section {
    width: 100% !important;
    height: 900px !important;
    min-height: 900px !important;
    margin: 0 !important;
    display: block !important;
    position: relative !important;
    overflow: hidden !important;
    opacity: 1 !important;
    visibility: visible !important;
    transform: none !important;
    transition: none !important;
  }

  .pdf-export-page .fragment {
    opacity: 1 !important;
    visibility: visible !important;
    transform: none !important;
  }

  .pdf-export-page pre code {
    overflow: hidden !important;
  }

  .pdf-export-page aside.notes {
    display: none !important;
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
  await page.goto('http://127.0.0.1:4174/', { waitUntil: 'networkidle' });
  await page.waitForFunction(() => window.presentationDeck?.isReady?.() === true);
  await page.addStyleTag({ content: printStyles });
  await page.evaluate(() => {
    const exportRoot = document.createElement('main');
    exportRoot.className = 'pdf-export-root';

    for (const slide of document.querySelectorAll('.reveal .slides > section')) {
      const pageNode = document.createElement('section');
      pageNode.className = 'pdf-export-page';

      const clone = slide.cloneNode(true);
      clone.classList.remove('past', 'present', 'future');
      clone.querySelectorAll('.fragment').forEach((fragment) => {
        fragment.classList.add('visible', 'current-fragment');
      });
      clone.querySelectorAll('aside.notes').forEach((notes) => notes.remove());

      pageNode.append(clone);
      exportRoot.append(pageNode);
    }

    document.body.replaceChildren(exportRoot);
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