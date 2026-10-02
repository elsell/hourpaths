import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createShellCache } from './offline-shell/cache';

function fixture() {
  const buckets = new Map<string, Map<string, Response>>();
  const requests: { url: string; init?: RequestInit }[] = [];
  let network: (url: string) => Promise<Response> = async url => new Response(url, {
    headers: url.endsWith('/studio') ? { 'content-type': 'text/html', 'x-hourpaths-public-shell': '1' } : {}
  });
  const storage = {
    async open(name: string) {
      if (!buckets.has(name)) buckets.set(name, new Map());
      const rows = buckets.get(name)!;
      return { async match(url: string) { return rows.get(url)?.clone(); }, async put(url: string, response: Response) { rows.set(url, response.clone()); } };
    },
    async keys() { return [...buckets.keys()]; },
    async delete(name: string) { return buckets.delete(name); }
  };
  const create = (version: string) => createShellCache({ origin: 'https://app.example', base: '', version, assets: ['/_app/immutable/start.js'], storage,
    request: async (url, init) => { requests.push({ url, init }); return network(url); } });
  return { buckets, requests, create, network: (next: typeof network) => { network = next; } };
}
const navigation = (url = 'https://app.example/studio') => ({ url, method: 'GET', mode: 'navigate' });

test('cold Studio navigation and build assets survive disconnection with an anonymous shell', async () => {
  const f = fixture(), cache = f.create('one');
  await cache.install();
  assert.equal(f.requests.length, 2);
  for (const request of f.requests) {
    assert.equal(request.init?.credentials, 'omit');
    assert.equal(request.init?.redirect, 'error');
  }
  f.network(async () => { throw new TypeError('offline'); });
  assert.equal(await (await cache.respond(navigation('https://app.example/studio/paths/123/history'))).text(), 'https://app.example/studio');
  assert.equal(await (await cache.respond({ url: 'https://app.example/_app/immutable/start.js', method: 'GET', mode: 'cors' })).text(), 'https://app.example/_app/immutable/start.js');
});

test('only Studio navigations and exact same-origin build assets are intercepted', () => {
  const cache = fixture().create('one');
  for (const url of ['https://api.example/studio', 'https://app.example/api/paths', 'https://app.example/callback', 'https://app.example/studios', 'https://provider.example/authorize', 'https://app.example/_app/immutable/unknown.js']) assert.equal(cache.handles(navigation(url)), false, url);
  assert.equal(cache.handles({ ...navigation(), method: 'POST' }), false);
  assert.equal(cache.handles({ ...navigation(), mode: 'cors' }), false);
  assert.equal(cache.handles({ url: 'https://app.example/_app/immutable/start.js?token=secret', method: 'GET', mode: 'cors' }), false);
});

test('explicit rejection is returned and authenticated navigation never replaces the anonymous shell', async () => {
  const f = fixture(), cache = f.create('one'); await cache.install();
  f.network(async () => new Response('Denied', { status: 403 }));
  assert.equal((await cache.respond(navigation())).status, 403);
  f.network(async () => new Response('personalized', { headers: { 'content-type': 'text/html' } }));
  assert.equal(await (await cache.respond(navigation())).text(), 'personalized');
  f.network(async () => { throw new TypeError('offline'); });
  assert.equal(await (await cache.respond(navigation())).text(), 'https://app.example/studio');
});

test('failed updates preserve prior cache and reject unmarked or unsuccessful shells', async () => {
  for (const response of [new Response('login'), new Response('denied', { status: 401 }), new Response('json', { headers: { 'x-hourpaths-public-shell': '1', 'content-type': 'application/json' } })]) {
    const f = fixture(); await f.create('one').install();
    const previous = [...f.buckets.keys()]; f.network(async () => response.clone());
    await assert.rejects(f.create('two').install());
    assert.deepEqual([...f.buckets.keys()], previous);
  }
});

test('activation retires only obsolete shell caches and leaves other storage untouched', async () => {
  const f = fixture(); await f.create('one').install();
  f.buckets.set('unrelated', new Map()); const current = f.create('two'); await current.install(); await current.activate();
  assert.equal(f.buckets.size, 2); assert.ok(f.buckets.has('unrelated'));
});
