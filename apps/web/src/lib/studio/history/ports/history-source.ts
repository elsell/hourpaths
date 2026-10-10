import type { HistoryCursor, HistoryPage, RecordedActivity } from '../domain/activity';
export type HistoryWindow = 'all' | 'retained';
export interface HistorySource {
  initial(signal?: AbortSignal): Promise<string>;
  read(participantId: string, cursor: string | null, window: HistoryWindow, signal?: AbortSignal): Promise<{ items: readonly RecordedActivity[]; next: string | null }>;
}
export interface HistoryRepository {
  page(cursor: HistoryCursor | null, signal?: AbortSignal, window?: HistoryWindow): Promise<HistoryPage>;
}

export interface PathHistorySource {
  read(pathId: string, pathName: string, participantId: string, cursor: string | null, signal?: AbortSignal): Promise<{ items: readonly RecordedActivity[]; next: string | null }>;
}
