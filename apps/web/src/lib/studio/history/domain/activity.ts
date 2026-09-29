export interface RecordedActivity {
  readonly id: string;
  readonly pathId: string;
  readonly pathName: string;
  readonly startedAt: number;
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
  readonly streams: readonly HistoryStream[];
}
export interface HistoryPage {
  readonly items: readonly RecordedActivity[];
  readonly next: HistoryCursor | null;
}
