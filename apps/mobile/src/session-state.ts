import { classifySessionFailure, type SessionAccessState, type SessionExchangeCredential, type SessionFailure } from '@hourpaths/client-core';

export type MobileSession = SessionExchangeCredential;
export type MobileSessionState = {
  session: MobileSession | null;
  accessState: SessionAccessState;
  retryable: boolean;
  storageUnreadable: boolean;
};

type MobileSessionEffects = {
  discardStored(): Promise<void>;
  transition(next: MobileSessionState): void;
  revoke(token: string): Promise<void>;
};

type MobileSessionStorageEffects = {
  read(): Promise<string | null>;
  write(session: MobileSession): Promise<void>;
  discard(): Promise<void>;
};

export type SerializedMobileSessionStorage = {
  read(): Promise<string | null>;
  persist(session: MobileSession, current: () => boolean): Promise<void>;
  discard(): Promise<void>;
  ready(): Promise<void>;
};

function unreadableStorageFailure() {
  return { kind: 'local_storage', reason: 'malformed' } as const;
}

export function createSerializedMobileSessionStorage(
  effects: MobileSessionStorageEffects,
): SerializedMobileSessionStorage {
  let queue: Promise<void> = Promise.resolve();
  let unreadable = false;
  const serialize = <T>(operation: () => Promise<T>): Promise<T> => {
    const result = queue.then(async () => {
      if (unreadable) throw unreadableStorageFailure();
      try { return await operation(); }
      catch {
        unreadable = true;
        throw unreadableStorageFailure();
      }
    });
    queue = result.then(() => undefined, () => undefined);
    return result;
  };
  return {
    read: () => serialize(effects.read),
    persist: (session, current) => serialize(async () => {
      if (current()) await effects.write(session);
    }),
    discard: () => serialize(effects.discard),
    ready: async () => {
      await queue;
      if (unreadable) throw unreadableStorageFailure();
    },
  };
}

export async function disposeMobileSession(
  current: MobileSession | null,
  requestedState: SessionAccessState,
  effects: MobileSessionEffects,
): Promise<void> {
  const transition = (storageUnreadable: boolean) => effects.transition({
    session: null,
    accessState: storageUnreadable ? 'local_session_unreadable' : requestedState,
    retryable: false,
    storageUnreadable,
  });
  const alreadyUnreadable = requestedState === 'local_session_unreadable';
  transition(alreadyUnreadable);
  let deletion: Promise<void> | undefined;
  try { deletion = effects.discardStored(); }
  catch { transition(true); }
  if (current) {
    try { void effects.revoke(current.token).catch(() => {}); }
    catch { /* Local state disposal is authoritative over remote revocation. */ }
  }
  if (deletion) {
    try { await deletion; }
    catch { transition(true); }
  }
}

export async function applyMobileSessionFailure(
  failure: SessionFailure,
  current: MobileSession,
  effects: MobileSessionEffects,
): Promise<void> {
  const decision = classifySessionFailure(failure);
  if (decision.discardCredential) {
    await disposeMobileSession(current, decision.state, effects);
    return;
  }
  effects.transition({
    session: current,
    accessState: decision.state,
    retryable: decision.retryable,
    storageUnreadable: false,
  });
}

export function shouldTransitionMobileSessionForFeatureFailure(failure: SessionFailure): boolean {
  return classifySessionFailure(failure).discardCredential;
}
