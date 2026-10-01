import type { ActivityDetail, ActivityRevision, ActivityDeletion } from '../domain/detail';
export interface ActivityRepository {
  detail(pathId: string, activityId: string, signal?: AbortSignal): Promise<ActivityDetail>;
  revisions(pathId: string, activityId: string, cursor: string | null, signal?: AbortSignal): Promise<{ items: readonly ActivityRevision[]; next: string | null }>;
  remove(review: ActivityDeletion): Promise<void>;
}
