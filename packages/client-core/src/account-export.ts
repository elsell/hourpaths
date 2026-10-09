import type { TrackingSnapshot } from './offline-tracking';

export interface ExportProfile {
  id: string; email: string; username: string; displayName: string; description: string;
  visibility: string; timeZone: string; firstDayOfWeek: number; createdAt: string; updatedAt: string;
}
export interface ExportPath {
  id: string; name: string; visibility: string; createdAt: string; updatedAt: string; archivedAt: string | null;
  intervalGoal: { targetSeconds: number; recurrence: string; alignment: { minute?: number; hour?: number; isoWeekday?: number; month?: number; day?: number } } | null;
  overallTargetSeconds: number | null;
}
export interface ExportActivity {
  id: string; pathId: string; participantId: string; startedAt: string; endedAt: string;
  timeZone: string; note: string; createdAt: string; updatedAt: string; version: number;
}
export interface ExportPage<T> { items: T[]; nextCursor: string }
export interface AccountExportRepository {
  profile(): Promise<ExportProfile>;
  paths(archived: boolean, cursor: string): Promise<ExportPage<ExportPath>>;
  activities(pathId: string, cursor: string): Promise<ExportPage<ExportActivity>>;
}
export interface DeviceExportData {
  runningTimers: { id: string; pathId: string; startedAt: string; timeZone: string }[];
  queuedTimers: { operationId: string; timerId: string; pathId: string; kind: string; startedAt: string; endedAt: string | null; correctedStartedAt: string | null; timeZone: string }[];
  queuedActivities: { operationId: string; kind: string; id: string; pathId: string; startedAt: string; endedAt: string; timeZone: string; note: string }[];
  clockCorrections: { timerId: string; pathId: string; startedAt: string; endedAt: string; timeZone: string }[];
  excludedItemCount: number;
}
export interface AccountExportDocument {
  version: 1;
  collectionStartedAt: string;
  collectionCompletedAt: string;
  profile: ExportProfile;
  paths: ExportPath[];
  activities: ExportActivity[];
  device: DeviceExportData;
}
export class AccountExportFailure extends Error {
  constructor(readonly kind: 'unavailable' | 'account_changed' | 'invalid' = 'unavailable') { super('account_export_' + kind); }
}
const instant = (value: string) => Number.isFinite(Date.parse(value));

function deviceData(snapshot: TrackingSnapshot | null, owner: string, paths: Set<string>): DeviceExportData {
  const result: DeviceExportData = { runningTimers: [], queuedTimers: [], queuedActivities: [], clockCorrections: [], excludedItemCount: 0 };
  if (!snapshot) return result;
  if (snapshot.owner !== owner) throw new AccountExportFailure('account_changed');
  const allowed = (pathId: string) => {
    if (!paths.has(pathId) || snapshot.unavailablePaths?.includes(pathId)) { result.excludedItemCount++; return false; }
    return true;
  };
  for (const timer of snapshot.timers) if (allowed(timer.pathId)) result.runningTimers.push({ id: timer.id, pathId: timer.pathId, startedAt: timer.startedAt, timeZone: timer.timeZone });
  for (const operation of snapshot.operations) if (allowed(operation.pathId)) result.queuedTimers.push({
    operationId: operation.operationId, timerId: operation.timerId, pathId: operation.pathId, kind: operation.kind,
    startedAt: operation.startedAt, endedAt: operation.endedAt ?? null, correctedStartedAt: operation.correctedStartedAt ?? null, timeZone: operation.timeZone,
  });
  for (const operation of snapshot.activityOperations ?? []) {
    const entry = operation.activity;
    if (entry.owner !== owner) throw new AccountExportFailure('account_changed');
    if (allowed(entry.pathId)) result.queuedActivities.push({
      operationId: operation.operationId, kind: operation.kind, id: entry.id, pathId: entry.pathId,
      startedAt: entry.startedAt, endedAt: entry.endedAt, timeZone: entry.timeZone, note: entry.note ?? '',
    });
  }
  for (const correction of snapshot.corrections) if (allowed(correction.timer.pathId)) result.clockCorrections.push({
    timerId: correction.timer.id, pathId: correction.timer.pathId, startedAt: correction.timer.startedAt,
    endedAt: correction.endedAt, timeZone: correction.timer.timeZone,
  });
  return result;
}

/** Collects explicit account-owned projections; never hands raw stores or DTOs
 * to a file adapter. Every awaited boundary is followed by an ownership check. */
