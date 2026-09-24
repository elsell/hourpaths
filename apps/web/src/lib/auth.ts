import { createSessionApiClient, generatedResponse } from '@hourpaths/api-client';
import { classifySessionFailure, createSessionOperationOwner, declineDuplicateEmailRecovery, exchangeSessionCredential, isSessionFailure, isValidSessionCredential, refreshSessionCredential, sessionFailureFromResponse, type ClientRuntimeConfig, type SessionCredential, type SessionFailure, type SessionNextAction, type SessionOperationTicket, type SessionRefreshResponse } from '@hourpaths/client-core';
import { problemMessageKey, type MessageKey } from '@hourpaths/i18n';
import { beginProviderSignIn, completeProviderSignIn, type ApplicationDestination } from './provider-auth';

const sessionKey = 'hourpaths_application_session';
export type ApplicationSession = SessionCredential;
type SessionStorageReader = Pick<Storage, 'getItem' | 'removeItem'>;
type SessionStorageRemover = Pick<Storage, 'removeItem'>;
type SessionStorageWriter = Pick<Storage, 'setItem'>;
type RemoteSessionRevoker = (config: ClientRuntimeConfig, token: string) => Promise<void>;

function unreadableLocalSession(): SessionFailure {
  return { kind: 'local_storage', reason: 'malformed' };
}
function discardLocalSession(storage: SessionStorageRemover): void {
  try { storage.removeItem(sessionKey); }
  catch { /* An inaccessible browser store remains unreadable; recovery must stay bounded. */ }
}

export function readApplicationSession(storage: SessionStorageReader): ApplicationSession | null {
  let raw: string | null;
  try { raw = storage.getItem(sessionKey); }
  catch { throw unreadableLocalSession(); }
  if (!raw) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (isValidSessionCredential(value)) return value;
  } catch { /* Invalid records share the same destructive recovery boundary. */ }
  discardLocalSession(storage);
  throw unreadableLocalSession();
}
export function applicationSession(): ApplicationSession | null { return readApplicationSession(window.sessionStorage); }
export function applicationSessionExpired(session: ApplicationSession, now = Date.now()): boolean {
  return Date.parse(session.expiresAt) <= now;
}
export function clearApplicationSession(storage: SessionStorageRemover = window.sessionStorage): void {
  discardLocalSession(storage);
}
export const applicationSessionOperations = createSessionOperationOwner();
export function persistOwnedApplicationSession(
  session: ApplicationSession,
  ticket: SessionOperationTicket,
  storage: SessionStorageWriter = window.sessionStorage,
): boolean {
  if (!ticket.current()) return false;
  storage.setItem(sessionKey, JSON.stringify(session));
  return true;
}

export async function activateApplicationSession(
  current: ApplicationSession,
  request: (current: ApplicationSession) => Promise<SessionRefreshResponse>,
  ticket: SessionOperationTicket,
  storage: SessionStorageWriter = window.sessionStorage,
): Promise<{ session: ApplicationSession & { nextAction: 'home' }; adopted: boolean }> {
  let response: SessionRefreshResponse;
  try { response = await request(current); }
  catch (cause) {
    if (isSessionFailure(cause)) throw cause;
    throw { kind: 'network' } satisfies SessionFailure;
  }
  if (!response.ok) {
    const code = response.problem && typeof response.problem === 'object' && 'code' in response.problem
      && typeof response.problem.code === 'string' ? response.problem.code : undefined;
    throw code
      ? { kind: 'http', status: response.status, code }
      : sessionFailureFromResponse(response.status, response.problem);
  }
  let body: unknown;
  try { body = await response.json(); }
  catch { throw sessionFailureFromResponse(502); }
  const replacement = (body as { data?: unknown } | null)?.data;
  if (!isValidSessionCredential(replacement) || replacement.nextAction !== 'home') {
    throw sessionFailureFromResponse(502);
  }
  const homeSession = replacement as ApplicationSession & { nextAction: 'home' };
  if (!ticket.current()) return { session: homeSession, adopted: false };
  try {
    return { session: homeSession, adopted: persistOwnedApplicationSession(homeSession, ticket, storage) };
  } catch {
    throw unreadableLocalSession();
  }
}

