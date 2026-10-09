/** Presentation values only. Durable tracking remains the timer authority. */
export type RunningTimerSurface = Readonly<{
  id: string;
  pathId: string;
  name: string;
  startedAt: number;
}>;

export interface NativeTimerSurfacePort {
  replace(timers: readonly RunningTimerSurface[]): Promise<void>;
  clear(): Promise<void>;
}

/** Serializes native effects so an old account's delayed update cannot win over
 * sign-out cleanup or a replacement account. Failures never reject timer work. */
export class NativeTimerSurfaceCoordinator {
  private owner: string | null | undefined;
  private generation = 0;
  private revision = 0;
  private needsClear = true;
  private tail: Promise<boolean> = Promise.resolve(true);

  constructor(private readonly port: NativeTimerSurfacePort) {}

  account(owner: string | null): Promise<boolean> {
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

  publish(owner: string, timers: readonly RunningTimerSurface[]): Promise<boolean> {
    if (!owner || owner !== this.owner) return Promise.resolve(false);
    const snapshot = timers.map(timer => Object.freeze({ ...timer }));
    const valid = snapshot.every(timer => timer.id && timer.pathId && timer.name &&
      Number.isFinite(timer.startedAt) && timer.startedAt >= 0) &&
      new Set(snapshot.map(timer => timer.id)).size === snapshot.length;
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
      if (!valid || snapshot.length === 0) await this.port.clear();
      else await this.port.replace(snapshot);
    });
  }

  private enqueue(effect: () => Promise<void>): Promise<boolean> {
    const work = this.tail.then(effect).then(() => true, () => false);
    this.tail = work;
    return work;
  }
}
