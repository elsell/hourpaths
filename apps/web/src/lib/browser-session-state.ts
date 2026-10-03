import { browserSessionRecords, sessionRecordKey } from './studio/session/adapters/browser-session-record';
import { DurableSessionOperations } from './studio/session/application/session-operations';

let state: ReturnType<typeof createState> | undefined;
function createState() {
  const signInRevisionKey = 'hourpaths_sign_in_revision_v1';
  const records = browserSessionRecords(window.localStorage, window.sessionStorage,
    window.navigator.locks, () => crypto.randomUUID());
  const operations = new DurableSessionOperations(records.coordinator);
  let ready = false;
  let initializing: Promise<void> | undefined;
  let clearing = false;
  return {
    operations,
    beginSignIn(): void {
      window.sessionStorage.setItem(signInRevisionKey, records.coordinator.read().revision);
    },
    signInTicket() {
      const revision = window.sessionStorage.getItem(signInRevisionKey);
      window.sessionStorage.removeItem(signInRevisionKey);
      // Missing or superseded intent cannot adopt a provider callback.
      return operations.issue(revision ?? 'missing_sign_in_intent');
    },
    subscribe(changed: () => void): () => void {
      const listener = (event: StorageEvent) => {
        if (event.storageArea === window.localStorage && (event.key === sessionRecordKey || event.key === null)) changed();
      };
      window.addEventListener('storage', listener);
      return () => window.removeEventListener('storage', listener);
    },
    async refreshExclusive<T>(operation: () => Promise<T>): Promise<T> {
      return await window.navigator.locks.request('hourpaths_session_refresh_v1', operation);
    },
    initialize(): Promise<void> {
      if (ready) return Promise.resolve();
      return initializing ??= records.initialize().then(() => { ready = true; }).catch(async error => {
        await records.discardUnreadable();
        throw error;
      }).finally(() => { initializing = undefined; });
    },
    read(): string | null {
      if (!ready) throw new Error('session_storage_not_initialized');
      return clearing ? null : records.coordinator.read().value;
    },
    async discard(value: string | null): Promise<void> {
      operations.invalidate();
      clearing = true;
      try {
        let previous;
        try { previous = records.coordinator.read(); }
        catch { await records.discardUnreadable(); return; }
        await records.coordinator.commit(previous.revision, value);
      }
      finally { clearing = false; }
    },
  };
}
export function browserSessionState() { return state ??= createState(); }

export function invalidateBrowserSessionOperations(): void { state?.operations.invalidate(); }
