import type { HistoryRepository, HistorySource } from '../ports/history-source';
import type { RecordedActivity } from '../domain/activity';

/** Merge ordered source pages without truncating a busy Path's history. */
export function historyRepository(source: HistorySource, pageSize = 25): HistoryRepository {
  return { async page(cursor, signal) {
    const current = cursor ?? await source.initial(signal);
    const streams = current.streams.map(stream => ({ ...stream, remaining: [...stream.remaining] }));
    const items: RecordedActivity[] = [];
    while (items.length < pageSize) {
      for (const stream of streams) {
        if (stream.remaining.length || (stream.loaded && !stream.cursor)) continue;
        const page = await source.read(stream.pathId, stream.pathName, current.participantId, stream.cursor, signal);
        if (page.next && page.next === stream.cursor) throw new Error('history_cursor_repeated');
        stream.remaining = [...page.items];
        stream.cursor = page.next;
        stream.loaded = true;
        // An empty continuation would otherwise create an unbounded request loop.
        if (!stream.remaining.length && stream.cursor) throw new Error('history_page_empty');
      }
      const stream = streams.filter(value => value.remaining.length).sort((left, right) =>
        right.remaining[0]!.startedAt - left.remaining[0]!.startedAt || right.remaining[0]!.id.localeCompare(left.remaining[0]!.id))[0];
      if (!stream) break;
      items.push(stream.remaining.shift()!);
    }
    return { items, next: streams.some(stream => stream.remaining.length || stream.cursor) ? { participantId: current.participantId, streams } : null };
  } };
}
