export type LifecycleAction = 'archive' | 'restore' | 'delete';
export interface LifecycleReview {
  readonly action: LifecycleAction;
  readonly pathId: string;
  readonly name: string;
  readonly expectedArchived: boolean;
  readonly operationId: string;
}
