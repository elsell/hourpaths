import { createSessionApiClient, generatedResponse } from '@hourpaths/api-client';
import { retainedAccount, readRetainedAccount, classifySessionFailure, declineDuplicateEmailRecovery, exchangeSessionCredential, isSessionFailure, isValidSessionCredential, refreshSessionCredential, sessionFailureFromResponse, type ClientRuntimeConfig, type SessionCredential, type SessionFailure, type SessionNextAction, type SessionOperationTicket, type SessionRefreshResponse } from '@hourpaths/client-core';
import { problemMessageKey, type MessageKey } from '@hourpaths/i18n';
import { browserSessionState, invalidateBrowserSessionOperations } from './browser-session-state';
import { forgetProviderRecoveryIntent, beginProviderSignIn, completeProviderSignIn, type ApplicationDestination } from './provider-auth';

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
    if (readRetainedAccount(value)) return null;
  } catch { /* Invalid records share the same destructive recovery boundary. */ }
  discardLocalSession(storage);
  throw unreadableLocalSession();
}
export function retainedApplicationAccount(storage: Pick<Storage, 'getItem'> = durableReader): string | null {
  try { return readRetainedAccount(JSON.parse(storage.getItem(sessionKey) ?? 'null'))?.ownerId ?? null; }
  catch { return null; }
}
const durableReader = {
  getItem: (_key: string) => browserSessionState().read(),
  removeItem: (_key: string) => { void clearApplicationSession(); },
};
export async function initializeApplicationSession(): Promise<void> {
  try { await browserSessionState().initialize(); }
  catch { throw unreadableLocalSession(); }
}
export async function pauseApplicationSession(owner: string, storage?: SessionStorageWriter, expectedToken?: string): Promise<void> {
  applicationSessionOperations.invalidate();
  const value = JSON.stringify(retainedAccount(owner));
  if (storage) storage.setItem(sessionKey, value);
  else await browserSessionState().discard(value, expectedToken);
}
export function applicationSession(): ApplicationSession | null { return readApplicationSession(durableReader); }
export function applicationSessionExpired(session: ApplicationSession, now = Date.now()): boolean {
  return Date.parse(session.expiresAt) <= now;
}
export async function clearApplicationSession(storage?: SessionStorageRemover, expectedToken?: string): Promise<void> {
  if (storage) discardLocalSession(storage);
  else {
    try { await browserSessionState().discard(null, expectedToken); }
    catch { /* Unreadable storage cannot grant access. */ }
  }
}
export const applicationSessionOperations = {
  issue: () => browserSessionState().operations.issue(),
  signIn: () => browserSessionState().signInTicket(),
  recovery: (revision: string) => browserSessionState().operations.issue(revision, true),
  invalidate: invalidateBrowserSessionOperations,
};
export async function persistOwnedApplicationSession(
  session: ApplicationSession,
  ticket: SessionOperationTicket,
  storage?: SessionStorageWriter,
): Promise<boolean> {
  if (!ticket.current()) return false;
  const value = JSON.stringify(session);
  if (storage) { storage.setItem(sessionKey, value); return true; }
  if (!('persist' in ticket) || typeof ticket.persist !== 'function') return false;
  return ticket.persist(value);
}

export async function activateApplicationSession(
  current: ApplicationSession,
  request: (current: ApplicationSession) => Promise<SessionRefreshResponse>,
  ticket: SessionOperationTicket,
  storage?: SessionStorageWriter,
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
    return { session: homeSession, adopted: await persistOwnedApplicationSession(homeSession, ticket, storage) };
  } catch {
    throw unreadableLocalSession();
  }
}

export async function declineApplicationRecovery(
  session: ApplicationSession,
  ticket: SessionOperationTicket,
  storage?: SessionStorageWriter,
): Promise<ApplicationSession | null> {
  const replacement = declineDuplicateEmailRecovery(session);
  if (!storage && applicationSession()?.token === session.token) forgetProviderRecoveryIntent(browserSessionState().revision());
  try {
    return await persistOwnedApplicationSession(replacement, ticket, storage) ? replacement : null;
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
    async (replacement) => { await persistOwnedApplicationSession(replacement, ticket); },
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
  return browserSessionState().refreshExclusive(async () => {
    if (!ticket.current() || applicationSession()?.token !== current.token) throw { kind: 'network' } satisfies SessionFailure;
    return refreshSessionCredential(
    current,
    async (credential) => generatedResponse(await createSessionApiClient(config.apiURL, () => credential.token).refresh()),
    async (replacement) => { await persistOwnedApplicationSession(replacement, ticket); },
    );
  });
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
  storage?: SessionStorageRemover,
  revoke: RemoteSessionRevoker = revokeRemoteApplicationSession,
): Promise<void> {
	applicationSessionOperations.invalidate();
	await clearApplicationSession(storage, session?.token);
	if (session) {
		try { void revoke(config, session.token).catch(() => {}); }
    catch { /* Local credential disposal must not depend on network availability. */ }
  }
}

export async function beginApplicationSignIn(config: ClientRuntimeConfig): Promise<void> {
  await initializeApplicationSession();
  browserSessionState().beginSignIn();
  await beginProviderSignIn(config.oidcIssuer, config.oidcClientId);
}

export async function completeApplicationSignIn(config: ClientRuntimeConfig): Promise<SessionNextAction | null> {
  await initializeApplicationSession();
  const ticket = applicationSessionOperations.signIn();
  const identityToken = await completeProviderSignIn(config.oidcIssuer, config.oidcClientId);
  if (!ticket.current()) return null;
  return exchangeApplicationSession(identityToken, config, ticket);
}
