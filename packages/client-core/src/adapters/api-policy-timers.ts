import { createSessionApiClient } from '@hourpaths/api-client';
import type { PolicyTimerRepository } from '../policy-timers';

/** Credentials stay inside the adapter; timer controls receive domain values. */
export function apiPolicyTimers(baseURL: string, credential: () => string | null, rejected?: (token: string | null) => void): PolicyTimerRepository {
  function client() {
    const captured = credential();
    if (!captured) throw new Error('policy_timer_session_unavailable');
    return { api: createSessionApiClient(baseURL, () => captured, undefined, rejected),
      assertCurrent() { if (credential() !== captured) throw new Error('policy_timer_session_changed'); } };
  }
  return {
    async list(cursor) {
      const session = client();
      const result = await session.api.ownRunningTimers(cursor);
      session.assertCurrent();
      if (!result.response.ok || !result.data) throw new Error('policy_timers_unavailable');
      const timers = result.data.data.map(timer => {
        if (!timer.id || !timer.pathId || !timer.pathName || !Number.isFinite(Date.parse(timer.startedAt))) throw new Error('policy_timers_invalid');
        return { id: timer.id, pathId: timer.pathId, name: timer.pathName, startedAt: timer.startedAt };
      });
      const nextCursor = result.data.meta.nextCursor ?? '';
      if (cursor && cursor === nextCursor) throw new Error('policy_timers_invalid');
      return { timers, nextCursor };
    },
    async stop(timer, key) {
      const session = client();
      const result = await session.api.stopTimer(timer.pathId, timer.id, key);
      session.assertCurrent();
      if (!result.response.ok || !result.data) throw new Error('policy_timer_stop_unavailable');
    },
  };
}
