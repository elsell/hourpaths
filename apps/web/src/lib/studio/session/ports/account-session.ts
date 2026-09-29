export interface RunningPathTimer { readonly pathId: string; readonly pathName: string; readonly timerId: string }
export interface AccountSession {
  review(signal?: AbortSignal): Promise<readonly RunningPathTimer[]>;
  keepRunning(): void;
  stopAndSave(reviewed: readonly RunningPathTimer[], signal?: AbortSignal): Promise<void>;
}
