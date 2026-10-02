import { TrackingReplaySuspended } from './offline-tracking';

/** Frameworks supply their scheduling/lifecycle adapters; the worker owns only
 * causal wakeups and bounded retry policy. No browser or native globals. */
export type TrackingSchedule = (work: () => void, delayMs: number) => () => void;

export class TrackingReplayWorker {
  private disposed = false;
  private paused = false;
  private again = false;
  private failures = 0;
  private pending: Promise<void> | null = null;
  private cancelRetry: (() => void) | null = null;
  constructor(
    private readonly replay: () => Promise<void>,
    private readonly schedule: TrackingSchedule,
    private readonly settled: () => void | Promise<void>,
  ) {}
  setPaused(paused: boolean): void {
    this.paused = paused;
    if (paused) { this.cancelRetry?.(); this.cancelRetry = null; }
  }
  dispose(): void {
    this.disposed = true;
    this.again = false;
    this.cancelRetry?.();
    this.cancelRetry = null;
  }
  idle(): Promise<void> { return this.pending ?? Promise.resolve(); }
  wake(): Promise<void> {
    if (this.disposed || this.paused) return Promise.resolve();
    this.cancelRetry?.();
    this.cancelRetry = null;
    if (this.pending) { this.again = true; return this.pending; }
    const work: Promise<void> = this.run().finally(() => {
      if (this.pending === work) this.pending = null;
      // A wake may arrive between the final drain and this completion callback.
      if (this.again && !this.disposed && !this.paused) return this.wake();
    });
    this.pending = work;
    return work;
  }
  private async run(): Promise<void> {
    do {
      this.again = false;
      try {
        await this.replay();
        this.failures = 0;
        if (!this.disposed && !this.paused) await this.settled();
      } catch (error) {
        if (error instanceof TrackingReplaySuspended) this.setPaused(true);
        // The durable queue retains the operation. A foreground/connectivity
        // wake cancels this delay, but never changes its operation identity.
        this.failures++;
        if (!this.disposed && !this.paused && !this.again) {
          const delay = Math.min(60_000, 1000 * 2 ** Math.min(this.failures - 1, 6));
          this.cancelRetry = this.schedule(() => { this.cancelRetry = null; void this.wake(); }, delay);
        }
      }
    } while (this.again && !this.disposed && !this.paused);
  }
}
