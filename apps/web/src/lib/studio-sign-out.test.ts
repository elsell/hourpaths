import test from 'node:test';
import assert from 'node:assert/strict';
import { accountSession } from './studio/session/application/account-session';
import type { PathRepository } from './studio/paths/ports/path-repository';
import type { Path } from './studio/paths/domain/path';

test('failed sign-out stop retains credentials and retry identity; newly started timers require another review', async () => {
  let current = true, signedOut = 0, failing = true, serial = 0;
  const running = new Map([['p', 'first']]);
  const stops: string[] = [];
  const paths = {
    list: async () => [{ id: 'p', name: 'path', canTrack: true, canEdit: true, canManageGoals: true, canManageLifecycle: true, archived: false, pinned: false, position: null, pinnedPosition: null, recentActivityAt: 0, goal: null, overallTarget: null }] as Path[],
    tracking: async (id: string) => ({ savedTotalSeconds: 0, period: null, activeSession: running.has(id) ? { id: running.get(id)!, startedAt: 0 } : null }),
    stop: async (id: string, timerId: string, key: string) => { stops.push(key); if (failing) throw new Error('unavailable'); running.delete(id); return { savedTotalSeconds: 10, period: null, activeSession: null }; },
  } satisfies Pick<PathRepository, 'list' | 'tracking' | 'stop'>;
  const session = accountSession(paths, () => current, () => { signedOut++; }, () => String(++serial));
  const review = await session.review();
  await assert.rejects(session.stopAndSave(review)); assert.equal(signedOut, 0);
  failing = false;
  await session.stopAndSave(review); assert.equal(signedOut, 1); assert.equal(stops[0], stops[1]);
  running.set('p', 'new');
  await assert.rejects(session.stopAndSave(review)); assert.equal(signedOut, 1); assert.equal(running.get('p'), 'new');
  current = false;
  assert.throws(() => session.keepRunning()); assert.equal(signedOut, 1);
});

test('cancelling a pending stop or replacing the account cannot complete sign-out', async () => {
  for (const replace of [false, true]) {
    let current = true, signedOut = false;
    let finish!: () => void;
    let started!: () => void;
    const stopping = new Promise<void>(resolve => { started = resolve; });
    const abort = new AbortController();
    const path: Path = { id: 'p', name: 'path', canTrack: true, canEdit: true, canManageGoals: true, canManageLifecycle: true, archived: false, pinned: false, position: null, pinnedPosition: null, recentActivityAt: 0, goal: null, overallTarget: null };
    const paths = {
      list: async () => [path],
      tracking: async () => ({ savedTotalSeconds: 0, period: null, activeSession: { id: 'timer', startedAt: 0 } }),
      stop: async () => { started(); await new Promise<void>(resolve => { finish = resolve; }); return { savedTotalSeconds: 10, period: null, activeSession: null }; },
    } satisfies Pick<PathRepository, 'list' | 'tracking' | 'stop'>;
    const session = accountSession(paths, () => current, () => { signedOut = true; }, () => 'operation');
    const review = await session.review();
    const pending = session.stopAndSave(review, abort.signal);
    await stopping;
    if (replace) current = false; else abort.abort();
    finish(); await assert.rejects(pending); assert.equal(signedOut, false);
  }
});
