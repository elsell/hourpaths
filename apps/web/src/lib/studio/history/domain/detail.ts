export interface ActivitySnapshot {
  readonly editStamp?: { authoredAt: string; counter: number };
  readonly originalStartedAt?: string;
  readonly originalEndedAt?: string;
  readonly createdAt?: string;
  readonly updatedAt?: string;
  readonly id: string;
  readonly pathId: string;
  readonly participantId: string;
  readonly startedAt: number;
  readonly endedAt: number;
  readonly seconds: number;
  readonly timeZone: string;
  readonly version: number;
  readonly note: string | null;
}
export interface ActivityDetail extends ActivitySnapshot {
  readonly pathName: string;
  readonly owned: boolean;
}
export interface ActivityRevision extends ActivitySnapshot { readonly replacedAt: number }
export interface ActivityDeletion { readonly pathId: string; readonly activityId: string; readonly operationId: string }

export interface ActivityDeletionResult {
  readonly accumulatedSeconds: number;
  readonly sessionCount: number;
  readonly unreadNotificationCount: number;
  readonly removedFeedEventIds: readonly string[];
  readonly period: { savedSeconds: number; targetSeconds: number; startsAt: number; endsAt: number } | null;
}

export class ActivityFailure extends Error {
  constructor(readonly retryable: boolean) { super('activity_unavailable'); }
}