export async function collectAccountExport(input: {
  owner: string; repository: AccountExportRepository; current(): boolean; now(): number;
  device(): Promise<TrackingSnapshot | null>; progress?(pages: number): void;
}): Promise<AccountExportDocument> {
  const assertCurrent = () => { if (!input.owner || !input.current()) throw new AccountExportFailure('account_changed'); };
  assertCurrent();
  const startedAt = new Date(input.now()).toISOString();
  let pages = 0;
  const profile = await input.repository.profile(); assertCurrent();
  if (profile.id !== input.owner) throw new AccountExportFailure('account_changed');
  if (!instant(profile.createdAt) || !instant(profile.updatedAt)) throw new AccountExportFailure('invalid');
  async function paged<T>(fetch: (cursor: string) => Promise<ExportPage<T>>, visit: (item: T) => void) {
    let cursor = '';
    const cursors = new Set<string>();
    do {
      assertCurrent();
      const page = await fetch(cursor); assertCurrent();
      if (page.items.length > 100 || typeof page.nextCursor !== 'string' || page.nextCursor.length > 4096 || page.nextCursor && cursors.has(page.nextCursor)) throw new AccountExportFailure('invalid');
      for (const item of page.items) visit(item);
      cursor = page.nextCursor;
      if (cursor) cursors.add(cursor);
      input.progress?.(++pages);
    } while (cursor);
  }
  const paths = new Map<string, ExportPath>();
  for (const archived of [false, true]) await paged(cursor => input.repository.paths(archived, cursor), path => {
    if (!path.id || !path.name || !instant(path.createdAt) || !instant(path.updatedAt)) throw new AccountExportFailure('invalid');
    const previous = paths.get(path.id);
    if (!previous || Date.parse(path.updatedAt) >= Date.parse(previous.updatedAt)) paths.set(path.id, path);
  });
  const activities = new Map<string, ExportActivity>();
  for (const path of paths.values()) await paged(cursor => input.repository.activities(path.id, cursor), entry => {
    if (entry.participantId !== input.owner || entry.pathId !== path.id) throw new AccountExportFailure('account_changed');
    if (!entry.id || !instant(entry.startedAt) || !instant(entry.endedAt) || Date.parse(entry.endedAt) < Date.parse(entry.startedAt)) throw new AccountExportFailure('invalid');
    if (activities.has(entry.id)) throw new AccountExportFailure('invalid');
    activities.set(entry.id, entry);
  });
  const snapshot = await input.device(); assertCurrent();
  return { version: 1, collectionStartedAt: startedAt, collectionCompletedAt: new Date(input.now()).toISOString(),
    profile, paths: [...paths.values()], activities: [...activities.values()], device: deviceData(snapshot, input.owner, new Set(paths.keys())) };
}

export interface AccountExportSink {
  save(document: AccountExportDocument, current: () => boolean): Promise<'saved' | 'cancelled'>;
}
export interface AccountExportState { phase: 'idle' | 'collecting' | 'saving' | 'saved' | 'failed'; pages: number }
export class AccountExportController {
  state: AccountExportState = { phase: 'idle', pages: 0 };
  private epoch = 0;
  private disposed = false;
  private listeners = new Set<() => void>();
  constructor(private readonly input: {
    owner(): string | null; current(): boolean; repository: AccountExportRepository; now(): number;
    device(owner: string): Promise<TrackingSnapshot | null>; sink: AccountExportSink;
  }) {}
  subscribe(listener: () => void) { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; }
  private update(state: AccountExportState) { if (!this.disposed) { this.state = state; for (const listener of this.listeners) listener(); } }
  async save() {
    if (this.disposed || this.state.phase === 'collecting' || this.state.phase === 'saving') return;
    const owner = this.input.owner(), epoch = ++this.epoch;
    const current = () => !this.disposed && epoch === this.epoch && !!owner && this.input.owner() === owner && this.input.current();
    if (!owner || !current()) { this.update({ phase: 'failed', pages: 0 }); return; }
    this.update({ phase: 'collecting', pages: 0 });
    try {
      const document = await collectAccountExport({ owner, repository: this.input.repository, now: this.input.now, current,
        device: () => this.input.device(owner), progress: pages => { if (current()) this.update({ phase: 'collecting', pages }); } });
      if (!current()) return;
      this.update({ phase: 'saving', pages: this.state.pages });
      const result = await this.input.sink.save(document, current);
      if (current()) this.update({ phase: result === 'saved' ? 'saved' : 'idle', pages: this.state.pages });
    } catch { if (current()) this.update({ phase: 'failed', pages: this.state.pages }); }
  }
  cancel() { this.epoch++; this.update({ phase: 'idle', pages: 0 }); }
  dispose() { this.disposed = true; this.epoch++; this.listeners.clear(); }
}
