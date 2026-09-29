export interface Session {
  readonly token: string;
  readonly expiresAt: number;
  readonly destination: 'home' | 'onboarding' | 'duplicate_email_recovery';
}
export class SessionUnavailable extends Error {
  constructor(readonly retryable: boolean) { super('session_unavailable'); }
}
