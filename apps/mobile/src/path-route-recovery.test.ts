import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  createPathDetailTarget,
  createPathMemberTarget,
  createPathRouteTarget,
  ownsPathDetailTarget,
  ownsPathMemberTarget,
  ownsPathRouteTarget,
  pathRouteIntent,
  rotatePathDetailTarget,
  rotatePathMemberTarget,
  rotatePathRouteTarget,
} from './path-route-recovery';

const appSource = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('admits only exact singleton Path, People, nudge settings, member, history, and activity routes', () => {
  assert.deepEqual(pathRouteIntent('/path/path-1', { pathID: 'path-1' }), {
    kind: 'path', pathID: 'path-1', routeKey: 'path:path-1',
  });
  assert.deepEqual(pathRouteIntent('/path/path-1/history', { pathID: 'path-1' }), {
    kind: 'history', pathID: 'path-1', routeKey: 'path:path-1:history',
  });
  assert.deepEqual(pathRouteIntent('/path/path-1/members', { pathID: 'path-1' }), {
    kind: 'members', pathID: 'path-1', routeKey: 'path:path-1:members',
  });
  assert.deepEqual(pathRouteIntent('/path/path-1/nudge-settings', { pathID: 'path-1' }), {
    kind: 'nudge-settings', pathID: 'path-1', routeKey: 'path:path-1:nudge-settings',
  });
  assert.deepEqual(pathRouteIntent('/path/path-1/members/user-1', {
    pathID: 'path-1', userID: 'user-1',
  }), {
    kind: 'member', pathID: 'path-1', routeKey: 'path:path-1:member:user-1', userID: 'user-1',
  });
  assert.deepEqual(pathRouteIntent('/path/path-1/history/activity-1', {
    activityID: 'activity-1', pathID: 'path-1',
  }), {
    activityID: 'activity-1', kind: 'activity', pathID: 'path-1',
    routeKey: 'path:path-1:activity:activity-1',
  });
  assert.equal(pathRouteIntent('/path/path-2', { pathID: 'path-1' }), null);
  assert.equal(pathRouteIntent('/path/path-1/history/activity-1', {
    activityID: ['activity-1', 'activity-2'], pathID: 'path-1',
  }), null);
  assert.equal(pathRouteIntent('/path/path-1/extra', { pathID: 'path-1' }), null);
  assert.equal(pathRouteIntent('/path/path-2/nudge-settings', { pathID: 'path-1' }), null);
  assert.equal(pathRouteIntent('/path/path-1/members/user-1', {
    pathID: 'path-1', userID: ['user-1', 'user-2'],
  }), null);
});

test('same-owner token rotation extends one route lineage without admitting replacements', () => {
  const target = createPathRouteTarget('profile-a', 'token-a', 'path:path-1');
  const rotated = rotatePathRouteTarget(target, 'profile-a', 'token-b');

  assert.equal(ownsPathRouteTarget(rotated, 'profile-a', 'token-a', 'path:path-1'), true);
  assert.equal(ownsPathRouteTarget(rotated, 'profile-a', 'token-b', 'path:path-1'), true);
  assert.equal(ownsPathRouteTarget(rotated, 'profile-b', 'token-b', 'path:path-1'), false);
  assert.equal(ownsPathRouteTarget(rotated, 'profile-a', 'token-c', 'path:path-1'), false);
  assert.strictEqual(rotatePathRouteTarget(rotated, 'profile-b', 'token-c'), rotated);
});

test('cleared lineage cannot be revived by a late completion from an earlier same-ID session', () => {
  const oldTarget = createPathRouteTarget('profile-a', 'old-token', 'path:path-1');
  const replacement = createPathRouteTarget('profile-a', 'new-token', 'path:path-1');

  assert.equal(ownsPathRouteTarget(replacement, 'profile-a', 'old-token', 'path:path-1'), false);
  assert.equal(ownsPathRouteTarget(oldTarget, 'profile-a', 'new-token', 'path:path-1'), false);
});

test('warm Path operations rotate only within their activated session lineage', () => {
  const target = createPathDetailTarget('profile-a', 'token-a', 'path-1', 'activity-1');
  const rotated = rotatePathDetailTarget(target, 'profile-a', 'token-b');

  assert.equal(ownsPathDetailTarget(rotated, 'profile-a', 'token-b', 'path-1', 'activity-1'), true);
  assert.equal(ownsPathDetailTarget(rotated, 'profile-a', 'token-a', 'path-1', 'activity-1'), true);
  assert.equal(ownsPathDetailTarget(rotated, 'profile-a', 'token-b', 'path-1'), false);
  assert.strictEqual(rotatePathDetailTarget(rotated, 'profile-b', 'token-c'), rotated);

  const replacement = createPathDetailTarget('profile-a', 'replacement-token', 'path-1', 'activity-1');
  assert.equal(ownsPathDetailTarget(replacement, 'profile-a', 'token-b', 'path-1', 'activity-1'), false);
});

test('People and member operations retain exact route identity across owned rotations', () => {
  const list = rotatePathMemberTarget(createPathMemberTarget('profile-a', 'token-a', 'path-1'), 'profile-a', 'token-b');
  const detail = rotatePathMemberTarget(createPathMemberTarget('profile-a', 'token-a', 'path-1', 'user-1'), 'profile-a', 'token-b');
  assert.equal(ownsPathMemberTarget(list, 'profile-a', 'token-b', 'path-1'), true);
  assert.equal(ownsPathMemberTarget(list, 'profile-a', 'token-b', 'path-1', 'user-1'), false);
  assert.equal(ownsPathMemberTarget(detail, 'profile-a', 'token-a', 'path-1', 'user-1'), true);
  assert.equal(ownsPathMemberTarget(detail, 'profile-a', 'token-b', 'path-1', 'user-2'), false);
  assert.equal(ownsPathMemberTarget(detail, 'profile-b', 'token-b', 'path-1', 'user-1'), false);
});

test('cold People recovery distinguishes retryable transport failure from opaque terminal access loss', () => {
  assert.match(appSource, /return decision\.retryable \? 'offline' : 'unavailable'/);
  assert.match(appSource, /routeReady = false;[\s\S]*setPathRouteRecovery\(page \|\| 'offline'\)/);
  assert.match(appSource, /if \(routeReady &&/);
  assert.match(appSource, /currentPathRouteIntent\?\.kind === 'members'[\s\S]*<NativeRouteRecoveryView[\s\S]*state=\{pathRouteRecovery\}/);
});
