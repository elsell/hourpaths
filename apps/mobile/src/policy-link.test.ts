import assert from 'node:assert/strict';
import test from 'node:test';
import { openMobilePolicyLink } from './policy-link';

test('a rejected native policy link open becomes a localized actionable alert', async () => {
  const events: string[] = [];
  await openMobilePolicyLink('https://example.test/terms', {
    open: async (url) => { events.push(`open:${url}`); throw new Error('unavailable'); },
    unavailable: () => { events.push('errors.temporarilyUnavailable'); },
  });
  assert.deepEqual(events, [
    'open:https://example.test/terms',
    'errors.temporarilyUnavailable',
  ]);
});

test('a successfully opened policy link does not surface an error', async () => {
  let unavailable = false;
  await openMobilePolicyLink('https://example.test/privacy', {
    open: async () => {},
    unavailable: () => { unavailable = true; },
  });
  assert.equal(unavailable, false);
});

test('non-HTTPS and credential-bearing destinations never reach native linking', async () => {
  for (const destination of [
    '/privacy',
    'http://example.test/privacy',
    'https://user:secret@example.test/privacy',
  ]) {
    let opened = false;
    let unavailable = false;
    await openMobilePolicyLink(destination, {
      open: async () => { opened = true; },
      unavailable: () => { unavailable = true; },
    });
    assert.equal(opened, false);
    assert.equal(unavailable, true);
  }
});
