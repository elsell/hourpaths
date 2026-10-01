import type { LifecycleAction, LifecycleReview } from '../domain/lifecycle';
import type { PathRepository } from '../ports/path-repository';
export type { LifecycleAction, LifecycleReview } from '../domain/lifecycle';
export function reviewLifecycle(path: { id: string; name: string; archived: boolean; canManageLifecycle: boolean }, action: LifecycleAction, operationId: string): LifecycleReview {
  if (!path.canManageLifecycle || !path.id || !operationId ||
      (action === 'archive' && path.archived) || (action === 'restore' && !path.archived)) throw new Error('invalid_lifecycle_review');
  return Object.freeze({ action, pathId: path.id, name: path.name, expectedArchived: path.archived, operationId });
}
export async function executeLifecycle(repository: Pick<PathRepository, 'lifecycle'>, review: LifecycleReview): Promise<void> {
  await repository.lifecycle(review);
}
