import type { VisibilityCommands } from './visibility-commands';
import type { LeaveCommands } from './leave-commands';
import type { LifecycleReview } from '../domain/lifecycle';
import type { NewPath, PathGoals, Path, PathAppearance, TrackingSnapshot } from '../domain/path';

export interface PathRepository {
  visibilityCommands(key: () => string): VisibilityCommands;
  leaveCommands(key: () => string): LeaveCommands;
  read(pathId: string, signal?: AbortSignal): Promise<Path>;
  lifecycle(review: LifecycleReview): Promise<void>;
  saveGoals(path: Path, goals: PathGoals, operationId: string): Promise<Path>;
  create(path: NewPath, operationId: string): Promise<Path>;
  rename(path: Path, name: string, operationId: string): Promise<Path>;
  reorder(paths: readonly Path[], operationId: string): Promise<void>;
  pin(pathId: string, pinned: boolean, operationId: string): Promise<void>;
  saveAppearance(pathId: string, appearance: PathAppearance, operationId: string): Promise<void>;
  appearance(pathId: string, signal?: AbortSignal): Promise<PathAppearance>;
  list(archived: boolean, signal?: AbortSignal): Promise<readonly Path[]>;
  tracking(pathId: string, signal?: AbortSignal): Promise<TrackingSnapshot>;
  start(pathId: string, operationId: string): Promise<TrackingSnapshot>;
  stop(pathId: string, sessionId: string, operationId: string): Promise<TrackingSnapshot>;
}
