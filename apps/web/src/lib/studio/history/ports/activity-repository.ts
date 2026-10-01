import type { ActivityDetail, ActivityRevision, ActivityDeletion, ActivityDeletionResult } from '../domain/detail';
import type { ActivityFormDefaults, ActivityWrite } from '../domain/write';
export interface ActivityRepository {
  defaults(pathId: string, signal?: AbortSignal): Promise<ActivityFormDefaults>;
  save(review: ActivityWrite): Promise<{ id: string; version: number }>;
  detail(pathId: string, activityId: string, signal?: AbortSignal): Promise<ActivityDetail>;
  revisions(pathId: string, activityId: string, cursor: string | null, signal?: AbortSignal): Promise<{ items: readonly ActivityRevision[]; next: string | null }>;
  remove(review: ActivityDeletion): Promise<ActivityDeletionResult>;
}
