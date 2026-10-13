import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createSessionApiClient, observeAccountRestriction } from './index';

test('account-restricted responses notify only the still-current credential and never reject it', async context => {
 let release!: () => void; let arrived!: () => void;
 const firstArrival = new Promise<void>(resolve => { arrived = resolve; });
 let held = true;
 const server = createServer(async (_, response) => {
  if (held) { held = false; arrived(); await new Promise<void>(resolve => { release = resolve; }); }
  response.writeHead(403, { 'Content-Type': 'application/problem+json' });
  response.end(JSON.stringify({ code: 'account_restricted' }));
 });
 server.listen(0, '127.0.0.1'); await once(server, 'listening');
 context.after(() => { server.closeAllConnections(); server.close(); });
 const address = server.address(); assert.ok(address && typeof address !== 'string');
 const baseURL = `http://127.0.0.1:${address.port}`;
 let token = 'old-session', own = 0, other = 0, foreignAPI = 0, rejected = 0;
 const stops = [observeAccountRestriction(baseURL, () => token, () => { own++; }), observeAccountRestriction(baseURL, () => 'different-account', () => { other++; }), observeAccountRestriction(baseURL + '/other', () => token, () => { foreignAPI++; })];
 context.after(() => stops.forEach(stop => stop()));
 const client = createSessionApiClient(baseURL, () => token, undefined, () => { rejected++; });
 const pending = client.configuredWeekStart(); await firstArrival;
 token = 'new-session'; release(); await pending;
 assert.equal(own, 0);
 await client.configuredWeekStart();
 assert.deepEqual([own, other, foreignAPI, rejected], [1, 0, 0, 0]);
 stops[0](); await client.configuredWeekStart();
 assert.equal(own, 1);
});
