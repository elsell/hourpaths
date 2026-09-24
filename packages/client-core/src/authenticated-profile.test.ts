import assert from 'node:assert/strict';
import test from 'node:test';
import { authenticatedProfileFromAPI } from './index.js';

test('authenticated profile retains the owner-only visibility field', () => {
  assert.deepEqual(authenticatedProfileFromAPI({
    id: 'user-1',
    email: 'reader@example.test',
    displayName: 'Reader',
    profileVisibility: 'private',
  }), {
    id: 'user-1',
    email: 'reader@example.test',
    displayName: 'Reader',
    profileVisibility: 'private',
  });
});

test('authenticated profile rejects missing, invalid, and public-projection data', () => {
  const valid = { id: 'user-1', email: 'reader@example.test', displayName: 'Reader', profileVisibility: 'public' };
  for (const malformed of [
    { ...valid, profileVisibility: undefined },
    { ...valid, profileVisibility: 'followers' },
    { ...valid, username: 'reader' },
    { ...valid, email: '' },
  ]) assert.throws(() => authenticatedProfileFromAPI(malformed), /invalid authenticated profile/);
});
