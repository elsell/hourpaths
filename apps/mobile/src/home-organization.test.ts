import assert from 'node:assert/strict';
import test from 'node:test';
import type { SessionPath, TimerState } from '@hourpaths/api-client';
import {
  defaultHomePreferences,
  moveHomePath,
  organizeHomePaths,
  parseHomePreferences,
  pinHomePath,
  serializeHomePreferences,
  reorderHomePaths,
  reorderVisibleHomePaths,
  type HomePath,
} from './ui/home-organization';

const trackable = {
  inviteMembers: false,
  leavePath: false,
  manageGoals: false,
  manageLifecycle: false,
  manageMembers: false,
  manageVisibility: false,
  renamePath: false,
  trackTime: true,
  transferOwnership: false,
} as const;

function path(
  id: string,
  classification: HomePath['home']['classification'],
  recentActivityAt?: string,
): HomePath {
  return {
    id,
    name: id.replaceAll('-', ' '),
    visibility: 'private',
    capabilities: classification === 'supporting' ? { ...trackable, trackTime: false } : trackable,
    home: {
      classification,
      pinned: false,
      ...(recentActivityAt ? { recentActivityAt } : {}),
    },
  } satisfies SessionPath & HomePath;
}

function timer(pathId: string, startedAt: string): TimerState {
  return {
    accumulatedSeconds: 0,
    running: true,
    timer: {
      id: `timer-${pathId}`,
      occurrenceTimeZone: String.fromCodePoint(65, 109, 101, 114, 105, 99, 97, 47, 78, 101, 119, 95, 89, 111, 114, 107),
      pathId,
      startedAt,
    },
  };
}

test('Home filters explicit membership and keeps supporter-only Paths in their own section', () => {
  const paths = [
    path('solo', 'solo'),
    path('shared', 'shared'),
    path('supporting', 'supporting'),
  ];

  assert.deepEqual(organizeHomePaths(paths, {}, defaultHomePreferences(), 'all'), {
    active: [], pinned: [], trackable: [paths[1], paths[0]], supporting: [paths[2]],
  });
  assert.deepEqual(organizeHomePaths(paths, {}, defaultHomePreferences(), 'solo').trackable, [paths[0]]);
  assert.deepEqual(organizeHomePaths(paths, {}, defaultHomePreferences(), 'shared').trackable, [paths[1]]);
  assert.deepEqual(organizeHomePaths(paths, {}, defaultHomePreferences(), 'supporting'), {
    active: [], pinned: [], trackable: [], supporting: [paths[2]],
  });
});

test('personal recent ordering, pinning, and running elevation never mutate saved order', () => {
  const paths = [
    path('never-zulu', 'solo'),
    path('older', 'shared', '2026-08-01T12:00:00Z'),
    path('newer', 'solo', '2026-08-02T12:00:00Z'),
    path('never-alpha', 'solo'),
  ];
  const preferences = pinHomePath(defaultHomePreferences(), 'older');
  const snapshot = structuredClone(preferences);

  const result = organizeHomePaths(paths, { newer: timer('newer', '2026-08-03T12:00:00Z') }, preferences, 'all');

  assert.deepEqual(result.active.map(({ id }) => id), ['newer']);
  assert.deepEqual(result.pinned.map(({ id }) => id), ['older']);
  assert.deepEqual(result.trackable.map(({ id }) => id), ['never-alpha', 'never-zulu']);
  assert.deepEqual(preferences, snapshot);
});

test('supporter Paths honor manual pinned order before ordinary supporter ordering', () => {
  const paths = [
    path('support-a', 'supporting'),
    path('support-b', 'supporting'),
    path('support-c', 'supporting'),
  ];
  const organized = organizeHomePaths(paths, {}, {
    order: 'alphabetical',
    pinnedPathIDs: ['support-b', 'support-a'],
    manualPathIDs: paths.map(({ id }) => id),
  }, 'supporting');
  assert.deepEqual(organized.supporting.map(({ id }) => id), ['support-b', 'support-a', 'support-c']);
});

test('manual moves are bounded to their own pinned or unpinned collection', () => {
  const initial = {
    order: 'manual' as const,
    pinnedPathIDs: ['pinned-a', 'pinned-b'],
    manualPathIDs: ['ordinary-a', 'ordinary-b'],
  };
  assert.deepEqual(moveHomePath(initial, 'pinned-b', -1), {
    ...initial,
    pinnedPathIDs: ['pinned-b', 'pinned-a'],
  });
  assert.deepEqual(moveHomePath(initial, 'ordinary-a', 1), {
    ...initial,
    manualPathIDs: ['ordinary-b', 'ordinary-a'],
  });
  assert.deepEqual(moveHomePath(initial, 'pinned-a', -1), initial);
  assert.deepEqual(reorderHomePaths(initial, 'manual', [0], 2), {
    ...initial,
    manualPathIDs: ['ordinary-b', 'ordinary-a'],
  });
  assert.deepEqual(reorderHomePaths(initial, 'pinned', [1], 0), {
    ...initial,
    pinnedPathIDs: ['pinned-b', 'pinned-a'],
  });
  assert.equal(reorderHomePaths(initial, 'manual', [4], 0), initial);
  assert.deepEqual(reorderVisibleHomePaths(
    { ...initial, manualPathIDs: ['ordinary-a', 'archived', 'ordinary-b'] },
    'manual',
    ['ordinary-a', 'ordinary-b'],
    [0],
    2,
  ).manualPathIDs, ['ordinary-b', 'archived', 'ordinary-a']);
});

test('stored preferences are exact, versioned, owner-bound, and fail closed', () => {
  const preferences = {
    order: 'alphabetical' as const,
    pinnedPathIDs: ['path-b'],
    manualPathIDs: ['path-a', 'path-b'],
  };
  const serialized = serializeHomePreferences('user-a', preferences);
  assert.deepEqual(parseHomePreferences(serialized, 'user-a'), preferences);
  assert.equal(parseHomePreferences(serialized, 'user-b'), null);
  assert.equal(parseHomePreferences('{"version":1,"ownerID":"user-a","preferences":{},"extra":true}', 'user-a'), null);
  assert.equal(parseHomePreferences('not json', 'user-a'), null);
});
