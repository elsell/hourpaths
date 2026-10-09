import { createSessionApiClient } from '@hourpaths/api-client';
import { validUnavailableMinutes, UnavailablePeriodFailure, type UnavailablePeriodRepository } from '../unavailable-period';
export function apiUnavailablePeriod(baseURL: string, token: string | null, userId: string, rejected?: (token: string | null) => void): UnavailablePeriodRepository {
  function failure(status: number) { return new UnavailablePeriodFailure(status === 409 ? 'conflict' : status === 401 || status === 403 ? 'rejected' : status === 400 || status === 422 ? 'invalid' : 'unavailable'); }
  function preference(value: { enabled: boolean; startMinute: number; endMinute: number; revision: number; timeZone: string }) {
    if (!userId || typeof value.enabled !== 'boolean' || !validUnavailableMinutes(value.startMinute, value.endMinute) || !Number.isSafeInteger(value.revision) || value.revision < 0 || !value.timeZone) throw new UnavailablePeriodFailure();
    return { userId, enabled: value.enabled, startMinute: value.startMinute, endMinute: value.endMinute, revision: value.revision, timeZone: value.timeZone };
  }
  return {
    async read(signal) {
      const result = await createSessionApiClient(baseURL, () => token, signal, rejected).unavailablePeriod();
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return preference(result.data.data);
    },
    async save(change, key, signal) {
      if (change.userId !== userId || !validUnavailableMinutes(change.startMinute, change.endMinute)) throw new UnavailablePeriodFailure('invalid');
      const body = { enabled: change.enabled, startMinute: change.startMinute, endMinute: change.endMinute, expectedRevision: change.expectedRevision, reviewedTimeZone: change.reviewedTimeZone };
      const result = await createSessionApiClient(baseURL, () => token, signal, rejected).updateUnavailablePeriod(body, key);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return preference(result.data.data);
    },
  };
}
