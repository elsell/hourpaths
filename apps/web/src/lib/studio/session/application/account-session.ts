import type { PathRepository } from '../../paths/ports/path-repository';
import type { AccountSession, RunningPathTimer } from '../ports/account-session';
import { SessionUnavailable } from '../domain/session';

export function accountSession(paths: Pick<PathRepository, 'list' | 'tracking' | 'stop'>, current: () => boolean, signOut: () => void, operationId: () => string): AccountSession {
  const operations = new Map<string, string>();
  const assertCurrent = () => { if (!current()) throw new SessionUnavailable(false); };
  const review = async (signal?: AbortSignal): Promise<readonly RunningPathTimer[]> => {
    assertCurrent();
    const visible = await paths.list(false, signal);
    assertCurrent();
    const result: RunningPathTimer[] = [];
    for (const path of visible.filter(path => path.canTrack && !path.archived)) {
      const snapshot = await paths.tracking(path.id, signal);
      assertCurrent();
      if (snapshot.activeSession) result.push({ pathId: path.id, pathName: path.name, timerId: snapshot.activeSession.id });
    }
    return result;
  };
  return {
    review,
    keepRunning() { assertCurrent(); signOut(); },
    async stopAndSave(reviewed, signal) {
      const assertActive = () => { assertCurrent(); if (signal?.aborted) throw new Error('sign_out_cancelled'); };
      assertActive();
      const expected = new Map(reviewed.map(timer => [timer.pathId, timer.timerId]));
      const running = await review(signal);
      assertActive();
      if (running.some(timer => expected.get(timer.pathId) !== timer.timerId)) throw new Error('sign_out_review_changed');
      for (const timer of running) {
        assertActive();
        const identity = timer.pathId + ':' + timer.timerId;
        if (!operations.has(identity)) operations.set(identity, operationId());
        const result = await paths.stop(timer.pathId, timer.timerId, operations.get(identity)!);
        assertActive();
        if (result.activeSession) throw new Error('sign_out_timer_still_running');
      }
      // Another client may have started a timer while stops were being saved.
      if ((await review(signal)).length) throw new Error('sign_out_review_changed');
      assertActive(); signOut();
    },
  };
}
