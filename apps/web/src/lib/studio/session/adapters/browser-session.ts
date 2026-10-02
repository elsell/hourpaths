import { createSessionApiClient, generatedResponse } from '@hourpaths/api-client';
import { classifySessionFailure, isSessionFailure, refreshSessionCredential, type SessionCredential } from '@hourpaths/client-core';
import { pauseApplicationSession, retainedApplicationAccount, applicationSession, applicationSessionOperations, clearApplicationSession, persistOwnedApplicationSession } from '../../../auth';
import { SessionUnavailable, type Session } from '../domain/session';
import type { SessionService, SessionStore } from '../ports/session-store';

function fromCredential(value: SessionCredential): Session {
  return { token: value.token, expiresAt: Date.parse(value.expiresAt), destination: value.nextAction ?? 'home', ...(value.ownerId ? { ownerId: value.ownerId } : {}) };
}
function toCredential(value: Session): SessionCredential {
  return { token: value.token, expiresAt: new Date(value.expiresAt).toISOString(), nextAction: value.destination, ...(value.ownerId ? { ownerId: value.ownerId } : {}) };
}
export function browserSessionStore(): SessionStore {
  return {
    read() {
      try { const value = applicationSession(); return value ? fromCredential(value) : null; }
      catch { return null; }
    },
    write(value) {
      const ticket = applicationSessionOperations.issue();
      persistOwnedApplicationSession(toCredential(value), ticket);
    },
    clear() { applicationSessionOperations.invalidate(); clearApplicationSession(); },
    pause: owner => pauseApplicationSession(owner),
    retainedOwner: () => retainedApplicationAccount(),
  };
}
export function apiSessionService(apiURL: string): SessionService {
  return {
    async refresh(current) {
      try {
        const value = await refreshSessionCredential(toCredential(current),
          async credential => generatedResponse(await createSessionApiClient(apiURL, () => credential.token).refresh()),
          async () => undefined);
        return fromCredential(value);
      } catch (error) {
        // Only expiry, rejection, or unreadable storage disposes a valid credential.
        throw new SessionUnavailable(!isSessionFailure(error) || !classifySessionFailure(error).discardCredential, isSessionFailure(error) && error.kind === 'http' && error.status === 401 ? 'rejected' : undefined);
      }
    },
    async revoke(current) { await createSessionApiClient(apiURL, () => current.token).revoke(); },
  };
}
