import type { NativeGoalPeriod } from './native-goal-period';

/** Presentation values only. Durable tracking remains the timer authority. */
export type RunningTimerSurface = Readonly<{
  id: string;
  pathId: string;
  name: string;
  startedAt: number;
  stoppedAt?: number;
  action?: Readonly<{ target: string; label: string }>;
  emoji?: string;
  color?: string;
  goalPeriods?: readonly NativeGoalPeriod[];
  goal?: Readonly<{ fraction: number; text: string; lower?: number; upper?: number; progressUpper?: number; pauseAt?: number; targetText?: string }>;
}>;

export type NativeTimerAccount = Readonly<{ owner: string; expiresAt: number }>;

export interface NativeTimerSurfacePort {
  revokeActions?(): Promise<void>;
  subscribe?(changed: () => void): () => void;
  replace(timers: readonly RunningTimerSurface[], account?: NativeTimerAccount): Promise<void>;
  clear(): Promise<void>;
}

/** Serializes native effects so an old account's delayed update cannot win over
 * sign-out cleanup or a replacement account. Failures never reject timer work. */
export class NativeTimerSurfaceCoordinator {
  private owner: string | null | undefined;
  private generation = 0;
  private revision = 0;
  private needsClear = true;
  private publicationPauses = 0;
  private tail: Promise<boolean> = Promise.resolve(true);

  constructor(private readonly port: NativeTimerSurfacePort) {}

  pausePublications(): () => void {
    this.publicationPauses++;
    let released = false;
    return () => {
      if (!released) { released = true; this.publicationPauses--; }
    };
  }

  revokeAccount(): Promise<boolean> {
    this.owner = null;
    ++this.generation;
    ++this.revision;
    this.needsClear = true;
    return this.enqueue(async () => { await this.port.revokeActions?.(); });
  }

  account(owner: string | null): Promise<boolean> {
    if (owner && this.publicationPauses) return Promise.resolve(false);
    if (owner === this.owner && !this.needsClear) return this.tail;
    this.owner = owner;
    const generation = ++this.generation;
    ++this.revision;
    this.needsClear = true;
    return this.enqueue(async () => {
      await this.port.clear();
      if (generation === this.generation) this.needsClear = false;
    });
  }

  publish(owner: string, timers: readonly RunningTimerSurface[], account?: NativeTimerAccount): Promise<boolean> {
    if (this.publicationPauses || !owner || owner !== this.owner || account && (account.owner !== owner || !Number.isFinite(account.expiresAt))) return Promise.resolve(false);
    const snapshot = timers.map(timer => Object.freeze({ ...timer }));
    const valid = snapshot.every(timer => timer.id && timer.pathId && timer.name &&
      Number.isFinite(timer.startedAt) && timer.startedAt >= 0) &&
      new Set(snapshot.map(timer => timer.id)).size === snapshot.length;
    const accountSnapshot = account ? Object.freeze({ ...account }) : undefined;
    const generation = this.generation;
    const revision = ++this.revision;
    return this.enqueue(async () => {
      if (generation !== this.generation || revision !== this.revision) return;
      if (this.needsClear) {
        await this.port.clear();
        if (generation !== this.generation) return;
        this.needsClear = false;
      }
      if (revision !== this.revision) return;
      if (!valid) await this.port.clear();
      else await this.port.replace(snapshot, accountSnapshot);
    });
  }

  private enqueue(effect: () => Promise<void>): Promise<boolean> {
    const work = this.tail.then(effect).then(() => true, () => false);
    this.tail = work;
    return work;
  }
}
