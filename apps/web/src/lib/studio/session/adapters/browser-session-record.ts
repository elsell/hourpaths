import { SessionRecordCoordinator } from '../application/session-record';
import type { SessionRecord, SessionRecordPort } from '../ports/session-record';

export const sessionRecordKey = 'hourpaths_session_record_v1';
const legacyKey = 'hourpaths_application_session';
const lockName = 'hourpaths_session_record_v1';
type StorageAccess = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;
export interface SessionLocks {
  request<T>(name: string, operation: () => T | PromiseLike<T>): Promise<T>;
}

/** Only opaque application credentials/retained references enter this adapter.
 * Provider credentials and authorization state stay in their existing adapter. */
export function browserSessionRecords(storage: StorageAccess, legacy: StorageAccess,
  locks: SessionLocks, revision: () => string) {
  const port: SessionRecordPort = {
    read() {
      const raw = storage.getItem(sessionRecordKey);
      if (raw === null) return null;
      const parsed: unknown = JSON.parse(raw);
      if (!parsed || typeof parsed !== 'object' || !('version' in parsed) || parsed.version !== 1 ||
        !('revision' in parsed) || typeof parsed.revision !== 'string' || !parsed.revision ||
        !('value' in parsed) || parsed.value !== null && typeof parsed.value !== 'string') {
        throw new Error('session_record_unreadable');
      }
      return { revision: parsed.revision, value: parsed.value };
    },
    write(record) { storage.setItem(sessionRecordKey, JSON.stringify({ version: 1, ...record })); },
    exclusive: operation => locks.request(lockName, operation),
  };
  const coordinator = new SessionRecordCoordinator(port, revision);
  return {
    coordinator,
    discardUnreadable(): Promise<void> {
      return port.exclusive(() => {
        try { port.read(); return; }
        catch { port.write({ revision: revision(), value: null }); }
      });
    },
    /** A durable logout tombstone always wins over an older per-tab session. */
    initialize(): Promise<SessionRecord> {
      return port.exclusive(() => {
        let current = port.read();
        if (!current) {
          current = { revision: revision(), value: legacy.getItem(legacyKey) };
          port.write(current);
        }
        legacy.removeItem(legacyKey);
        return current;
      });
    },
  };
}
