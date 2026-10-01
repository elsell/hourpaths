import type { QueryClient, InfiniteData } from '@tanstack/react-query';
import type { ActivityDeletion, ActivityDeletionResult } from '../history/domain/detail';
import type { HistoryPage, HistoryCursor } from '../history/domain/activity';
import type { SocialEvent, Page } from '../social/domain/activity';
import type { TrackingSnapshot } from '../paths/domain/path';

export function applyActivityDeletion(client: QueryClient, scope: string, review: ActivityDeletion, result: ActivityDeletionResult) {
  const retainedCursor = (cursor: HistoryCursor | null): HistoryCursor | null => cursor && ({ ...cursor, streams: cursor.streams.map(stream => ({ ...stream, remaining: stream.remaining.filter(item => item.id !== review.activityId) })) });
  client.setQueriesData<InfiniteData<HistoryPage>>({ queryKey: [scope, 'history'] }, data => data && ({ ...data, pageParams: data.pageParams.map(cursor => retainedCursor(cursor as HistoryCursor | null)), pages: data.pages.map(page => ({ ...page, items: page.items.filter(item => item.id !== review.activityId), next: retainedCursor(page.next) })) }));
  const removed = new Set(result.removedFeedEventIds);
  client.setQueriesData<InfiniteData<Page<SocialEvent>>>({ queryKey: [scope, 'social', 'feed'] }, data => data && ({ ...data, pages: data.pages.map(page => ({ ...page, items: page.items.filter(item => !removed.has(item.id)) })) }));
  client.setQueryData<TrackingSnapshot>([scope, 'tracking', review.pathId], data => data && ({ ...data, savedTotalSeconds: result.accumulatedSeconds, period: result.period }));
  client.removeQueries({ predicate: query => {
    const [account, area, resource, id] = query.queryKey;
    return account === scope && ((['activity', 'activity-revisions'].includes(String(area)) && resource === review.pathId && id === review.activityId) || (area === 'social' && ['comments', 'roster', 'commentHistory'].includes(String(resource)) && removed.has(String(id))) || area === 'statistics');
  } });
  // Other activity, people, preferences, appearance, and Path caches remain intact.
}
