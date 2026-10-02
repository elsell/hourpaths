export interface Session {
  readonly ownerId?: string;
  readonly token: string;
  readonly expiresAt: number;
  readonly destination: 'home' | 'onboarding' | 'duplicate_email_recovery';
}
export class SessionUnavailable extends Error {
  constructor(readonly retryable: boolean, readonly reason?: 'rejected') { super('session_unavailable'); }
}