export function declineApplicationRecovery(
  session: ApplicationSession,
  ticket: SessionOperationTicket,
  storage: SessionStorageWriter = window.sessionStorage,
): ApplicationSession | null {
  const replacement = declineDuplicateEmailRecovery(session);
  try {
    return persistOwnedApplicationSession(replacement, ticket, storage) ? replacement : null;
  } catch {
    throw unreadableLocalSession();
  }
}

export function webSessionFailure(failure: SessionFailure): {
  accessState: ReturnType<typeof classifySessionFailure>['state'];
  discardCredential: boolean;
  retryable: boolean;
  message: MessageKey;
} {
  const decision = classifySessionFailure(failure);
  const message = decision.retryable
    ? 'errors.temporarilyUnavailable'
    : decision.discardCredential
      ? failure.kind === 'local_storage' ? 'errors.localSessionUnreadable' : 'errors.sessionExpired'
      : problemMessageKey(failure.kind === 'http' ? failure.code : undefined);
  return { accessState: decision.state, discardCredential: decision.discardCredential, retryable: decision.retryable, message };
}
export function exchangeSessionFailureMessage(failure: SessionFailure): MessageKey {
  if (failure.kind === 'http' && failure.code) return problemMessageKey(failure.code);
  return webSessionFailure(failure).message;
}
export function applicationDestination(nextAction: SessionNextAction): ApplicationDestination {
  switch (nextAction) {
    case 'home': return '/';
    case 'onboarding': return '/onboarding';
    case 'duplicate_email_recovery': return '/account-recovery';
  }
}
export async function exchangeApplicationSession(
  identityToken: string,
  config: ClientRuntimeConfig,
  ticket: SessionOperationTicket,
): Promise<SessionNextAction | null> {
  const credential = await exchangeSessionCredential(
    async () => generatedResponse(await createSessionApiClient(config.apiURL, () => null).exchange(identityToken)),
    async (replacement) => { persistOwnedApplicationSession(replacement, ticket); },
  );
  if (ticket.current()) return credential.nextAction;
  revokeSupersededApplicationSession(config, credential);
  return null;
}
export async function refreshApplicationSession(
  config: ClientRuntimeConfig,
  current: ApplicationSession,
  ticket: SessionOperationTicket,
): Promise<ApplicationSession> {
  return refreshSessionCredential(
    current,
    async (credential) => generatedResponse(await createSessionApiClient(config.apiURL, () => credential.token).refresh()),
    async (replacement) => { persistOwnedApplicationSession(replacement, ticket); },
  );
}
async function revokeRemoteApplicationSession(config: ClientRuntimeConfig, token: string): Promise<void> {
  await createSessionApiClient(config.apiURL, () => token).revoke();
}
export function revokeSupersededApplicationSession(
  config: ClientRuntimeConfig,
  session: ApplicationSession,
  revoke: RemoteSessionRevoker = revokeRemoteApplicationSession,
): void {
  try { void revoke(config, session.token).catch(() => {}); }
  catch { /* Superseded local state remains authoritative. */ }
}
export async function revokeApplicationSession(
  config: ClientRuntimeConfig,
  session: ApplicationSession | null,
  storage: SessionStorageRemover = window.sessionStorage,
  revoke: RemoteSessionRevoker = revokeRemoteApplicationSession,
): Promise<void> {
	applicationSessionOperations.invalidate();
	clearApplicationSession(storage);
	if (session) {
		try { void revoke(config, session.token).catch(() => {}); }
    catch { /* Local credential disposal must not depend on network availability. */ }
  }
}

export async function beginApplicationSignIn(config: ClientRuntimeConfig): Promise<void> {
  await beginProviderSignIn(config.oidcIssuer, config.oidcClientId);
}

export async function completeApplicationSignIn(config: ClientRuntimeConfig): Promise<SessionNextAction | null> {
  const identityToken = await completeProviderSignIn(config.oidcIssuer, config.oidcClientId);
  const ticket = applicationSessionOperations.issue();
  return exchangeApplicationSession(identityToken, config, ticket);
}
