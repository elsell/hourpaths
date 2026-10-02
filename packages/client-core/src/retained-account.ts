/** Credential-free marker kept only in the client's existing session storage.
 * It permits local Stop and sign-in recovery, never an authenticated request. */
export interface RetainedAccount { state: 'sign_in_required'; ownerId: string }
export function retainedAccount(ownerId: string): RetainedAccount {
  if (typeof ownerId !== 'string' || !ownerId || ownerId.trim() !== ownerId || ownerId.length > 128) throw new Error('retained_account_invalid');
  return { state: 'sign_in_required', ownerId };
}
export function readRetainedAccount(value: unknown): RetainedAccount | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if (Object.keys(record).length !== 2 || record.state !== 'sign_in_required' || typeof record.ownerId !== 'string') return null;
  try { return retainedAccount(record.ownerId); } catch { return null; }
}
