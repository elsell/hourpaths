import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createSessionApiClient } from './index';

test('disposing an account scope cancels its in-flight generated HTTP request', async context => {
  let received!: () => void;
  const incoming = new Promise<void>(resolve => { received = resolve; });
  const server = createServer((request, response) => {
    assert.equal(request.url, '/v1/me');
    assert.equal(request.headers.authorization, 'Bearer old-account');
    received();
    request.once('close', () => response.destroy());
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  context.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const owner = new AbortController();
  const client = createSessionApiClient(`http://127.0.0.1:${address.port}`, () => 'old-account', owner.signal);
  const pending = client.profile();
  const rejected = assert.rejects(pending, error => error instanceof Error && error.name === 'AbortError');
  await incoming;
  owner.abort();
  await rejected;
});
