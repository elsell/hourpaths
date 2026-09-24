import type { SessionOperationTicket } from '@hourpaths/client-core';

export type OnboardingActivationAttempt = Readonly<{
  sessionToken: string;
  ticket: SessionOperationTicket;
}>;

export function createOnboardingActivationOwner() {
  let active: OnboardingActivationAttempt | null = null;
  return {
    begin(sessionToken: string, ticket: SessionOperationTicket): OnboardingActivationAttempt {
      const attempt = Object.freeze({ sessionToken, ticket });
      active = attempt;
      return attempt;
    },
    blocked(): boolean {
      return active !== null && active.ticket.current();
    },
    ownedBy(ticket: SessionOperationTicket): boolean {
      return active?.ticket === ticket && ticket.current();
    },
    invalidate(): void {
      active = null;
    },
    owns(attempt: OnboardingActivationAttempt): boolean {
      return active === attempt && attempt.ticket.current();
    },
    release(attempt: OnboardingActivationAttempt): void {
      if (active === attempt) active = null;
    },
  };
}
