import { currentGoalPeriod, type CalendarGoal } from './calendar-goal';
/** Credential loss pauses replay while preserving the account's durable work. */
export class TrackingReplaySuspended extends Error {
  constructor() { super('tracking_replay_suspended'); }
}

// Durable tracking values contain only JSON data, never platform objects.
function copy<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T; }

export interface TrackingSummary {
  savedTotalSeconds: number;
  period: { savedSeconds: number; targetSeconds: number; startsAt: number; endsAt: number } | null;
}
export interface LocalTrackingView extends TrackingSummary {
  activeSession: { id: string; startedAt: number; originalStartedAt: string; timeZone: string } | null;
  pending: boolean;
}
export interface RetainedTrackingPath { id: string; name: string; timeZone: string; goal?: CalendarGoal | null }
export interface LocalTimer { serverId?: string; id: string; pathId: string; startedAt: string; timeZone: string }
export interface TrackingOperation extends LocalTimer {
  operationId: string;
  timerId: string;
  kind: 'start' | 'stop' | 'correct';
  correctedStartedAt?: string;
  endedAt?: string;
}
export type TrackingRejection = 'membership' | 'deleted' | 'conflict' | 'archived' | 'validation';
export interface RetainedActivity {
  version?: number; note?: string; createdAt?: string; updatedAt?: string;
  id: string; owner: string; pathId: string; pathName?: string; startedAt: string; endedAt: string; timeZone: string;
}
export type TrackingOutcome = { activity?: RetainedActivity | null; serverTimerId?: string; terminal?: boolean; mustStop?: boolean } & ({ kind: 'accepted' } | {
  kind: 'rejected'; reason: TrackingRejection; disclosePath: boolean; savedSeconds?: number; discardedSeconds?: number;
});
export interface TrackingSync { send(owner: string, operation: TrackingOperation): Promise<TrackingOutcome> }
export interface TrackingNotice { id: string; reason: TrackingRejection | 'subsecond'; pathId?: string; savedSeconds?: number; discardedSeconds?: number }
export interface TrackingSnapshot {
  owner: string;
  revision: number;
  paths: RetainedTrackingPath[];
  unavailablePaths?: string[];
  history: RetainedActivity[];
  historyRetainedAt: string | null;
  summaries: Record<string, TrackingSummary>;
  unreflected: RetainedActivity[];
  timers: LocalTimer[];
  operations: TrackingOperation[];
  corrections: { timer: LocalTimer; endedAt: string; reviewedStartedAt?: string }[];
  notices: TrackingNotice[];
}
/** Implementations must atomically compare revision and persist the entire change. */
export interface TrackingStore {
  read(owner: string): Promise<TrackingSnapshot | null>;
  commit(owner: string, expectedRevision: number, snapshot: TrackingSnapshot): Promise<boolean>;
}

