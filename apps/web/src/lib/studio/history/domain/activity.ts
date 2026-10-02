export interface RecordedActivity {
  readonly id: string;
  readonly pathId: string;
  readonly pathName: string;
  readonly startedAt: number;
  readonly endedAt?: number;
  readonly pending?: boolean;
  readonly seconds: number;
  readonly timeZone: string;
}
export interface HistoryStream {
  readonly pathId: string;
  readonly pathName: string;
  readonly remaining: readonly RecordedActivity[];
  readonly cursor: string | null;
  readonly loaded: boolean;
}
export interface HistoryCursor {
  readonly participantId: string;
  readonly retained?: boolean;
  readonly incomplete?: boolean;
  readonly streams: readonly HistoryStream[];
}
export interface HistoryPage {
  readonly retained?: boolean;
  readonly incomplete?: boolean;
  readonly items: readonly RecordedActivity[];
  readonly next: HistoryCursor | null;
}
