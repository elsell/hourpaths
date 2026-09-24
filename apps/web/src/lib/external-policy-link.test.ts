import assert from 'node:assert/strict';
import test from 'node:test';
import { openWebPolicyLink } from './external-policy-link';

test('opens an external HTTPS policy destination through the reviewed browser effect', () => {
  const opened: string[] = [];
  let unavailable = false;
  openWebPolicyLink('https://policies.example.test/terms', () => { unavailable = true; }, {
    open: (url) => { opened.push(url); },
  });
  assert.deepEqual(opened, ['https://policies.example.test/terms']);
  assert.equal(unavailable, false);
});

test('rejects non-HTTPS and credential-bearing policy destinations', () => {
  for (const destination of [
    '/terms',
    'http://policies.example.test/terms',
    'https://user:secret@policies.example.test/terms',
  ]) {
    let opened = false;
    let unavailable = false;
    openWebPolicyLink(destination, () => { unavailable = true; }, { open: () => { opened = true; } });
    assert.equal(opened, false);
    assert.equal(unavailable, true);
  }
});

test('surfaces a browser effect that cannot create the external policy window', () => {
  let unavailable = false;
  openWebPolicyLink('https://policies.example.test/privacy', () => { unavailable = true; }, {
    open: () => { throw new Error(['popup', 'blocked'].join(' ')); },
  });
  assert.equal(unavailable, true);
});
