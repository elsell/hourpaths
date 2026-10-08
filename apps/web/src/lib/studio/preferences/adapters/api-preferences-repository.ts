import { apiTimerSubscriptions, apiNotificationChannels, NotificationChannelFailure } from '@hourpaths/client-core';
import { createSessionApiClient, type GeneratedOperationResult } from '@hourpaths/api-client';
import { PreferenceFailure } from '../domain/preferences';
import type { PreferencesRepository } from '../ports/preferences-repository';
function zone(value: { timeZone: string; effectiveAt: string }) {
  const effectiveAt = Date.parse(value.effectiveAt);
  try { new Intl.DateTimeFormat('en', { timeZone: value.timeZone }); } catch { throw new PreferenceFailure(); }
  if (!Number.isFinite(effectiveAt)) throw new PreferenceFailure();
  return { zone: value.timeZone, effectiveAt };
}
function interactions(value: { commentsEnabled: boolean; reactionsEnabled: boolean }) {
  if (typeof value.commentsEnabled !== 'boolean' || typeof value.reactionsEnabled !== 'boolean') throw new PreferenceFailure();
  return { comments: value.commentsEnabled, reactions: value.reactionsEnabled };
}
function nudges(value: { enabled: boolean; revision: number }) {
  if (typeof value.enabled !== 'boolean' || !Number.isSafeInteger(value.revision) || value.revision < 0) throw new PreferenceFailure();
  return { enabled: value.enabled, revision: value.revision };
}
export function apiPreferencesRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): PreferencesRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  async function read<T>(request: Promise<GeneratedOperationResult<T>>): Promise<T> {
    try {
      const result = await request;
      if (!result.response.ok) throw new PreferenceFailure(result.response.status === 409 ? 'conflict' : result.response.status === 401 || result.response.status === 403 ? 'rejected' : 'unavailable');
      return result.data as T;
    } catch (error) {
      if (error instanceof PreferenceFailure || error instanceof DOMException && error.name === 'AbortError') throw error;
      throw new PreferenceFailure();
    }
  }
  async function channels<T>(work: () => Promise<T>): Promise<T> {
    try { return await work(); }
    catch (error) { if (error instanceof NotificationChannelFailure) throw new PreferenceFailure(error.kind); throw error; }
  }
  return {
    timerSubscription: (subject, signal) => apiTimerSubscriptions(baseURL, token(), rejected, signal).get(subject),
    saveTimerSubscription: (subject, value, key, signal) => apiTimerSubscriptions(baseURL, token(), rejected, signal).update(subject, value, key),
    notificationChannels: signal => channels(() => apiNotificationChannels(baseURL, token(), rejected, signal).list()),
    saveNotificationChannel: (value, key, signal) => channels(() => apiNotificationChannels(baseURL, token(), rejected, signal).update(value, key)),
    async identity(signal) {
      const { data } = await read(client(signal).profile());
      if (!data.id || typeof data.email !== 'string' || typeof data.displayName !== 'string' || !['public', 'private'].includes(data.profileVisibility)) throw new PreferenceFailure();
      return { id: data.id, name: data.displayName, email: data.email, visibility: data.profileVisibility };
    },
    async timeZone(signal) { return zone((await read(client(signal).configuredTimeZone())).data); },
    async changeTimeZone(value, key, signal) { return zone((await read(client(signal).updateConfiguredTimeZone({ reviewedTimeZone: value.reviewed, proposedTimeZone: value.proposed, confirmed: true }, key))).data); },
    async interactions(signal) { return interactions((await read(client(signal).getInteractionSettings())).data); },
    async saveInteractions(value, key, signal) { return interactions((await read(client(signal).updateInteractionSettings({ commentsEnabled: value.comments, reactionsEnabled: value.reactions }, key))).data); },
    async nudges(signal) { return nudges((await read(client(signal).getNudgeNotificationChannel())).data); },
    async saveNudges(value, key, signal) { return nudges((await read(client(signal).updateNudgeNotificationChannel({ enabled: value.enabled, expectedRevision: value.revision }, key))).data); },
    async blocked(cursor, signal) {
      const result = await read(client(signal).blockedAccounts(cursor));
      return { items: result.data.map(person => ({ id: person.userId, name: person.displayName, username: person.username })), next: result.meta.nextCursor || undefined };
    },
    async unblock(id, key, signal) { await read(client(signal).unblockAccount(id, key)); },
  };
}
