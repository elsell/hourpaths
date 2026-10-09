import { createSessionApiClient } from '@hourpaths/api-client';
import { validWeekday, WeekStartFailure, type WeekStartPreferenceRepository } from '../week-start-preference';
export function apiWeekStartPreference(baseURL: string, token: string | null, userId: string, rejected?: (token: string | null) => void): WeekStartPreferenceRepository {
  function failure(status: number) { return new WeekStartFailure(status === 409 ? 'conflict' : status === 401 || status === 403 ? 'rejected' : status === 400 || status === 422 ? 'invalid' : 'unavailable'); }
  function preference(day: number) { if (!userId || !validWeekday(day)) throw new WeekStartFailure(); return { userId, firstDayOfWeek: day }; }
  return {
    async read(signal) {
      const result = await createSessionApiClient(baseURL, () => token, signal, rejected).configuredWeekStart();
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return preference(result.data.data.firstDayOfWeek);
    },
    async save(change, key, signal) {
      if (change.userId !== userId || !validWeekday(change.reviewedFirstDayOfWeek) || !validWeekday(change.proposedFirstDayOfWeek)) throw new WeekStartFailure('invalid');
      const result = await createSessionApiClient(baseURL, () => token, signal, rejected).updateConfiguredWeekStart({ reviewedFirstDayOfWeek: change.reviewedFirstDayOfWeek, proposedFirstDayOfWeek: change.proposedFirstDayOfWeek }, key);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return preference(result.data.data.firstDayOfWeek);
    },
  };
}
