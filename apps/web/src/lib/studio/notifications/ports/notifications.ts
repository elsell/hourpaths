import type { NotificationChange, NotificationsSnapshot } from '../domain/notification';
export interface NotificationOperations {
  run<T>(operation: (signal: AbortSignal) => Promise<T>): Promise<T>;
  cancel(): void;
}
export interface Notifications {
  start(operations: NotificationOperations): void;
  snapshot(): NotificationsSnapshot;
  subscribe(listener: () => void): () => void;
  refresh(): Promise<void>;
  invalidate(): Promise<void>;
  loadMore(): Promise<void>;
  mutate(change: NotificationChange): Promise<boolean>;
  dispose(): void;
}
