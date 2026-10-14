export type LiveActivityRefreshRegistration = Readonly<{
  owner: string;
  activityId: string;
  accountLease: string;
  token: string;
  expiresAt: number;
  boundaries: readonly number[];
}>;
export type LiveActivityRefreshIdentity = Pick<LiveActivityRefreshRegistration,
  'owner' | 'activityId' | 'accountLease' | 'expiresAt'>;
export interface LiveActivityRefreshPort {
  register(value: LiveActivityRefreshRegistration): Promise<void>;
  revoke(value: LiveActivityRefreshIdentity): Promise<void>;
}
const identity = (value: LiveActivityRefreshIdentity) => JSON.stringify([value.owner, value.activityId, value.accountLease]);

/** Network delivery is serialized separately from native timer mutations. A
 * revoked generation cannot publish queued tokens, even after a slow request. */
export class LiveActivityRefreshCoordinator {
  private current?: LiveActivityRefreshRegistration;
  private revision = 0;
  private accepted?: string;
  private pendingRevocations = new Map<string, LiveActivityRefreshIdentity>();
  private tail: Promise<boolean> = Promise.resolve(true);

  constructor(private readonly port: LiveActivityRefreshPort, private readonly now: () => number) {}

  update(input: LiveActivityRefreshRegistration): Promise<boolean> {
    if (!input.owner || !input.activityId || !input.accountLease || !input.token ||
      !Number.isFinite(input.expiresAt) || input.expiresAt <= this.now()) return this.clear();
    const same = this.current && identity(this.current) === identity(input);
    if (this.current && !same) this.stageRevocation(this.current);
    const expiresAt = Math.min(input.expiresAt, same ? this.current!.expiresAt : this.now() + 8 * 60 * 60 * 1000);
    const value = Object.freeze({ ...input, expiresAt, boundaries: Object.freeze([...new Set(input.boundaries)]
      .filter(boundary => Number.isFinite(boundary) && boundary > this.now() && boundary < expiresAt)
      .sort((a, b) => a - b).slice(0, 64)) });
    this.current = value;
    const revision = ++this.revision;
    return this.enqueue(async () => {
      await this.flushRevocations().catch(() => undefined);
      if (this.pendingRevocations.has(identity(value))) return;
      if (revision !== this.revision || expiresAt <= this.now()) return;
      const fingerprint = JSON.stringify(value);
      if (fingerprint === this.accepted) return;
      await this.port.register(value);
      if (revision === this.revision) this.accepted = fingerprint;
    });
  }

  clear(persisted?: LiveActivityRefreshIdentity): Promise<boolean> {
    if (persisted) this.stageRevocation(persisted);
    if (this.current) this.stageRevocation(this.current);
    this.current = undefined;
    this.accepted = undefined;
    ++this.revision;
    return this.enqueue(() => this.flushRevocations());
  }

  private stageRevocation(value: LiveActivityRefreshIdentity) {
    const { owner, activityId, accountLease, expiresAt } = value;
    this.pendingRevocations.set(identity(value), { owner, activityId, accountLease, expiresAt });
    this.accepted = undefined;
  }

  private async flushRevocations() {
    let failed = false;
    for (const [key, value] of this.pendingRevocations) {
      try {
        if (value.expiresAt > this.now()) await this.port.revoke(value);
        this.pendingRevocations.delete(key);
      } catch { failed = true; }
    }
    if (failed) throw new Error('live_activity_cleanup_pending');
  }

  private enqueue(work: () => Promise<void>): Promise<boolean> {
    this.tail = this.tail.then(work).then(() => true, () => false);
    return this.tail;
  }
}