/** Account-bound durable commands. UI publishes only after the commit resolves. */
export class OfflineTracking {
  private disposed = false;
  private periods = new Map<string, { key: string; value: ReturnType<typeof currentGoalPeriod> }>();
  private replaying: Promise<void> | null = null;
  constructor(
    private readonly store: TrackingStore,
    private readonly owner: string,
    private readonly now: () => number,
    private readonly newId: () => string,
  ) {
    if (!owner.trim()) throw new Error('tracking_owner_required');
  }
  private periodFor(path: RetainedTrackingPath, now: number) {
    if (!path.goal) return null;
    const key = JSON.stringify([path.timeZone, path.goal]);
    const previous = this.periods.get(path.id);
    if (previous?.key === key && now >= previous.value.startsAt && now < previous.value.endsAt) return previous.value;
    const value = currentGoalPeriod(path.goal, path.timeZone, now);
    this.periods.set(path.id, { key, value });
    return value;
  }
  private validatePath(path: RetainedTrackingPath): void {
    if (!path.id || !path.name || !path.timeZone) throw new Error('tracking_home_invalid');
    new Intl.DateTimeFormat('en', { timeZone: path.timeZone });
    if (path.goal) this.periodFor(path, this.now());
  }
  async snapshot(): Promise<TrackingSnapshot> {
    const value = await this.store.read(this.owner);
    if (value && value.owner !== this.owner) throw new Error('tracking_owner_mismatch');
    if (value) {
      const cutoff = this.now() - 90 * 24 * 60 * 60 * 1000;
      value.summaries ??= {};
      value.unreflected ??= [];
      value.history = value.history.filter(entry => Date.parse(entry.endedAt) >= cutoff);
      return value;
    }
    return { owner: this.owner, revision: 0, paths: [], history: [], historyRetainedAt: null, summaries: {}, unreflected: [], timers: [], operations: [], corrections: [], notices: [] };
  }
  private async change<T>(apply: (state: TrackingSnapshot) => T): Promise<T> {
    for (let attempt = 0; attempt < 8; attempt++) {
      if (this.disposed) throw new Error('tracking_disposed');
      const state = await this.snapshot();
      const revision = state.revision;
      const result = apply(state);
      state.revision++;
      if (await this.store.commit(this.owner, revision, state)) return result;
    }
    throw new Error('tracking_concurrent_change');
  }
  dispose(): void { this.disposed = true; }
  dismissNotice(id: string): Promise<void> {
    return this.change(state => { state.notices = state.notices.filter(notice => notice.id !== id); });
  }
  replay(sync: TrackingSync): Promise<void> {
    if (this.replaying) return this.replaying;
    const work = this.drain(sync).finally(() => { if (this.replaying === work) this.replaying = null; });
    this.replaying = work;
    return work;
  }
  private async drain(sync: TrackingSync): Promise<void> {
    while (!this.disposed) {
      const snapshot = await this.snapshot();
      if (this.disposed) return;
      // A clock correction must be resolved before this timer can be replayed.
      const correctionIds = new Set(snapshot.corrections.map(value => value.timer.id));
      const operation = snapshot.operations.find(value => !correctionIds.has(value.timerId));
      if (!operation) return;
      const outcome = await sync.send(this.owner, copy(operation));
      if (this.disposed) return; // Its stable identity makes a later retry safe.
      const archiveStopId = outcome.mustStop ? this.newId() : '';
      const archiveEnd = outcome.mustStop ? new Date(this.now()).toISOString() : '';
      await this.change(state => {
        if (!state.operations.some(value => value.operationId === operation.operationId)) return;
        if (outcome.activity) {
          this.validateHistory([outcome.activity]);
          if (outcome.activity.pathId !== operation.pathId) throw new Error('tracking_history_invalid');
          state.history = state.history.filter(entry => entry.id !== outcome.activity!.id);
          state.history.push(copy(outcome.activity));
          state.unreflected = state.unreflected.filter(entry => entry.id !== outcome.activity!.id);
          state.unreflected.push(copy(outcome.activity));
        }
        if (outcome.kind === 'accepted') {
          if (operation.kind === 'start' && outcome.serverTimerId) {
            const timer = state.timers.find(value => value.id === operation.timerId);
            if (timer) timer.serverId = outcome.serverTimerId;
          }
          state.operations = state.operations.filter(value => outcome.terminal
            ? value.timerId !== operation.timerId : value.operationId !== operation.operationId);
          if (outcome.terminal) state.timers = state.timers.filter(value => value.id !== operation.timerId);
          if (outcome.mustStop && state.timers.some(value => value.id === operation.timerId)) {
            this.queueStop(state, operation.timerId, archiveStopId, archiveEnd);
          }
        } else {
          if (outcome.reason === 'membership' || outcome.reason === 'deleted') {
            state.unavailablePaths = [...new Set([...(state.unavailablePaths ?? []), operation.pathId])];
            state.operations = state.operations.filter(value => value.pathId !== operation.pathId);
            state.timers = state.timers.filter(value => value.pathId !== operation.pathId);
            state.corrections = state.corrections.filter(value => value.timer.pathId !== operation.pathId);
          }
          state.operations = state.operations.filter(value => value.timerId !== operation.timerId);
          state.timers = state.timers.filter(value => value.id !== operation.timerId);
          state.corrections = state.corrections.filter(value => value.timer.id !== operation.timerId);
          if (operation.kind === 'correct' && outcome.reason === 'validation') {
            state.corrections.push({ timer: { id: operation.timerId, pathId: operation.pathId, startedAt: operation.startedAt,
              timeZone: operation.timeZone, ...(operation.serverId ? { serverId: operation.serverId } : {}) },
              endedAt: operation.endedAt!, reviewedStartedAt: operation.correctedStartedAt });
          }
          state.notices.push({ id: operation.operationId, reason: outcome.reason,
            ...(outcome.disclosePath ? { pathId: operation.pathId } : {}),
            ...(outcome.reason === 'archived' ? { savedSeconds: outcome.savedSeconds ?? 0, discardedSeconds: outcome.discardedSeconds ?? 0 } : {}) });
        }
      });
    }
  }
  /** Refresh only after this Path's queue is settled; otherwise a server read
   * can include a lost acknowledgement and double-count the local interval. */
  async retainTracking(pathId: string, summary: TrackingSummary, timer: LocalTimer | null, expectedRevision: number): Promise<boolean> {
    if (this.disposed) throw new Error('tracking_disposed');
    this.validateTracking(pathId, summary, timer);
    const state = await this.snapshot();
    if (state.revision !== expectedRevision || state.unavailablePaths?.includes(pathId) || state.corrections.some(value => value.timer.pathId === pathId) || state.operations.some(value => value.pathId === pathId)) return false;
    this.applyTracking(state, pathId, summary, timer);
    state.revision++;
    if (this.disposed) throw new Error('tracking_disposed');
    return this.store.commit(this.owner, expectedRevision, state);
  }
  private validateTracking(pathId: string, summary: TrackingSummary, timer: LocalTimer | null): void {
    if (!Number.isSafeInteger(summary.savedTotalSeconds) || summary.savedTotalSeconds < 0
      || summary.period && (!Number.isSafeInteger(summary.period.savedSeconds) || summary.period.savedSeconds < 0
        || !Number.isFinite(summary.period.targetSeconds) || summary.period.targetSeconds <= 0
        || !Number.isFinite(summary.period.startsAt) || !Number.isFinite(summary.period.endsAt)
        || summary.period.endsAt <= summary.period.startsAt)
      || timer && (timer.pathId !== pathId || !timer.id || !Number.isFinite(Date.parse(timer.startedAt)))) {
      throw new Error('tracking_summary_invalid');
    }
  }
  private applyTracking(state: TrackingSnapshot, pathId: string, summary: TrackingSummary, timer: LocalTimer | null): void {
    const previous = timer && state.timers.find(value => value.id === timer.id || value.serverId === timer.id);
    state.summaries = { ...state.summaries, [pathId]: copy(summary) };
    state.unreflected = state.unreflected.filter(value => value.pathId !== pathId);
    state.timers = state.timers.filter(value => value.pathId !== pathId);
    if (timer) state.timers.push({ ...copy(timer), id: previous?.id ?? timer.id, serverId: timer.id });
  }
  /** The complete participating Home snapshot is published atomically. The
   * revision is captured before network reads, so a concurrent local command
   * cannot be overwritten by summaries containing an uncertain acknowledgement. */
  async retainHome(paths: RetainedTrackingPath[], entries: { pathId: string; summary: TrackingSummary; timer: LocalTimer | null }[], expectedRevision: number): Promise<boolean> {
    const identifiers = new Set(paths.map(path => path.id));
    if (identifiers.size !== paths.length || entries.length !== paths.length
      || new Set(entries.map(entry => entry.pathId)).size !== entries.length
      || entries.some(entry => !identifiers.has(entry.pathId))) throw new Error('tracking_home_invalid');
    for (const path of paths) this.validatePath(path);
    for (const entry of entries) this.validateTracking(entry.pathId, entry.summary, entry.timer);
    if (this.disposed) throw new Error('tracking_disposed');
    const state = await this.snapshot();
    if (state.revision !== expectedRevision) return false;
    state.paths = copy(paths);
    state.unavailablePaths = state.unavailablePaths?.filter(id => !identifiers.has(id));
    for (const entry of entries) {
      if (!state.corrections.some(value => value.timer.pathId === entry.pathId) && !state.operations.some(operation => operation.pathId === entry.pathId)) this.applyTracking(state, entry.pathId, entry.summary, entry.timer);
    }
    state.revision++;
    if (this.disposed) throw new Error('tracking_disposed');
    return this.store.commit(this.owner, expectedRevision, state);
  }
  async tracking(pathId: string): Promise<LocalTrackingView | null> {
    const state = await this.snapshot();
    if (state.unavailablePaths?.includes(pathId)) return null;
    const summary = Object.hasOwn(state.summaries, pathId) ? state.summaries[pathId] : undefined;
    if (!summary) return null;
    const pending = state.operations.filter(value => value.pathId === pathId);
    const intervals = [
      ...state.unreflected.filter(value => value.pathId === pathId),
      ...pending.filter(value => value.kind !== 'start' && value.endedAt).map(value => ({ startedAt: value.correctedStartedAt ?? value.startedAt, endedAt: value.endedAt! })),
    ];
    const seconds = (start: number, end: number) => Math.max(0, Math.floor((end - start) / 1000));
    const total = intervals.reduce((sum, value) => sum + seconds(Date.parse(value.startedAt), Date.parse(value.endedAt)), 0);
    const path = state.paths.find(value => value.id === pathId);
    let period = summary.period;
    let periodIntervals = intervals;
    if (path && path.goal !== undefined) {
      const bounds = this.periodFor(path, this.now());
      if (!bounds) period = null;
      else if (summary.period?.startsAt === bounds.startsAt && summary.period.endsAt === bounds.endsAt) {
        period = { ...bounds, savedSeconds: summary.period.savedSeconds };
      } else {
        // The old authoritative total belongs only to its old period. Use the
        // retained occurrences for a different period, deduplicating replay
        // acknowledgements already present in the history snapshot.
        const recorded = new Map([...state.history, ...state.unreflected].filter(value => value.pathId === pathId).map(value => [value.id, value]));
        periodIntervals = [...recorded.values(), ...pending.filter(value => value.kind !== 'start' && value.endedAt)
          .map(value => ({ startedAt: value.correctedStartedAt ?? value.startedAt, endedAt: value.endedAt! }))];
        period = { ...bounds, savedSeconds: 0 };
      }
    }
    const periodDelta = period ? periodIntervals.reduce((sum, value) => sum + seconds(Math.max(period.startsAt, Date.parse(value.startedAt)), Math.min(period.endsAt, Date.parse(value.endedAt))), 0) : 0;
    const timer = state.timers.find(value => value.pathId === pathId);
    return { savedTotalSeconds: summary.savedTotalSeconds + total,
      period: period ? { ...period, savedSeconds: period.savedSeconds + periodDelta } : null,
      activeSession: timer ? { id: timer.id, startedAt: Date.parse(timer.startedAt), originalStartedAt: timer.startedAt, timeZone: timer.timeZone } : null,
      pending: pending.some(operation => !state.corrections.some(value => value.timer.id === operation.timerId)) };
  }
  /** The complete server timer snapshot is adopted only against the revision
   * captured before fetching. Pending local starts remain visible until replay. */
  async adoptTimers(timers: LocalTimer[], expectedRevision: number): Promise<boolean> {
    if (this.disposed) throw new Error('tracking_disposed');
    const paths = new Set<string>();
    for (const timer of timers) {
      if (!timer.id || !timer.pathId || !timer.timeZone || !Number.isFinite(Date.parse(timer.startedAt))
        || paths.has(timer.pathId)) throw new Error('tracking_timer_invalid');
      paths.add(timer.pathId);
    }
    const state = await this.snapshot();
    if (state.revision !== expectedRevision) return false;
    const pendingStarts = new Set(state.operations.filter(value => value.kind === 'start').map(value => value.timerId));
    const pending = state.timers.filter(value => pendingStarts.has(value.id));
    const stopping = new Set(state.operations.filter(value => value.kind !== 'start')
      .flatMap(value => [value.timerId, ...(value.serverId ? [value.serverId] : [])]));
    const adopted = timers.filter(timer => !stopping.has(timer.id) && !pending.some(value => value.pathId === timer.pathId))
      .map(timer => {
        const local = state.timers.find(value => value.serverId === timer.id || value.id === timer.id);
        return { ...copy(timer), id: local?.id ?? timer.id, serverId: timer.id };
      });
    state.timers = [...pending, ...adopted];
    state.revision++;
    if (this.disposed) throw new Error('tracking_disposed');
    return this.store.commit(this.owner, expectedRevision, state);
  }
  private validateHistory(entries: RetainedActivity[]): void {
    const ids = new Set<string>();
    for (const entry of entries) {
      if (entry.owner !== this.owner || !entry.id || !entry.pathId || !entry.timeZone
        || !Number.isFinite(Date.parse(entry.startedAt)) || !Number.isFinite(Date.parse(entry.endedAt))
        || Date.parse(entry.endedAt) <= Date.parse(entry.startedAt) || ids.has(entry.id)) {
        throw new Error('tracking_history_invalid');
      }
      ids.add(entry.id);
    }
  }
  /** Commit a fully paginated own-history snapshot, never a partial page.
   * The revision captured before fetching fences concurrent stops/replay. */
  async retainHistory(entries: RetainedActivity[], expectedRevision: number): Promise<boolean> {
    this.validateHistory(entries);
    if (this.disposed) throw new Error('tracking_disposed');
    const state = await this.snapshot();
    if (state.revision !== expectedRevision) return false;
    const instant = this.now();
    const cutoff = instant - 90 * 24 * 60 * 60 * 1000;
    state.history = copy(entries.filter(entry => Date.parse(entry.endedAt) >= cutoff));
    state.historyRetainedAt = new Date(instant).toISOString();
    state.revision++;
    if (this.disposed) throw new Error('tracking_disposed');
    return this.store.commit(this.owner, expectedRevision, state);
  }
  async pendingHistory(): Promise<RetainedActivity[]> {
    const state = await this.snapshot();
    return state.operations.filter(operation => operation.kind !== 'start' && operation.endedAt
      && Date.parse(operation.endedAt) - Date.parse(operation.correctedStartedAt ?? operation.startedAt) >= 1000)
      .map(operation => ({ id: operation.operationId, owner: this.owner, pathId: operation.pathId,
        startedAt: operation.correctedStartedAt ?? operation.startedAt, endedAt: operation.endedAt!, timeZone: operation.timeZone }));
  }
  async localHistory(pathId: string): Promise<{ items: (RetainedActivity & { pending: boolean })[]; incomplete: boolean }> {
    const state = await this.snapshot();
    if (state.unavailablePaths?.includes(pathId)) throw new Error('tracking_path_unavailable');
    const saved = state.history.filter(entry => entry.pathId === pathId).map(entry => ({ ...entry, pending: false }));
    const pending = state.operations.filter(operation => operation.pathId === pathId && operation.kind !== 'start' && operation.endedAt
      && Date.parse(operation.endedAt) - Date.parse(operation.correctedStartedAt ?? operation.startedAt) >= 1000).map(operation => ({
        id: operation.operationId, owner: this.owner, pathId, startedAt: operation.correctedStartedAt ?? operation.startedAt, endedAt: operation.endedAt!,
        timeZone: operation.timeZone, pending: true,
      }));
    return { items: [...saved, ...pending].sort((a, b) => Date.parse(b.startedAt) - Date.parse(a.startedAt) || b.id.localeCompare(a.id)), incomplete: !state.historyRetainedAt };
  }
  async retainPaths(paths: RetainedTrackingPath[], expectedRevision?: number): Promise<boolean> {
    for (const path of paths) this.validatePath(path);
    if (this.disposed) throw new Error('tracking_disposed');
    const state = await this.snapshot();
    if (expectedRevision !== undefined && state.revision !== expectedRevision) return false;
    const revision = state.revision;
    state.paths = copy(paths);
    if (expectedRevision !== undefined) state.unavailablePaths = state.unavailablePaths?.filter(id => !paths.some(path => path.id === id));
    state.revision++;
    if (this.disposed) throw new Error('tracking_disposed');
    return this.store.commit(this.owner, revision, state);
  }
  async start(pathId: string): Promise<LocalTimer> {
    // IDs and instants are captured once, before any storage contention retry.
    const id = this.newId(), operationId = this.newId();
    const startedAt = new Date(this.now()).toISOString();
    return this.change(state => {
      const path = state.paths.find(candidate => candidate.id === pathId);
      if (!path || state.unavailablePaths?.includes(pathId)) throw new Error('tracking_path_unavailable');
      if (state.timers.some(timer => timer.pathId === pathId)) throw new Error('tracking_timer_already_running');
      const timer = { id, pathId, startedAt, timeZone: path.timeZone };
      state.timers.push(timer);
      state.operations.push({ ...timer, timerId: id, operationId, kind: 'start' });
      return copy(timer);
    });
  }
  async correct(timerId: string, reviewedStart: string, reviewedEnd: string): Promise<void> {
    const start = Date.parse(reviewedStart), end = Date.parse(reviewedEnd);
    if (!Number.isFinite(start) || !Number.isFinite(end) || end - start < 1000 || end > this.now()) throw new Error('tracking_correction_invalid');
    const operationId = this.newId();
    return this.change(state => {
      const correction = state.corrections.find(value => value.timer.id === timerId);
      if (!correction) throw new Error('tracking_correction_unavailable');
      if (state.unavailablePaths?.includes(correction.timer.pathId)) throw new Error('tracking_path_unavailable');
      state.operations = state.operations.filter(value => value.timerId !== timerId);
      state.operations.push({ ...correction.timer, timerId, operationId, kind: 'correct',
        correctedStartedAt: new Date(start).toISOString(), endedAt: new Date(end).toISOString() });
      state.corrections = state.corrections.filter(value => value.timer.id !== timerId);
    });
  }
  async stop(timerId: string): Promise<void> {
    const operationId = this.newId(), endedAt = new Date(this.now()).toISOString();
    return this.change(state => {
      this.queueStop(state, timerId, operationId, endedAt);
    });
  }
  private queueStop(state: TrackingSnapshot, timerId: string, operationId: string, endedAt: string): void {
      const timer = state.timers.find(candidate => candidate.id === timerId);
      if (!timer) throw new Error('tracking_timer_unavailable');
      state.timers = state.timers.filter(candidate => candidate.id !== timerId);
      if (Date.parse(endedAt) <= Date.parse(timer.startedAt)) {
        state.corrections.push({ timer, endedAt });
      } else {
        state.operations.push({ ...timer, timerId, operationId, kind: 'stop', endedAt });
        if (Date.parse(endedAt) - Date.parse(timer.startedAt) < 1000) state.notices.push({ id: operationId, reason: 'subsecond', pathId: timer.pathId });
      }
  }
}
