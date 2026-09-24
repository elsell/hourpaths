import assert from 'node:assert/strict';
import test from 'node:test';
import {
  defaultPathCreationVisibility,
  ownsPathCreationTarget,
  rotatePathCreationTarget,
} from './path-creation-target';

test('Path creation completion belongs only to its exact session and Home owner', () => {
  const target = {
    ownerID: 'owner-a',
    requestSessionToken: 'session-a',
    sessionTokens: ['session-a', 'session-b', 'session-c'],
  };

  assert.equal(ownsPathCreationTarget(target, 'session-a', 'owner-a'), true);
  assert.equal(ownsPathCreationTarget(target, 'session-b', 'owner-a'), true);
  assert.equal(ownsPathCreationTarget(target, 'session-c', 'owner-a'), true);
  assert.equal(ownsPathCreationTarget(target, 'session-d', 'owner-a'), false);
  assert.equal(ownsPathCreationTarget(target, 'session-a', 'owner-b'), false);
  assert.equal(ownsPathCreationTarget(null, 'session-a', 'owner-a'), false);
});

test('Path creation retains every same-owner rotation until completion', () => {
  const initial = {
    ownerID: 'owner-a',
    requestSessionToken: 'session-a',
    sessionTokens: ['session-a'],
  };
  const second = rotatePathCreationTarget(initial, 'session-b');
  const third = rotatePathCreationTarget(second, 'session-c');

  assert.deepEqual(third.sessionTokens, ['session-a', 'session-b', 'session-c']);
  assert.equal(rotatePathCreationTarget(third, 'session-c'), third);
});

test('Path creation defaults follow profile privacy and fail safe unresolved', () => {
  assert.equal(defaultPathCreationVisibility('public'), 'public');
  assert.equal(defaultPathCreationVisibility('private'), 'followers');
  assert.equal(defaultPathCreationVisibility(null), 'private');
});
