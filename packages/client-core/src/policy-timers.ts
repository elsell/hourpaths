import { OfflineTracking, type TrackingStore } from './offline-tracking';
/** Minimal account-bound timer controls available while ordinary use is paused. */
export interface PolicyTimer {
  id: string;
  pathId: string;
  name: string;
  startedAt: string;
  source: 'server' | 'device';
}
export interface PolicyTimerRepository {
  list(cursor: string): Promise<{ timers: Omit<PolicyTimer, 'source'>[]; nextCursor: string }>;
  stop(timer: Omit<PolicyTimer, 'source'>, key: string): Promise<void>;
}
export interface RetainedPolicyTimers {
  snapshot(): Promise<{
    timers: (Omit<PolicyTimer, 'source'> & { serverId?: string })[];
    pendingStops: { id: string; serverId?: string }[];
  }>;
  stop(timer: PolicyTimer): Promise<void>;
}
export interface PolicyTimersState {
  timers: PolicyTimer[];
  loading: boolean;
  stopping: string | null;
  nextCursor: string;
  error: boolean;
  pending: boolean;
}
export class PolicyTimersController {
  state: PolicyTimersState = { timers: [], loading: false, stopping: null, nextCursor: '', error: false, pending: false };
  private disposed = false;
  private epoch = 0;
  private listeners = new Set<() => void>();
  private remote: Omit<PolicyTimer, 'source'>[] = [];
  private retryKeys = new Map<string, string>();
  constructor(private readonly repository: PolicyTimerRepository, private readonly key: () => string, private readonly retained?: RetainedPolicyTimers) {}
  subscribe(listener: () => void) { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; }
  private update(patch: Partial<PolicyTimersState>) {
    if (this.disposed) return;
    this.state = { ...this.state, ...patch };
    for (const listener of this.listeners) listener();
  }
  async refresh(more = false): Promise<void> {
    if (this.disposed || this.state.loading || this.state.stopping || more && !this.state.nextCursor) return;
    const epoch = ++this.epoch, cursor = more ? this.state.nextCursor : '';
    this.update({ loading: true, error: false });
    // Read device state even when the server is unreachable. Never clear a
    // durable timer because policy review or a network failure interrupted it.
    const [local, remote] = await Promise.allSettled([
      this.retained?.snapshot() ?? Promise.resolve({ timers: [], pendingStops: [] }),
      this.repository.list(cursor),
    ]);
    if (this.disposed || epoch !== this.epoch) return;
    if (remote.status === 'fulfilled') {
      this.remote = more ? [...this.remote, ...remote.value.timers] : remote.value.timers;
    }
    if (local.status === 'rejected') { this.update({ loading: false, error: true }); return; }
    const device = local.value;
    const hidden = new Set(device ? [...device.timers, ...device.pendingStops].flatMap(timer => [timer.id, timer.serverId].filter((id): id is string => !!id)) : []);
    const paths = new Set(device?.timers.map(timer => timer.pathId) ?? []);
    const timers: PolicyTimer[] = device?.timers.map(timer => ({ ...timer, source: 'device' })) ?? this.state.timers.filter(timer => timer.source === 'device');
    for (const timer of this.remote) if (!hidden.has(timer.id) && !paths.has(timer.pathId) && !timers.some(value => value.id === timer.id)) {
      timers.push({ ...timer, source: 'server' });
    }
    this.update({ timers, loading: false, error: remote.status === 'rejected',
      pending: device ? device.pendingStops.length > 0 : this.state.pending,
      nextCursor: remote.status === 'fulfilled' ? remote.value.nextCursor : this.state.nextCursor });
  }
  async stop(id: string, source: PolicyTimer['source']): Promise<void> {
    if (this.disposed || this.state.loading || this.state.stopping) return;
    const timer = this.state.timers.find(value => value.id === id && value.source === source);
    if (!timer) return;
    const epoch = this.epoch;
    this.update({ stopping: id, error: false });
    try {
      if (source === 'device') {
        if (!this.retained) throw new Error('retained_timer_unavailable');
        await this.retained.stop({ ...timer });
      } else {
        const key = this.retryKeys.get(id) ?? this.key(); this.retryKeys.set(id, key);
        await this.repository.stop({ ...timer }, key);
      }
      if (this.disposed || epoch !== this.epoch) return;
      this.retryKeys.delete(id);
      this.remote = this.remote.filter(value => value.id !== id);
      this.update({ stopping: null, timers: this.state.timers.filter(value => value !== timer) });
      await this.refresh();
    } catch {
      if (!this.disposed && epoch === this.epoch) this.update({ stopping: null, error: true });
    }
  }
  dispose() { this.disposed = true; this.epoch++; this.listeners.clear(); this.retryKeys.clear(); }
}

/** Reuses durable stop/CAS semantics without requiring ordinary Home requests. */
export function retainedPolicyTimers(store: () => Promise<TrackingStore>, owner: () => string | null, current: () => boolean, now: () => number, newId: () => string): RetainedPolicyTimers {
  let boundOwner: string | null = null;
  async function access<T>(operation: (tracking: OfflineTracking) => Promise<T>): Promise<T> {
    const account = owner();
    if (!current() || !account || boundOwner && boundOwner !== account) throw new Error('policy_timer_owner_changed');
    boundOwner = account;
    const storage = await store();
    if (!current() || owner() !== account) throw new Error('policy_timer_owner_changed');
    const tracking = new OfflineTracking(storage, account, now, newId);
    try {
      const value = await operation(tracking);
      if (!current() || owner() !== account) throw new Error('policy_timer_owner_changed');
      return value;
    } finally { tracking.dispose(); }
  }
  return {
    snapshot: () => access(async tracking => {
      const state = await tracking.snapshot();
      return {
        timers: state.timers.map(timer => ({ id: timer.id, serverId: timer.serverId, pathId: timer.pathId, startedAt: timer.startedAt, name: state.paths.find(path => path.id === timer.pathId)?.name ?? '' })),
        pendingStops: [
          ...state.operations.filter(operation => operation.kind !== 'start').map(operation => ({ id: operation.timerId, serverId: operation.serverId })),
          ...state.corrections.map(correction => ({ id: correction.timer.id, serverId: correction.timer.serverId })),
        ],
      };
    }),
    stop: timer => access(tracking => tracking.stop(timer.id)),
  };
}
