export type RunningTimerSnapshot = Readonly<{
  pathId: string;
  startedAt: string;
  accumulatedSeconds: number;
}>;

export type TimerStopAcknowledgement = Readonly<{
  pathId: string;
  running: false;
}>;

export type SignOutTimerFailure = Readonly<{
  timer: RunningTimerSnapshot;
  cause: unknown;
}>;

export type SignOutTimerResolutionState =
  | Readonly<{
      kind: 'awaiting_choice';
      timers: readonly RunningTimerSnapshot[];
      authorizeSignOut: false;
    }>
  | Readonly<{
      kind: 'stop_failed';
      timers: readonly RunningTimerSnapshot[];
      unresolvedTimers: readonly RunningTimerSnapshot[];
      failures: readonly SignOutTimerFailure[];
      authorizeSignOut: false;
    }>
  | Readonly<{
      kind: 'ready_to_sign_out';
      resolution: 'stopped_and_saved' | 'kept_running';
      timers: readonly RunningTimerSnapshot[];
      authorizeSignOut: true;
    }>
  | Readonly<{
      kind: 'cancelled' | 'superseded';
      timers: readonly RunningTimerSnapshot[];
      authorizeSignOut: false;
    }>;

export type StopAndSaveTimer = (
  timer: RunningTimerSnapshot,
) => Promise<unknown>;

export type SignOutTimerResolution = Readonly<{
  state(): SignOutTimerResolutionState;
  stopAndSave(stop: StopAndSaveTimer): Promise<SignOutTimerResolutionState>;
  keepRunning(): SignOutTimerResolutionState;
  cancel(): SignOutTimerResolutionState;
}>;

export type SignOutTimerResolutionCoordinator = Readonly<{
  begin(timers: readonly RunningTimerSnapshot[]): SignOutTimerResolution;
  invalidate(): void;
}>;

function validPathId(value: string): boolean {
  return value.length > 0 && value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function timerSnapshots(
  source: readonly RunningTimerSnapshot[],
): readonly RunningTimerSnapshot[] {
  const pathIds = new Set<string>();
  const snapshots = source.map((timer) => {
    if (
      !validPathId(timer.pathId) ||
      !Number.isFinite(Date.parse(timer.startedAt)) ||
      !Number.isSafeInteger(timer.accumulatedSeconds) ||
      timer.accumulatedSeconds < 0 ||
      pathIds.has(timer.pathId)
    ) {
      throw new Error('invalid running timer snapshot');
    }
    pathIds.add(timer.pathId);
    return Object.freeze({
      pathId: timer.pathId,
      startedAt: timer.startedAt,
      accumulatedSeconds: timer.accumulatedSeconds,
    });
  });
  if (snapshots.length === 0) throw new Error('invalid running timer snapshot');
  return Object.freeze(snapshots);
}

function frozenState<T extends SignOutTimerResolutionState>(state: T): T {
  return Object.freeze(state);
}

function stopAcknowledgement(value: unknown, pathId: string): TimerStopAcknowledgement {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid timer stop acknowledgement');
  }
  const record = value as Record<string, unknown>;
  if (
    Object.keys(record).length !== 2 ||
    record.pathId !== pathId ||
    record.running !== false
  ) {
    throw new Error('invalid timer stop acknowledgement');
  }
  return Object.freeze({ pathId, running: false });
}

export function createSignOutTimerResolutionCoordinator(): SignOutTimerResolutionCoordinator {
  let ownershipEpoch = 0;

  return Object.freeze({
    begin(source: readonly RunningTimerSnapshot[]): SignOutTimerResolution {
      const timers = timerSnapshots(source);
      const ownedEpoch = ++ownershipEpoch;
      let actionEpoch = 0;
      let current: SignOutTimerResolutionState = frozenState({
        kind: 'awaiting_choice',
        timers,
        authorizeSignOut: false,
      });

      const superseded = (): SignOutTimerResolutionState => frozenState({
        kind: 'superseded',
        timers,
        authorizeSignOut: false,
      });
      const owned = (): boolean => ownershipEpoch === ownedEpoch;
      const terminal = (): boolean =>
        current.kind === 'ready_to_sign_out' || current.kind === 'cancelled';

      const resolution: SignOutTimerResolution = {
        state(): SignOutTimerResolutionState {
          return owned() ? current : superseded();
        },

        async stopAndSave(stop: StopAndSaveTimer): Promise<SignOutTimerResolutionState> {
          if (!owned()) return superseded();
          if (terminal()) return current;
          const unresolved = current.kind === 'stop_failed'
            ? current.unresolvedTimers
            : timers;
          const ownedAction = ++actionEpoch;
          const settled = await Promise.all(unresolved.map(async (timer) => {
            try {
              stopAcknowledgement(await stop(timer), timer.pathId);
              return { timer, stopped: true as const };
            } catch (cause) {
              return { timer, stopped: false as const, cause };
            }
          }));

          if (!owned() || actionEpoch !== ownedAction) return superseded();
          const failures = settled
            .filter((item): item is typeof item & { stopped: false; cause: unknown } => !item.stopped)
            .map(({ timer, cause }) => Object.freeze({ timer, cause }));
          if (failures.length > 0) {
            current = frozenState({
              kind: 'stop_failed',
              timers,
              unresolvedTimers: Object.freeze(failures.map(({ timer }) => timer)),
              failures: Object.freeze(failures),
              authorizeSignOut: false,
            });
            return current;
          }

          current = frozenState({
            kind: 'ready_to_sign_out',
            resolution: 'stopped_and_saved',
            timers,
            authorizeSignOut: true,
          });
          return current;
        },

        keepRunning(): SignOutTimerResolutionState {
          if (!owned()) return superseded();
          if (terminal()) return current;
          actionEpoch += 1;
          current = frozenState({
            kind: 'ready_to_sign_out',
            resolution: 'kept_running',
            timers,
            authorizeSignOut: true,
          });
          return current;
        },

        cancel(): SignOutTimerResolutionState {
          if (!owned()) return superseded();
          if (terminal()) return current;
          actionEpoch += 1;
          current = frozenState({
            kind: 'cancelled',
            timers,
            authorizeSignOut: false,
          });
          return current;
        },
      };
      return Object.freeze(resolution);
    },

    invalidate(): void {
      ownershipEpoch += 1;
    },
  });
}
