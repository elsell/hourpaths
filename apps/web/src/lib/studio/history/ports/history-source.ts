import type { HistoryCursor, HistoryPage, RecordedActivity } from '../domain/activity';
export interface HistorySource {
  initial(signal?: AbortSignal): Promise<HistoryCursor>;
  read(pathId: string, pathName: string, participantId: string, cursor: string | null, signal?: AbortSignal): Promise<{ items: readonly RecordedActivity[]; next: string | null }>;
}
export interface HistoryRepository {
  page(cursor: HistoryCursor | null, signal?: AbortSignal): Promise<HistoryPage>;
}
