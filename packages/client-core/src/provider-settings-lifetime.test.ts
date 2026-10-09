import test from 'node:test';
import assert from 'node:assert/strict';
import { providerSettingsLifetime } from './provider-settings-lifetime';
import type { ProviderSettingsService } from './provider-identities';

test('superseded settings reject asynchronously without dispatch or confirmation ownership', async () => {
  let calls = 0;
  const remote: ProviderSettingsService = {
    owner: () => 'old', list: async () => { calls++; return []; },
    link: async () => { calls++; }, unlink: async () => { calls++; },
  };
  const service = providerSettingsLifetime(() => remote, () => false);
  assert.equal(service.owner(), null);
  for (const work of [() => service.list(), () => service.link('apple'), () => service.unlink('google', 'old')]) {
    let result!: Promise<unknown>;
    assert.doesNotThrow(() => { result = work(); });
    await assert.rejects(result, /settings_presentation_superseded/);
  }
  assert.equal(calls, 0);
});

test('replacement during a pending list cannot expose old provider identities', async () => {
  let active = true;
  let resolve!: (rows: []) => void;
  const rows = new Promise<[]>(done => { resolve = done; });
  const remote: ProviderSettingsService = {
    owner: () => 'old', list: () => rows, link: async () => {}, unlink: async () => {},
  };
  const service = providerSettingsLifetime(() => remote, () => active);
  assert.equal(service.owner(), 'old');
  const pending = service.list();
  active = false; resolve([]);
  await assert.rejects(pending, /settings_presentation_superseded/);
});
