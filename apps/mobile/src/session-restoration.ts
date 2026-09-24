import { isValidSessionCredential, type SessionExchangeCredential } from '@hourpaths/client-core';

export type StoredSession = SessionExchangeCredential;

type RestorationDependencies = {
  read(): Promise<string | null>;
  now(): number;
  refreshLeadMs: number;
  refresh(current: StoredSession): Promise<StoredSession>;
  expiryAdvanced(previous: string, next: string): boolean;
  current(): boolean;
  revokeSuperseded(current: StoredSession): Promise<void>;
  activate(current: StoredSession, renewable: boolean): Promise<void>;
  handleFailure(cause: unknown, current: StoredSession): Promise<void>;
  handleUnreadable(): Promise<void>;
};

function decodeStoredSession(raw: string): StoredSession {
  let parsed: unknown;
  try { parsed = JSON.parse(raw); }
  catch { throw { kind: 'local_storage', reason: 'malformed' } as const; }
  if (!isValidSessionCredential(parsed) || !parsed.nextAction) {
    throw { kind: 'local_storage', reason: 'missing_fields' } as const;
  }
  return parsed as StoredSession;
}

export async function restoreStoredSession(dependencies: RestorationDependencies): Promise<void> {
  let raw: string | null | undefined;
  let current: StoredSession | null = null;
  try {
    raw = await dependencies.read();
    if (!raw) return;
    current = decodeStoredSession(raw);
    if (!dependencies.current()) return;
    const now = dependencies.now();
    if (Date.parse(current.expiresAt) <= now) throw { kind: 'expired' } as const;
    let renewable = current.nextAction === 'home';
    if (renewable && Date.parse(current.expiresAt) - now < dependencies.refreshLeadMs) {
      const previousExpiry = current.expiresAt;
      current = await dependencies.refresh(current);
      if (!dependencies.current()) {
        await dependencies.revokeSuperseded(current);
        return;
      }
      renewable = dependencies.expiryAdvanced(previousExpiry, current.expiresAt);
    }
    if (dependencies.current()) await dependencies.activate(current, renewable);
  } catch (cause) {
    if (!dependencies.current()) return;
    if (!current && raw) {
      try { current = decodeStoredSession(raw); }
      catch { await dependencies.handleUnreadable(); return; }
    }
    if (!current) { await dependencies.handleUnreadable(); return; }
    await dependencies.handleFailure(cause, current);
  }
}
