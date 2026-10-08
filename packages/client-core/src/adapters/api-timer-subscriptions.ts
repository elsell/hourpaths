import { createSessionApiClient } from '@hourpaths/api-client';
import { TimerSubscriptionFailure, type TimerSubscriptionPreference, type TimerSubscriptionSubject, type TimerSubscriptionsRepository } from '../timer-subscriptions';

export function timerSubscriptionFromAPI(value: unknown): TimerSubscriptionPreference {
  if (!value || typeof value !== 'object') throw new TimerSubscriptionFailure();
  const row = value as Partial<TimerSubscriptionPreference>;
  if (typeof row.enabled !== 'boolean' || !Number.isSafeInteger(row.revision) || row.revision! < 0) throw new TimerSubscriptionFailure();
  return { enabled: row.enabled, revision: row.revision! };
}
function subjectValid(subject: TimerSubscriptionSubject): void {
  if ((subject.scope !== 'person' && subject.scope !== 'path') || !subject.id || subject.id.trim() !== subject.id || subject.id.length > 128 || subject.id.includes('\0')) throw new TimerSubscriptionFailure('not_found');
}
function failure(status: number): TimerSubscriptionFailure {
  return new TimerSubscriptionFailure(status === 409 ? 'conflict' : status === 404 ? 'not_found' : status === 401 || status === 403 ? 'rejected' : 'unavailable');
}
/** Bound to one admitted session; presentation discards superseded account work. */
export function apiTimerSubscriptions(baseURL: string, token: string | null, rejected?: (token: string | null) => void, signal?: AbortSignal): TimerSubscriptionsRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async get(subject) {
      subjectValid(subject);
      const result = await api.timerSubscription(subject.scope, subject.id);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return timerSubscriptionFromAPI(result.data.data);
    },
    async update(subject, preference, idempotencyKey) {
      subjectValid(subject);
      const current = timerSubscriptionFromAPI(preference);
      if (!Number.isSafeInteger(current.revision + 1)) throw new TimerSubscriptionFailure('conflict');
      const result = await api.updateTimerSubscription(subject.scope, subject.id, { enabled: current.enabled, expectedRevision: current.revision }, idempotencyKey);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      const saved = timerSubscriptionFromAPI(result.data.data);
      if (saved.enabled !== current.enabled || saved.revision !== current.revision + 1) throw new TimerSubscriptionFailure();
      return saved;
    },
  };
}
