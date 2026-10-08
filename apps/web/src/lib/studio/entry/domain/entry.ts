export type EntryPhase = 'entry' | 'onboarding' | 'recovery' | 'home';
export interface EntryContext { kind: EntryPhase; expiresAt?: number }
export type EntryError = 'recovery' | 'unavailable' | 'expired' | 'storage' | 'username' | 'policy' | 'timeZone' | 'locale' | 'validation' | 'signIn' | 'callback' | 'superseded' | 'rejected' | 'forbidden' | 'identity' | 'rateLimited';
export class EntryFailure extends Error { constructor(readonly kind: EntryError) { super(kind); } }
export interface PolicyLink { url: string; version: string }
export interface EntryReview { email: string; displayName: string; username: string; token: string; policies: { terms: PolicyLink; privacy: PolicyLink; guidelines: PolicyLink; support: string } }
export interface EntryFields { username: string; displayName: string; visibility: 'public' | 'private' | ''; timeZone: string; firstDayOfWeek: number; age: boolean; terms: boolean; privacy: boolean; guidelines: boolean }
export interface EntryActivation extends EntryFields { visibility: 'public' | 'private'; reviewToken: string }
export interface EntryState { phase: EntryPhase | 'loading'; busy: boolean; error?: EntryError; review?: EntryReview; defaults?: { timeZone: string; firstDayOfWeek: number } }
export function activation(fields: EntryFields, reviewToken: string): EntryActivation {
  const username = fields.username.trim(), displayName = fields.displayName.trim();
  if (!/^[A-Za-z0-9_.]{3,64}$/.test(username) || !displayName || !fields.visibility || !fields.timeZone || !reviewToken || !Number.isInteger(fields.firstDayOfWeek) || fields.firstDayOfWeek < 1 || fields.firstDayOfWeek > 7 || !fields.age || !fields.terms || !fields.privacy || !fields.guidelines) throw new EntryFailure('validation');
  return { ...fields, username, displayName, visibility: fields.visibility, reviewToken };
}
