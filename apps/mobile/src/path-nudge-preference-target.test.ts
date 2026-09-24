import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createPathNudgePreferenceTarget,
  ownsPathNudgePreferenceTarget,
  pathNudgeFailureDisposition,
  rotatePathNudgePreferenceTarget,
} from './path-nudge-preference-target';

test('nudge preference ownership retains arbitrary same-account token rotations', () => {
  const a = createPathNudgePreferenceTarget('profile-a', 'token-a', 'path-1');
  const b = rotatePathNudgePreferenceTarget(a, 'profile-a', 'token-b');
  const c = rotatePathNudgePreferenceTarget(b, 'profile-a', 'token-c');

  assert.equal(ownsPathNudgePreferenceTarget(c, 'profile-a', 'token-a', 'path-1'), true);
  assert.equal(ownsPathNudgePreferenceTarget(c, 'profile-a', 'token-b', 'path-1'), true);
  assert.equal(ownsPathNudgePreferenceTarget(c, 'profile-a', 'token-c', 'path-1'), true);
  assert.equal(ownsPathNudgePreferenceTarget(c, 'profile-b', 'token-c', 'path-1'), false);
  assert.equal(ownsPathNudgePreferenceTarget(c, 'profile-a', 'token-c', 'path-2'), false);
});

test('replacement with the same profile id starts a disjoint lineage', () => {
  const previous = rotatePathNudgePreferenceTarget(
    createPathNudgePreferenceTarget('profile-a', 'token-a', 'path-1'),
    'profile-a',
    'token-b',
  );
  const replacement = createPathNudgePreferenceTarget('profile-a', 'replacement-token', 'path-1');

  assert.equal(ownsPathNudgePreferenceTarget(replacement, 'profile-a', 'token-a', 'path-1'), false);
  assert.equal(ownsPathNudgePreferenceTarget(previous, 'profile-a', 'replacement-token', 'path-1'), false);
  assert.strictEqual(rotatePathNudgePreferenceTarget(previous, 'profile-b', 'token-c'), previous);
});

test('only the latest active credential failure may discard an owned session', () => {
  const target = rotatePathNudgePreferenceTarget(
    rotatePathNudgePreferenceTarget(
      createPathNudgePreferenceTarget('profile-a', 'token-a', 'path-1'),
      'profile-a',
      'token-b',
    ),
    'profile-a',
    'token-c',
  );
  const common = {
    discardCredential: true,
    latestOwnerID: 'profile-a',
    latestSessionToken: 'token-c',
    pathID: 'path-1',
    retryable: false,
    target,
  };

  assert.equal(pathNudgeFailureDisposition({ ...common, requestSessionToken: 'token-a' }), 'offline');
  assert.equal(pathNudgeFailureDisposition({ ...common, requestSessionToken: 'token-c' }), 'discard-current');
  assert.equal(pathNudgeFailureDisposition({
    ...common,
    latestSessionToken: 'replacement-token',
    requestSessionToken: 'token-a',
  }), 'ignore');
});

test('owned retryable and terminal failures publish distinct recovery states', () => {
  const target = createPathNudgePreferenceTarget('profile-a', 'token-a', 'path-1');
  const common = {
    discardCredential: false,
    latestOwnerID: 'profile-a',
    latestSessionToken: 'token-a',
    pathID: 'path-1',
    requestSessionToken: 'token-a',
    target,
  };
  assert.equal(pathNudgeFailureDisposition({ ...common, retryable: true }), 'offline');
  assert.equal(pathNudgeFailureDisposition({ ...common, retryable: false }), 'unavailable');
});
