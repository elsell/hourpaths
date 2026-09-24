import assert from 'node:assert/strict';
import test from 'node:test';
import type { SessionPath, TimerState } from '@hourpaths/api-client';
import { homePathSections } from './ui/home-path-sections';

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

const readOnly = { ...trackable, trackTime: false } as const;

function path(id: string, capabilities: SessionPath['capabilities'] = trackable): SessionPath {
  return { id, name: id, visibility: 'private', capabilities };
}

function timer(pathId: string, startedAt: string, running = true): TimerState {
  return {
    accumulatedSeconds: 0,
    running,
    timer: {
      id: `timer-${pathId}`,
      occurrenceTimeZone: 'America/New_York',
      pathId,
      startedAt,
    },
  };
}

test('Home elevates only valid authoritative running trackable Paths without mutation or duplication', () => {
  const paths = [
    path('ordinary'),
    path('older'),
    path('supporter', readOnly),
    path('tie-first'),
    path('invalid-start'),
    path('date-only-start'),
    path('impossible-start'),
    path('newest'),
    path('tie-second'),
    path('stopped'),
    path('mismatched'),
    { ...path('archived'), archivedAt: '2026-07-27T12:00:00Z' },
    path('missing-state'),
  ] satisfies SessionPath[];
  const timers: Record<string, TimerState> = {
    older: timer('older', '2026-07-27T12:00:00Z'),
    supporter: timer('supporter', '2026-07-27T18:00:00Z'),
    'tie-first': timer('tie-first', '2026-07-27T16:00:00Z'),
    'invalid-start': timer('invalid-start', 'not-an-instant'),
    'date-only-start': timer('date-only-start', '2026-07-27'),
    'impossible-start': timer('impossible-start', '2026-02-31T00:00:00Z'),
    newest: timer('newest', '2026-07-27T17:00:00Z'),
    'tie-second': timer('tie-second', '2026-07-27T16:00:00Z'),
    stopped: timer('stopped', '2026-07-27T19:00:00Z', false),
    mismatched: timer('another-path', '2026-07-27T20:00:00Z'),
    archived: timer('archived', '2026-07-27T21:00:00Z'),
  };
  const sourceOrder = paths.map(({ id }) => id);
  const timerSnapshot = structuredClone(timers);

  const result = homePathSections(paths, timers);

  assert.deepEqual(result.active.map(({ id }) => id), ['newest', 'tie-first', 'tie-second', 'older']);
  assert.deepEqual(result.ordinary.map(({ id }) => id), [
    'ordinary',
    'supporter',
    'invalid-start',
    'date-only-start',
    'impossible-start',
    'stopped',
    'mismatched',
    'archived',
    'missing-state',
  ]);
  assert.deepEqual([...result.active, ...result.ordinary].map(({ id }) => id).sort(), [...sourceOrder].sort());
  assert.equal(new Set([...result.active, ...result.ordinary]).size, paths.length);
  assert.deepEqual(paths.map(({ id }) => id), sourceOrder);
  assert.deepEqual(timers, timerSnapshot);
  for (const projected of [...result.active, ...result.ordinary]) {
    assert.equal(projected, paths.find(({ id }) => id === projected.id));
  }
});

test('stopping one simultaneous timer restores only its Path to ordinary source order', () => {
  const paths = [
    path('before'),
    path('older-active'),
    path('between'),
    path('newer-active'),
    path('after'),
  ] satisfies SessionPath[];
  const runningTimers: Record<string, TimerState> = {
    'older-active': timer('older-active', '2026-07-27T15:00:00Z'),
    'newer-active': timer('newer-active', '2026-07-27T16:00:00Z'),
  };
  const pathsSnapshot = structuredClone(paths);
  const timersSnapshot = structuredClone(runningTimers);

  const whileBothRun = homePathSections(paths, runningTimers);
  const afterOlderStops = homePathSections(paths, {
    ...runningTimers,
    'older-active': timer('older-active', '2026-07-27T15:00:00Z', false),
  });

  assert.deepEqual(whileBothRun.active.map(({ id }) => id), ['newer-active', 'older-active']);
  assert.deepEqual(whileBothRun.ordinary.map(({ id }) => id), ['before', 'between', 'after']);
  assert.deepEqual(afterOlderStops.active.map(({ id }) => id), ['newer-active']);
  assert.deepEqual(afterOlderStops.ordinary.map(({ id }) => id), [
    'before',
    'older-active',
    'between',
    'after',
  ]);
  assert.deepEqual(paths, pathsSnapshot);
  assert.deepEqual(runningTimers, timersSnapshot);
});
