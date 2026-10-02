import type { RetainedActivity } from '@hourpaths/client-core';
import type { HistoryRepository } from '../../history/ports/history-source';
import type { HistoryCursor, HistoryPage, RecordedActivity } from '../../history/domain/activity';
import type { TrackingRuntime } from './durable-path-repository';

/** History keeps its remote pagination contract. Only a complete, account-bound
 * refresh may replace the retained snapshot; failed pages cannot erase history. */
export function durableHistoryRepository(
  remote: HistoryRepository,
  runtime: () => Promise<TrackingRuntime>,
  temporary: (failure: unknown) => boolean,
  pageSize = 25,
  now: () => number = () => Date.now(),
): HistoryRepository & { refresh(): Promise<boolean> } {
  let refreshing: Promise<boolean> | null = null;
  const record = (entry: RetainedActivity, names: Map<string, string>, pending = false): RecordedActivity => ({
    editStamp: entry.editStamp, originalStartedAt: entry.startedAt, originalEndedAt: entry.endedAt, note: entry.note, version: entry.version, createdAt: entry.createdAt, updatedAt: entry.updatedAt,
    id: entry.id, pathId: entry.pathId, pathName: entry.pathName ?? names.get(entry.pathId) ?? '',
    startedAt: Date.parse(entry.startedAt), endedAt: Date.parse(entry.endedAt),
    seconds: Math.floor((Date.parse(entry.endedAt) - Date.parse(entry.startedAt)) / 1000), timeZone: entry.timeZone, pending,
  });
  const ordered = (items: RecordedActivity[]) => [...new Map(items.map(item => [item.id, item])).values()].sort((a, b) => b.startedAt - a.startedAt || b.id.localeCompare(a.id));
  async function local(current: TrackingRuntime, cursor: HistoryCursor | null): Promise<HistoryPage> {
    const state = await current.tracking.snapshot();
    const names = new Map(state.paths.map(path => [path.id, path.name]));
    const pending = await current.tracking.pendingHistory();
    current.assertCurrent();
    const items = cursor?.retained ? [...cursor.streams.flatMap(stream => stream.remaining)] : ordered([
      ...state.history.map(entry => record(entry, names)), ...pending.map(entry => record(entry, names, true)),
    ]);
    const incomplete = cursor?.incomplete ?? !state.historyRetainedAt;
    const remainder = items.slice(pageSize);
    return { retained: true, incomplete, items: items.slice(0, pageSize), next: remainder.length ? {
      participantId: current.owner, retained: true, incomplete,
      streams: [{ pathId: '', pathName: '', remaining: remainder, cursor: null, loaded: true }],
    } : null };
  }
  async function hydrate(): Promise<boolean> {
    const current = await runtime();
    current.assertCurrent();
    const before = await current.tracking.snapshot();
    const entries = new Map<string, RetainedActivity>();
    const cutoff = now() - 90 * 24 * 60 * 60 * 1000;
    const seen = new Set<string>();
    let cursor: HistoryCursor | null = null;
    do {
      const page = await remote.page(cursor);
      current.assertCurrent();
      if (page.retained || page.next && page.next.participantId !== current.owner) throw new Error('history_owner_mismatch');
      for (const entry of page.items) {
        if (!Number.isFinite(entry.endedAt)) throw new Error('history_interval_missing');
        if (entry.endedAt! < cutoff) continue;
        entries.set(entry.id, { id: entry.id, owner: current.owner, pathId: entry.pathId, pathName: entry.pathName,
          startedAt: entry.originalStartedAt ?? new Date(entry.startedAt).toISOString(), endedAt: entry.originalEndedAt ?? new Date(entry.endedAt!).toISOString(), timeZone: entry.timeZone,
          editStamp: entry.editStamp, note: entry.note, version: entry.version, createdAt: entry.createdAt, updatedAt: entry.updatedAt });
      }
      cursor = page.next;
      if (cursor) {
        const key = JSON.stringify(cursor.streams.map(stream => [stream.pathId, stream.cursor, stream.remaining[0]?.id, stream.loaded]));
        if (seen.has(key)) throw new Error('history_cursor_repeated');
        seen.add(key);
      }
    } while (cursor);
    current.assertCurrent();
    return current.tracking.retainHistory([...entries.values()], before.revision);
  }
  return {
    refresh() {
      if (!refreshing) refreshing = hydrate().catch(() => false).finally(() => { refreshing = null; });
      return refreshing;
    },
    async page(cursor, signal) {
      const current = await runtime();
      current.assertCurrent();
      if (cursor && cursor.participantId !== current.owner) throw new Error('history_owner_mismatch');
      if (cursor?.retained) return local(current, cursor);
      try {
        const page = await remote.page(cursor, signal);
        current.assertCurrent();
        if (page.next && page.next.participantId !== current.owner) throw new Error('history_owner_mismatch');
        if (cursor) return page;
        const state = await current.tracking.snapshot();
        const pending = await current.tracking.pendingHistory();
        current.assertCurrent();
        const names = new Map(state.paths.map(path => [path.id, path.name]));
        return { ...page, items: ordered([...page.items, ...pending.map(entry => record(entry, names, true))]) };
      } catch (error) {
        if (signal?.aborted || !temporary(error)) throw error;
        // Switching sources mid-pagination would skip or repeat entries. A
        // refresh restarts the timeline against the retained snapshot instead.
        if (cursor) throw error;
        return local(current, null);
      }
    },
  };
}
