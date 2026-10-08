export const NOTIFICATION_CHANNELS = ['following', 'path_access', 'tracking_activity', 'achievements', 'comments', 'reactions', 'comment_hearts', 'nudges', 'goal_reminders', 'timer_health'] as const;
export type NotificationChannel = typeof NOTIFICATION_CHANNELS[number];
export interface NotificationChannelPreference { channel: NotificationChannel; enabled: boolean; revision: number }
export interface NotificationChannelsRepository {
  list(): Promise<NotificationChannelPreference[]>;
  update(value: NotificationChannelPreference, idempotencyKey: string): Promise<NotificationChannelPreference>;
}
export class NotificationChannelFailure extends Error {
  constructor(readonly kind: 'conflict' | 'rejected' | 'unavailable' = 'unavailable') { super(`notification_channel_${kind}`); }
}

export function createNotificationChannelOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(value: NotificationChannelPreference, request: (value: NotificationChannelPreference, key: string) => Promise<NotificationChannelPreference>): Promise<
      { kind: 'applied'; preference: NotificationChannelPreference } | { kind: 'failed'; cause: unknown } | { kind: 'superseded' | 'busy' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch;
      active = generation;
      const frozen = { ...value };
      const signature = JSON.stringify([frozen.channel, frozen.revision, frozen.enabled]);
      const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() };
      retry = attempt;
      try {
        const preference = await request(frozen, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (preference.channel !== frozen.channel || preference.enabled !== frozen.enabled || preference.revision !== frozen.revision + 1) throw new NotificationChannelFailure();
        retry = undefined;
        return { kind: 'applied', preference };
      } catch (cause) {
        return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
