export type ForegroundNotificationPresentation = 'quiet' | 'actionable' | 'informational';

export type ForegroundNotificationContext = Readonly<{
  sessionId: string;
  targetKey: string | null;
  userId: string;
}>;

export type ForegroundNotificationReceipt = Readonly<{
  notificationId: string;
  sessionId: string;
  userId: string;
}>;

export type ForegroundNotificationResolution = Readonly<{
  presentation: Exclude<ForegroundNotificationPresentation, 'quiet'>;
  recipientUserId: string;
  targetKey: string | null;
}>;

export type ForegroundNotificationOutcome = Readonly<{
  effects: Promise<void>;
  markRead: false;
  presentation: ForegroundNotificationPresentation;
  refreshHistory: boolean;
  refreshRelevantTargetKey: string | null;
}>;

export type ForegroundNotificationCoordinator = Readonly<{
  request(receipt: ForegroundNotificationReceipt): Promise<ForegroundNotificationOutcome>;
  dispose(): void;
}>;

export type ForegroundNotificationCoordinatorPorts = Readonly<{
  current(): ForegroundNotificationContext | null;
  resolve(
    notificationId: string,
    context: ForegroundNotificationContext,
  ): Promise<ForegroundNotificationResolution | null>;
  refreshHistory(context: ForegroundNotificationContext): Promise<void>;
  refreshRelevantTarget(
    targetKey: string,
    context: ForegroundNotificationContext,
  ): Promise<void>;
}>;

function validIdentifier(value: string): boolean {
  return value.length > 0 && value.trim() === value;
}

function validContext(context: ForegroundNotificationContext | null): context is ForegroundNotificationContext {
  return context !== null
    && validIdentifier(context.sessionId)
    && validIdentifier(context.userId)
    && (context.targetKey === null || validIdentifier(context.targetKey));
}

function sameContext(
  current: ForegroundNotificationContext | null,
  expected: ForegroundNotificationContext,
): boolean {
  return validContext(current)
    && current.sessionId === expected.sessionId
    && current.userId === expected.userId
    && current.targetKey === expected.targetKey;
}

function ownedReceipt(
  receipt: ForegroundNotificationReceipt,
  context: ForegroundNotificationContext,
): boolean {
  return validIdentifier(receipt.notificationId)
    && validIdentifier(receipt.sessionId)
    && validIdentifier(receipt.userId)
    && receipt.sessionId === context.sessionId
    && receipt.userId === context.userId;
}

function quietOutcome(
  effects: Promise<void> = Promise.resolve(),
  refreshHistory = false,
): ForegroundNotificationOutcome {
  return Object.freeze({
    effects,
    markRead: false as const,
    presentation: 'quiet' as const,
    refreshHistory,
    refreshRelevantTargetKey: null,
  });
}

function validResolution(
  resolution: ForegroundNotificationResolution,
  userId: string,
): boolean {
  return resolution.recipientUserId === userId
    && validIdentifier(resolution.recipientUserId)
    && (resolution.presentation === 'actionable' || resolution.presentation === 'informational')
    && (resolution.targetKey === null || validIdentifier(resolution.targetKey));
}

export function createForegroundNotificationCoordinator(
  ports: ForegroundNotificationCoordinatorPorts,
): ForegroundNotificationCoordinator {
  let disposed = false;
  let activeKey = '';
  let active: Promise<ForegroundNotificationOutcome> | null = null;
  let cachedKey = '';
  let cached: ForegroundNotificationOutcome | null = null;
  const cacheable = new WeakSet<ForegroundNotificationOutcome>();

  const clearActive = (key: string) => {
    if (activeKey !== key) return;
    activeKey = '';
    active = null;
  };

  const runEffects = async (
    context: ForegroundNotificationContext,
    targetKey: string | null,
  ) => {
    if (disposed || !sameContext(ports.current(), context)) return;
    await ports.refreshHistory(context);
    if (disposed || !sameContext(ports.current(), context) || targetKey === null) return;
    await ports.refreshRelevantTarget(targetKey, context);
    if (disposed || !sameContext(ports.current(), context)) return;
  };

  const resolve = async (
    receipt: ForegroundNotificationReceipt,
    context: ForegroundNotificationContext,
  ): Promise<ForegroundNotificationOutcome> => {
    let resolution: ForegroundNotificationResolution | null;
    try {
      resolution = await ports.resolve(receipt.notificationId, context);
    } catch {
      const effects = runEffects(context, null);
      void effects.catch(() => {});
      return quietOutcome(effects, true);
    }
    if (disposed || !sameContext(ports.current(), context)) return quietOutcome();

    if (resolution === null) {
      const effects = runEffects(context, null);
      void effects.catch(() => {});
      return quietOutcome(effects, true);
    }
    if (!validResolution(resolution, context.userId)) return quietOutcome();

    const relevant = resolution.targetKey !== null
      && resolution.targetKey === context.targetKey;
    const effects = runEffects(context, relevant ? resolution.targetKey : null);
    void effects.catch(() => {});
    const outcome = Object.freeze({
      effects,
      markRead: false as const,
      presentation: relevant ? 'quiet' as const : resolution.presentation,
      refreshHistory: true,
      refreshRelevantTargetKey: relevant ? resolution.targetKey : null,
    });
    cacheable.add(outcome);
    return outcome;
  };

  return Object.freeze({
    request(receipt: ForegroundNotificationReceipt) {
      if (disposed) return Promise.resolve(quietOutcome());
      const context = ports.current();
      if (!validContext(context) || !ownedReceipt(receipt, context)) {
        return Promise.resolve(quietOutcome());
      }
      const key = [
        receipt.notificationId,
        context.sessionId,
        context.userId,
        context.targetKey ?? '',
      ].join('\u0000');
      if (active && activeKey === key) return active;
      if (cached && cachedKey === key) return Promise.resolve(cached);

      const handling = resolve(receipt, context);
      activeKey = key;
      active = handling;
      void handling.then((outcome) => outcome.effects.then(
        () => {
          if (!disposed && cacheable.has(outcome) && sameContext(ports.current(), context)) {
            cachedKey = key;
            cached = outcome;
          }
          clearActive(key);
        },
        () => { clearActive(key); },
      ), () => { clearActive(key); });
      return handling;
    },
    dispose() {
      disposed = true;
      activeKey = '';
      active = null;
      cachedKey = '';
      cached = null;
    },
  });
}
