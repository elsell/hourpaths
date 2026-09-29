import assert from 'node:assert/strict';
import { test } from 'node:test';
import { OidcClient, WebStorageStateStore } from 'oidc-client-ts';

function client() {
  const records = new Map<string, string>();
  const storage = { get length() { return records.size; }, clear: () => records.clear(), getItem: (key: string) => records.get(key) ?? null, setItem: (key: string, value: string) => { records.set(key, value); }, removeItem: (key: string) => { records.delete(key); }, key: (index: number) => [...records.keys()][index] ?? null };
  return new OidcClient({ authority: 'https://broker.example/oidc', client_id: 'web', redirect_uri: 'https://app.example/callback', post_logout_redirect_uri: 'https://app.example/signed-out', response_type: 'code', scope: 'openid', stateStore: new WebStorageStateStore({ store: storage }), metadata: { issuer: 'https://broker.example/oidc', authorization_endpoint: 'https://broker.example/oidc/auth', token_endpoint: 'https://broker.example/oidc/token', jwks_uri: 'https://broker.example/oidc/jwks', end_session_endpoint: 'https://broker.example/oidc/session/end' } });
}
test('OIDC logout sends client identity without retaining tokens and validates a one-use callback', async () => {
  const broker = client();
  const request = await broker.createSignoutRequest({ state: 'hourpaths_logout' });
  const url = new URL(request.url);
  assert.equal(url.searchParams.get('client_id'), 'web');
  assert.equal(url.searchParams.has('id_token_hint'), false);
  assert.equal(url.searchParams.get('post_logout_redirect_uri'), 'https://app.example/signed-out');
  const callback = 'https://app.example/signed-out?state=' + url.searchParams.get('state');
  assert.equal((await broker.processSignoutResponse(callback)).userState, 'hourpaths_logout');
  await assert.rejects(broker.processSignoutResponse(callback));
  await assert.rejects(broker.processSignoutResponse('https://app.example/signed-out?state=forged'));
});
