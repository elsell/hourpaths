import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer } from 'node:http';
import { createSessionApiClient } from './index';

test('opt-in retained reads retry the same endpoint but never mutations or replacement credentials', async t => {
  let token = 'first', reads = 0, writes = 0, reject = false;
  const seen: string[] = [];
  const server = createServer((req, res) => {
    seen.push(req.headers.authorization ?? '');
    res.setHeader('Content-Type', 'application/json');
    if (req.method === 'POST') { writes++; res.writeHead(429, { 'Retry-After': '0' }); res.end('{}'); return; }
    reads++;
    if (reject) { res.writeHead(401); res.end('{}'); return; }
    if (reads % 2 === 1) { res.writeHead(429, { 'Retry-After': '0' }); res.end('{}'); return; }
    res.end(JSON.stringify({ data: { id: 'owner' } }));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const url = `http://127.0.0.1:${address.port}`;
  let rejections = 0;
  const client = createSessionApiClient(url, () => token, undefined, () => rejections++, { retryRateLimitedReads: true });
  assert.equal((await client.profile()).response.status, 200);
  assert.equal(reads, 2);
  assert.equal((await client.createPath({ name: 'Path', visibility: 'private' }, 'operation-0000001')).response.status, 429);
  assert.equal(writes, 1);
  let calls = 0;
  const replacement = createSessionApiClient(url, () => ++calls === 1 ? 'first' : 'second', undefined, undefined, { retryRateLimitedReads: true });
  await assert.rejects(replacement.profile(), /cancelled/);
  assert.equal(reads, 3);
  assert.ok(seen.every(value => value === 'Bearer first'));
  reject = true;
  assert.equal((await client.profile()).response.status, 401);
  assert.equal(reads, 4); assert.equal(rejections, 1);
});

test('read retries are bounded and aborting a delayed read sends no retry', async t => {
  let count = 0, delay = '0';
  const server = createServer((_req, res) => { count++; res.writeHead(429, { 'Content-Type': 'application/json', 'Retry-After': delay }); res.end('{}'); });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const url = `http://127.0.0.1:${address.port}`;
  assert.equal((await createSessionApiClient(url, () => 'first', undefined, undefined, { retryRateLimitedReads: true }).profile()).response.status, 429);
  assert.equal(count, 3);
  delay = '61';
  assert.equal((await createSessionApiClient(url, () => 'first', undefined, undefined, { retryRateLimitedReads: true }).profile()).response.status, 429);
  assert.equal(count, 4);
  delay = '60';
  const controller = new AbortController();
  const operation = createSessionApiClient(url, () => 'first', controller.signal, undefined, { retryRateLimitedReads: true }).profile();
  const timeout = setTimeout(() => controller.abort(), 20);
  t.after(() => clearTimeout(timeout));
  await assert.rejects(operation, /cancelled|abort/i);
  assert.equal(count, 5);
});
