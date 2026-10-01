export interface ActivitySnapshot {
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
