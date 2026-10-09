import { createSessionApiClient } from '@hourpaths/api-client';
import { GoalReminderFailure, type GoalReminderPreference, type GoalReminderSubject, type GoalRemindersRepository } from '../goal-reminders';

export function goalReminderFromAPI(value: unknown): GoalReminderPreference {
  if (!value || typeof value !== 'object') throw new GoalReminderFailure();
  const row = value as Partial<GoalReminderPreference>;
  if (typeof row.enabled !== 'boolean' || !Number.isSafeInteger(row.revision) || row.revision! < 0) throw new GoalReminderFailure();
  return { enabled: row.enabled, revision: row.revision! };
}
function subjectValid(subject: GoalReminderSubject): void {
  if (!subject.id || subject.id.trim() !== subject.id || subject.id.length > 128 || subject.id.includes('\0')) throw new GoalReminderFailure('not_found');
}
function failure(status: number): GoalReminderFailure {
  return new GoalReminderFailure(status === 409 ? 'conflict' : status === 404 ? 'not_found' : status === 401 || status === 403 ? 'rejected' : 'unavailable');
}
/** Bound to one admitted session; presentation discards superseded account work. */
export function apiGoalReminders(baseURL: string, token: string | null, rejected?: (token: string | null) => void, signal?: AbortSignal): GoalRemindersRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async get(subject) {
      subjectValid(subject);
      const result = await api.goalReminder(subject.id);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return goalReminderFromAPI(result.data.data);
    },
    async update(subject, preference, idempotencyKey) {
      subjectValid(subject);
      const current = goalReminderFromAPI(preference);
      if (!Number.isSafeInteger(current.revision + 1)) throw new GoalReminderFailure('conflict');
      const result = await api.updateGoalReminder(subject.id, { enabled: current.enabled, expectedRevision: current.revision }, idempotencyKey);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      const saved = goalReminderFromAPI(result.data.data);
      if (saved.enabled !== current.enabled || saved.revision !== current.revision + 1) throw new GoalReminderFailure();
      return saved;
    },
  };
}
