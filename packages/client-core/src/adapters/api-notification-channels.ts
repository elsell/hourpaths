import { createSessionApiClient } from '@hourpaths/api-client';
import { NOTIFICATION_CHANNELS, NotificationChannelFailure, type NotificationChannel, type NotificationChannelPreference, type NotificationChannelsRepository } from '../notification-channels';

export function notificationChannelPreferenceFromAPI(value: unknown): NotificationChannelPreference {
  if (!value || typeof value !== 'object') throw new NotificationChannelFailure();
  const row = value as Partial<NotificationChannelPreference>;
  if (!NOTIFICATION_CHANNELS.includes(row.channel as NotificationChannel) || typeof row.enabled !== 'boolean' || !Number.isSafeInteger(row.revision) || row.revision! < 0) throw new NotificationChannelFailure();
  return { channel: row.channel!, enabled: row.enabled, revision: row.revision! };
}

export function notificationChannelPreferencesFromAPI(value: unknown): NotificationChannelPreference[] {
  if (!Array.isArray(value)) throw new NotificationChannelFailure();
  const rows = value.map(notificationChannelPreferenceFromAPI);
  if (rows.length !== NOTIFICATION_CHANNELS.length || new Set(rows.map(row => row.channel)).size !== rows.length) throw new NotificationChannelFailure();
  return NOTIFICATION_CHANNELS.map(channel => rows.find(row => row.channel === channel)!);
}

function failure(status: number): NotificationChannelFailure {
  return new NotificationChannelFailure(status === 409 ? 'conflict' : status === 401 || status === 403 ? 'rejected' : 'unavailable');
}
/** Each repository is bound to one admitted credential. Callers recreate it for
 * later operations after rotation and discard results from an old account. */
export function apiNotificationChannels(baseURL: string, token: string | null, rejected?: (token: string | null) => void, signal?: AbortSignal): NotificationChannelsRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async list() {
      const result = await api.notificationChannels();
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return notificationChannelPreferencesFromAPI(result.data.data);
    },
    async update(value, key) {
      notificationChannelPreferenceFromAPI(value);
      const result = await api.updateNotificationChannel(value.channel, { enabled: value.enabled, expectedRevision: value.revision }, key);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      const saved = notificationChannelPreferenceFromAPI(result.data.data);
      if (saved.channel !== value.channel || saved.enabled !== value.enabled || saved.revision !== value.revision + 1) throw new NotificationChannelFailure();
      return saved;
    },
  };
}
