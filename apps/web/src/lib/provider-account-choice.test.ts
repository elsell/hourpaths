import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import test from 'node:test';
import { InMemoryWebStorage } from 'oidc-client-ts';
import { beginProviderSignIn } from './provider-auth';

test('interactive sign-in sends fresh-login, PKCE and state even with a remembered provider session', async () => {
  let issuer = '';
  const provider = createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.url === '/.well-known/openid-configuration') {
      response.end(JSON.stringify({ issuer, authorization_endpoint: `${issuer}/authorize`, token_endpoint: `${issuer}/token`, jwks_uri: `${issuer}/keys` }));
    } else {
      const url = new URL(request.url!, issuer);
      response.end(JSON.stringify({ screen: url.searchParams.get('prompt') === 'login' ? 'sign-in' : 'remembered-account' }));
    }
  });
  provider.listen(0, '127.0.0.1');
  await once(provider, 'listening');
  const address = provider.address();
  assert.ok(address && typeof address !== 'string');
  issuer = `http://127.0.0.1:${address.port}`;
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'window');
  try {
    const destination = new Promise<URL>((resolve, reject) => {
      const browser = { location: { origin: 'https://app.hourpaths.test', assign: (url: string) => resolve(new URL(url)) }, sessionStorage: new InMemoryWebStorage(), stop() {} };
      Object.defineProperty(globalThis, 'window', { configurable: true, value: { ...browser, self: browser } });
      void beginProviderSignIn(issuer, 'web-client').catch(reject);
    });
    const url = await destination;
    assert.equal(url.searchParams.get('prompt'), 'login');
    assert.equal(url.searchParams.get('code_challenge_method'), 'S256');
    assert.ok(url.searchParams.get('code_challenge'));
    assert.ok(url.searchParams.get('state'));
    const response = await fetch(url, { headers: { Cookie: 'logto-session=previous-account' } });
    assert.deepEqual(await response.json(), { screen: 'sign-in' });
  } finally {
    if (previous) Object.defineProperty(globalThis, 'window', previous);
    else Reflect.deleteProperty(globalThis, 'window');
    provider.close();
    await once(provider, 'close');
  }
});
