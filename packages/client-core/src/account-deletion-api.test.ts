import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import test from 'node:test';
import { apiAccountDeletion } from './adapters/api-account-deletion';

test('deletion uses the session while receipt recovery sends only its narrow body capability', async () => {
  const secret = 'a'.repeat(64);
  let deleted = false;
  const received: { path: string; authorization: string | undefined }[] = [];
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString());
    received.push({ path: request.url!, authorization: request.headers.authorization });
    if (request.url === '/v1/me/deletion' && request.headers.authorization === 'Bearer session'
      && body.reviewedUserId === 'owner' && body.confirmed && body.receiptSecret === secret) {
      deleted = true; response.writeHead(204).end(); return;
    }
    response.writeHead(deleted && body.userId === 'owner' && body.receiptSecret === secret ? 204 : 401).end();
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  try {
    const address = server.address();
    assert.ok(address && typeof address !== 'string');
    const remote = apiAccountDeletion(`http://127.0.0.1:${address.port}`, () => 'session');
    assert.equal(await remote.receipt('owner', secret), false);
    await remote.remove('owner', secret);
    assert.equal(await remote.receipt('owner', secret), true);
    assert.equal(await remote.receipt('other', secret), false);
    assert.deepEqual(received.map(value => value.authorization), [undefined, 'Bearer session', undefined, undefined]);
  } finally {
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
});
