import type { ActivityDetail, ActivityDeletion } from '../domain/detail';
import type { ActivityRepository } from '../ports/activity-repository';
export function reviewActivityDeletion(detail: ActivityDetail, operationId: string): ActivityDeletion {
  if (!detail.owned || !detail.id || !detail.pathId || !operationId) throw new Error('activity_delete_unavailable');
  return Object.freeze({ pathId: detail.pathId, activityId: detail.id, operationId });
}
export async function deleteReviewedActivity(repository: ActivityRepository, review: ActivityDeletion): Promise<void> {
  await repository.remove(review);
}
