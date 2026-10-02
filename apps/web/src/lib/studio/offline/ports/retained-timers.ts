export interface RetainedTimersSnapshot {
  pending: boolean;
  timers: readonly { id: string; name: string; startedAt: number }[];
}
export interface RetainedTimers {
  valid(): boolean;
  snapshot(): Promise<RetainedTimersSnapshot>;
  stop(id: string): Promise<RetainedTimersSnapshot>;
}
