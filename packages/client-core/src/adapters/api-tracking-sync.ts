import type { createSessionApiClient } from '@hourpaths/api-client';
import { TrackingReplaySuspended, type TrackingSync, type RetainedActivity } from '../offline-tracking';

type SessionAPI = ReturnType<typeof createSessionApiClient>;

/** The bootstrap supplies only the API belonging to this immutable account.
 * Generated wire values terminate here, before the tracking application port. */
export function apiTrackingSync(clientForOwner: (owner: string) => SessionAPI | null): TrackingSync {
  return {
    async sendActivity(owner, operation) {
      const client = clientForOwner(owner);
      if (!client) throw new TrackingReplaySuspended();
      const entry = operation.activity;
      if (entry.owner !== owner) throw new Error('tracking_sync_invalid');
      const durationSeconds = Math.floor((Date.parse(entry.endedAt) - Date.parse(entry.startedAt)) / 1000);
      if (!Number.isSafeInteger(durationSeconds) || durationSeconds < 1) throw new Error('tracking_sync_invalid');
      const response = await client.synchronizeOfflineActivity(entry.pathId, {
        kind: operation.kind, activityId: entry.id, startedAt: entry.startedAt, durationSeconds,
        occurrenceTimeZone: entry.timeZone, note: entry.note ?? '',
        authoredAt: operation.stamp.authoredAt, counter: operation.stamp.counter,
      }, operation.operationId);
      if (!clientForOwner(owner)) throw new TrackingReplaySuspended();
      const status = response.response.status;
      if (status === 401) throw new TrackingReplaySuspended();
      if (status === 403 || status === 404) {
        if (response.error?.operation !== 'synchronize-offline-path-activity') throw new Error('tracking_sync_temporarily_unavailable');
        return { kind: 'rejected', reason: 'membership', disclosePath: false };
      }
      if (status === 400 || status === 409 || status === 422) return { kind: 'rejected', reason: 'validation', disclosePath: true };
      if (!response.response.ok || !response.data) throw new Error('tracking_sync_temporarily_unavailable');
      const result = response.data.data;
      if (result.outcome === 'deleted' || result.outcome === 'archived') {
        if (result.activity || result.order) throw new Error('tracking_sync_invalid');
        return { kind: 'rejected', reason: result.outcome, disclosePath: true };
      }
      const saved = result.activity, order = result.order;
      if (result.outcome !== 'accepted' || !saved || !order || saved.id !== entry.id
        || saved.participantId !== owner || saved.pathId !== entry.pathId
        || !Number.isFinite(Date.parse(order.authoredAt)) || !Number.isSafeInteger(order.counter) || order.counter < 0
        || typeof order.operationId !== 'string' || !order.operationId) throw new Error('tracking_sync_invalid');
      return { kind: 'accepted', activity: { id: saved.id, owner, pathId: saved.pathId,
        startedAt: saved.startedAt, endedAt: saved.endedAt, timeZone: saved.occurrenceTimeZone,
        note: saved.note ?? '', createdAt: saved.createdAt, updatedAt: saved.updatedAt,
        editStamp: { authoredAt: order.authoredAt, counter: order.counter } } };
    },
    async send(owner, operation) {
      const client = clientForOwner(owner);
      if (!client) throw new TrackingReplaySuspended();
      const response = await client.synchronizeOfflineTimer(operation.pathId, {
        timerId: operation.timerId,
        kind: operation.kind,
        startedAt: operation.startedAt,
        ...(operation.correctedStartedAt ? { correctedStartedAt: operation.correctedStartedAt } : {}),
        ...(operation.endedAt ? { endedAt: operation.endedAt } : {}),
        occurrenceTimeZone: operation.timeZone,
      }, operation.operationId);
      if (!clientForOwner(owner)) throw new TrackingReplaySuspended();
      const status = response.response.status;
      if (status === 401) throw new TrackingReplaySuspended();
      if (status === 403 || status === 404) {
        if (response.error?.operation !== 'synchronize-offline-path-timer') throw new Error('tracking_sync_temporarily_unavailable');
        return { kind: 'rejected', reason: 'membership', disclosePath: false };
      }
      if (status === 400 || status === 409 || status === 422) return { kind: 'rejected', reason: 'validation', disclosePath: true };
      if (!response.response.ok || !response.data) throw new Error('tracking_sync_temporarily_unavailable');
      const result = response.data.data;
      if (!['accepted', 'archived', 'conflict'].includes(result.outcome)
        || !Number.isSafeInteger(result.savedSeconds) || result.savedSeconds < 0
        || !Number.isSafeInteger(result.discardedSeconds) || result.discardedSeconds < 0
        || typeof result.terminal !== 'boolean' || typeof result.mustStop !== 'boolean'
        || (result.terminal && result.mustStop)) throw new Error('tracking_sync_invalid');
      let activity: RetainedActivity | null = null;
      if (result.activity) {
        const entry = result.activity;
        if (entry.participantId !== owner || entry.pathId !== operation.pathId) throw new Error('tracking_sync_invalid');
        activity = { id: entry.id, owner, pathId: entry.pathId, startedAt: entry.startedAt,
          endedAt: entry.endedAt, timeZone: entry.occurrenceTimeZone };
      }
      if (result.timer && (result.timer.pathId !== operation.pathId || result.terminal || result.mustStop)) {
        throw new Error('tracking_sync_invalid');
      }
      if (result.outcome === 'accepted') return { kind: 'accepted', activity,
        terminal: result.terminal, mustStop: result.mustStop,
        ...(result.timer ? { serverTimerId: result.timer.id } : {}) };
      return { kind: 'rejected', reason: result.outcome === 'archived' ? 'archived' : 'conflict',
        disclosePath: true, activity, savedSeconds: result.savedSeconds, discardedSeconds: result.discardedSeconds };
    },
  };
}
