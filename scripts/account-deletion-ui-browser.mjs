import { realpathSync } from 'node:fs';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
const require = createRequire(realpathSync(new URL('../apps/web/node_modules/tsx/package.json', import.meta.url)));
const { build } = require('esbuild');
const root = fileURLToPath(new URL('../apps/web', import.meta.url));
const output = await build({ bundle: true, write: false, outdir: '/virtual', format: 'esm', jsx: 'automatic', stdin: {
  resolveDir: root, loader: 'tsx', contents: `
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { createTranslator } from '@hourpaths/i18n';
import { AccountDeletionSettings } from './src/lib/studio/presentation/account-deletion';
import './src/lib/studio/presentation/studio.css';
window.calls = { review: 0, confirm: 0 };
const service = {
  async review() { window.calls.review++; return { owner: 'alice', name: 'Alice' }; },
  async confirm(owner) { if (owner !== 'alice') throw new Error('wrong_owner'); window.calls.confirm++; throw new Error('controlled_service_unavailable'); },
  async resume() {}, async pendingOwners() { return []; }
};
const route = createRootRoute({ component: () => <div className="studio studio-entry"><AccountDeletionSettings service={service} i18n={createTranslator(['en'])} /></div> });
const router = createRouter({ routeTree: route });
createRoot(document.getElementById('root')).render(<QueryClientProvider client={new QueryClient()}><RouterProvider router={router} /></QueryClientProvider>);
` } });
const assets = new Map(output.outputFiles.map(file => [file.path.endsWith('.css') ? '/app.css' : '/app.js', file.text]));
const server = createServer((req, res) => {
  res.setHeader('Content-Type', req.url === '/app.js' ? 'text/javascript' : req.url === '/app.css' ? 'text/css' : 'text/html');
  res.end(assets.get(req.url) ?? '<link rel="stylesheet" href="/app.css"><div id="root"></div><script type="module" src="/app.js"></script>');
});
server.listen(0, '127.0.0.1'); await once(server, 'listening');
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  const errors = []; page.on('pageerror', error => errors.push(error.message));
  await page.goto(`http://127.0.0.1:${server.address().port}`);
  await page.getByRole('button', { name: 'Delete account', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByText('Alice', { exact: true }).waitFor();
  for (const text of ['cannot be canceled or undone', 'every participant’s recorded activity', 'unsynchronized changes', 'Encrypted backups']) {
    await dialog.getByText(text, { exact: false }).waitFor();
  }
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  assert.equal(await dialog.count(), 0);
  assert.equal(await page.evaluate(() => window.calls.confirm), 0);
  await page.getByRole('button', { name: 'Delete account', exact: true }).click();
  await dialog.getByRole('button', { name: 'Permanently delete account', exact: true }).click();
  await dialog.getByRole('alert').waitFor();
  assert.equal(await page.evaluate(() => window.calls.confirm), 1);
  assert.equal(await dialog.getByRole('button', { name: 'Cancel', exact: true }).count(), 0);
  await page.keyboard.press('Escape');
  assert.equal(await dialog.count(), 1);
  await dialog.getByRole('button', { name: 'Try again', exact: true }).click();
  assert.equal(await page.evaluate(() => window.calls.confirm), 2);
  assert.deepEqual(errors, []);
  console.log('PASS: rendered deletion warning, cancellation without mutation, reviewed owner, persistent failure and retry');
} finally { await browser.close(); server.close(); }
