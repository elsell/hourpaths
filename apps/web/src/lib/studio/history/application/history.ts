import type { HistoryRepository, HistorySource } from '../ports/history-source';
import type { RecordedActivity } from '../domain/activity';

/** Buffer account-ordered pages without querying each Path or mutating cursors. */
export function historyRepository(source: HistorySource, pageSize = 25): HistoryRepository {
  return { async page(cursor, signal, window = cursor?.window ?? 'all') {
    if (cursor?.retained || cursor?.window && cursor.window !== window) throw new Error('history_cursor_invalid');
    const participantId = cursor?.participantId ?? await source.initial(signal);
    const remaining = [...(cursor?.remaining ?? [])];
    let remoteCursor = cursor?.remoteCursor ?? null;
    let loaded = cursor !== null;
    const seen = new Set<string>();
    if (remoteCursor) seen.add(remoteCursor);
    const items: RecordedActivity[] = [];
    while (items.length < pageSize) {
      if (!remaining.length) {
        if (loaded && !remoteCursor) break;
        const page = await source.read(participantId, remoteCursor, window, signal);
        if (page.next && seen.has(page.next)) throw new Error('history_cursor_repeated');
        if (page.next) seen.add(page.next);
        remaining.push(...page.items);
        remoteCursor = page.next;
        loaded = true;
        // Access filtering can legitimately leave a continuation page empty.
        if (!remaining.length) continue;
      }
      items.push(remaining.shift()!);
    }
    return { items, next: remaining.length || remoteCursor ? { participantId, window, remoteCursor, remaining, streams: [] } : null };
  } };
}
