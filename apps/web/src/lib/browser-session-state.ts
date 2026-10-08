import { forgetProviderLinkIntent, forgetProviderRecoveryIntent } from './provider-auth';
import { browserSessionRecords, sessionRecordKey } from './studio/session/adapters/browser-session-record';
import { DurableSessionOperations } from './studio/session/application/session-operations';

let state: ReturnType<typeof createState> | undefined;
function createState() {
  const signInRevisionKey = 'hourpaths_sign_in_revision_v1';
  const records = browserSessionRecords(window.localStorage, window.sessionStorage,
    window.navigator.locks, () => crypto.randomUUID());
  const operations = new DurableSessionOperations(records.coordinator);
  const observedFamilies = new Map<string, string>();
  let ready = false;
  let initializing: Promise<void> | undefined;
  let clearing = false;
  return {
    operations,
    revision: () => records.coordinator.read().revision,
    beginSignIn(): void {
      window.sessionStorage.setItem(signInRevisionKey, records.coordinator.read().revision);
    },
    signInTicket() {
      const revision = window.sessionStorage.getItem(signInRevisionKey);
      window.sessionStorage.removeItem(signInRevisionKey);
      // Missing or superseded intent cannot adopt a provider callback.
      return operations.issue(revision ?? 'missing_sign_in_intent', true);
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
      if (clearing) return null;
      const record = records.coordinator.read();
      if (record.value) {
        const value: unknown = JSON.parse(record.value);
        if (value && typeof value === 'object' && 'token' in value && typeof value.token === 'string') observedFamilies.set(value.token, record.family);
      }
      return record.value;
    },
    async discardOwner(owner: string): Promise<void> {
      // Do not invalidate operations belonging to a replacement account.
      // The shared revision change invalidates tickets for the removed account.
      forgetProviderLinkIntent(owner);
      await records.coordinator.discardOwner(owner);
    },
    async discard(value: string | null, expectedToken?: string): Promise<void> {
      operations.invalidate();
      clearing = true;
      try {
        let previous;
        try { previous = records.coordinator.read(); }
        catch { await records.discardUnreadable(); return; }
        const family = expectedToken ? observedFamilies.get(expectedToken) : previous.family;
        if (family !== undefined && family === previous.family) {
          forgetProviderRecoveryIntent(previous.revision);
          if (previous.value) {
            let prior: unknown;
            try { prior = JSON.parse(previous.value); } catch { prior = null; }
            if (prior && typeof prior === 'object' && 'ownerId' in prior && typeof prior.ownerId === 'string') forgetProviderLinkIntent(prior.ownerId);
          }
          await records.coordinator.discardFamily(family, value);
        }
      }
      finally { clearing = false; }
    },
  };
}
export function browserSessionState() { return state ??= createState(); }

export function invalidateBrowserSessionOperations(): void { state?.operations.invalidate(); }
