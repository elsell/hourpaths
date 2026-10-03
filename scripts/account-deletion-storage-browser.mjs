import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { chromium } from 'playwright';
import ts from '../apps/web/node_modules/typescript/lib/typescript.js';

// Exercise the actual adapter against Chromium IndexedDB, including two open tabs.
const sources = new Map();
for (const [url, path] of [
  ['/store.js', '../apps/web/src/lib/studio/offline/adapters/indexeddb-tracking-store.ts'],
  ['/core.js', '../packages/client-core/src/account-deletion.ts'],
]) {
  const source = await readFile(new URL(path, import.meta.url), 'utf8');
  sources.set(url, ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText);
}
const server = createServer((req, res) => {
  res.setHeader('Content-Type', sources.has(req.url) ? 'text/javascript' : 'text/html');
  res.end(sources.get(req.url) ?? '<script type="importmap">{"imports":{"@hourpaths/client-core":"/core.js"}}</script>');
});
server.listen(0, '127.0.0.1');
await once(server, 'listening');
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  const a = await context.newPage(), b = await context.newPage();
  for (const page of [a, b]) {
    await page.goto(`http://127.0.0.1:${server.address().port}`);
    await page.evaluate(async () => {
      const { IndexedDBTrackingStore } = await import('/store.js');
      window.store = new IndexedDBTrackingStore('deletion-test');
    });
  }
  await a.evaluate(async () => {
    await store.saveHome('alice', [], {});
    await store.saveHome('bob', [], {});
    await store.commit('alice', 0, { owner: 'alice', revision: 1 });
  });
  assert.equal(await b.evaluate(async () => (await store.readHome('alice')).owner), 'alice');
  await a.evaluate(() => store.deletion.save({ owner: 'alice', receiptSecret: 'a'.repeat(64), phase: 'pending' }));
  assert.deepEqual(await b.evaluate(async () => [await store.read('alice'), await store.readHome('alice')]), [null, null]);
  assert.equal(await b.evaluate(async () => {
    try { await store.saveHome('alice', [], {}); return false; } catch { return true; }
  }), true);
  await a.reload();
  await a.evaluate(async () => {
    const { IndexedDBTrackingStore } = await import('/store.js');
    window.store = new IndexedDBTrackingStore('deletion-test');
  });
  assert.deepEqual(await a.evaluate(() => store.deletion.pendingOwners()), ['alice']);
  assert.equal(await a.evaluate(async () => {
    const saved = await store.deletion.save({ owner: 'alice', receiptSecret: 'b'.repeat(64), phase: 'pending' });
    return saved.receiptSecret;
  }), 'a'.repeat(64));
  await a.evaluate(async () => {
    await store.deletion.save({ owner: 'alice', receiptSecret: 'a'.repeat(64), phase: 'confirmed' });
    await store.deletion.purge('alice');
    await store.deletion.forget('alice');
  });
  assert.equal(await b.evaluate(async () => {
    try { await store.commit('alice', 0, { owner: 'alice', revision: 1 }); return false; } catch { return true; }
  }), true);
  assert.equal(await b.evaluate(async () => (await store.readHome('bob')).owner), 'bob');
  assert.deepEqual(await b.evaluate(() => store.deletion.pendingOwners()), []);
  assert.deepEqual(await b.evaluate(() => new Promise((resolve, reject) => {
    const request = indexedDB.open('deletion-test', 3);
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      const db = request.result;
      const tx = db.transaction(['accounts', 'home', 'deletionIntents']);
      const reads = ['accounts', 'home', 'deletionIntents'].map(name => tx.objectStore(name).get('alice'));
      tx.oncomplete = () => { resolve(reads.map(read => read.result ?? null)); db.close(); };
      tx.onabort = () => { reject(tx.error); db.close(); };
    };
  })), [null, null, null]);
  assert.equal(await b.evaluate(() => new Promise(resolve => {
    const request = indexedDB.open('deletion-test', 2);
    request.onerror = () => resolve(request.error.name);
    request.onsuccess = () => { request.result.close(); resolve('unexpected downgrade'); };
  })), 'VersionError');
  console.log('PASS: actual IndexedDB restart, cross-tab fence, owner isolation, late writes, and old-client downgrade rejection');
} finally {
  await browser.close();
  server.close();
}
