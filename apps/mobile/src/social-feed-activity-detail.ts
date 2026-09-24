import type { ActivityDetail, ActivityRevision } from './activity-history';
import type { PracticeSessionFeedEvent } from './ui/social-feed-presentation';

export async function prepareSocialFeedActivityDetail({
  event,
  load,
  loadRevisions,
  commitRoute,
  publish,
}: {
  event: PracticeSessionFeedEvent;
  load: (pathID: string, activityID: string) => Promise<ActivityDetail>;
  loadRevisions: (pathID: string, activityID: string) => Promise<{
    items: readonly ActivityRevision[];
    nextCursor: string | null;
  }>;
  commitRoute: (pathID: string, activityID: string) => void;
  publish: (
    detail: ActivityDetail,
    revisions: readonly ActivityRevision[],
    nextCursor: string | null,
  ) => void;
}): Promise<
  | { kind: 'loaded'; detail: ActivityDetail; revisions: readonly ActivityRevision[]; nextCursor: string | null }
  | { kind: 'failed'; cause: unknown }
> {
  try {
    const detail = await load(event.path.id, event.activity.id);
    if (detail.activity.pathId !== event.path.id || detail.activity.id !== event.activity.id) {
      throw new Error('activity detail did not match the selected feed event');
    }
    const page = await loadRevisions(event.path.id, event.activity.id);
    if (page.items.some((revision) =>
      revision.pathId !== event.path.id || revision.id !== event.activity.id)) {
      throw new Error('activity revisions did not match the selected feed event');
    }
    publish(detail, page.items, page.nextCursor);
    commitRoute(event.path.id, event.activity.id);
    return { detail, kind: 'loaded', nextCursor: page.nextCursor, revisions: page.items };
  } catch (cause) {
    return { cause, kind: 'failed' };
  }
}
