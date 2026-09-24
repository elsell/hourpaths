import {
  isSessionFailure,
  isValidSessionCredential,
  sessionFailureFromResponse,
  type SessionExchangeCredential,
  type SessionFailure,
} from '@hourpaths/client-core';

export type MobileOnboardingActivationResponse = {
  ok: boolean;
  status: number;
  problem?: unknown;
  json(): Promise<unknown>;
};

export type MobileOnboardingActivationOutcome<Profile> =
  | { kind: 'activated' }
  | { kind: 'policy_set_changed'; profile: Profile }
  | { kind: 'username_unavailable' }
  | { kind: 'adoption_failed' }
  | { kind: 'superseded' };

type MobileOnboardingActivationDependencies<Profile> = {
  request(): Promise<MobileOnboardingActivationResponse>;
  current(): boolean;
  persist(credential: SessionExchangeCredential): Promise<void>;
  adopt(credential: SessionExchangeCredential): Promise<void>;
  handleAdoptionFailure(cause: unknown, credential: SessionExchangeCredential): Promise<boolean>;
  refreshReview(): Promise<Profile>;
  revokeSuperseded(credential: SessionExchangeCredential): Promise<void>;
};

function activationProblemCode(problem: unknown): string | undefined {
  if (!problem || typeof problem !== 'object' || !('code' in problem)) return undefined;
  return typeof problem.code === 'string' ? problem.code : undefined;
}

export async function completeMobileOnboardingActivation<Profile>(
  dependencies: MobileOnboardingActivationDependencies<Profile>,
): Promise<MobileOnboardingActivationOutcome<Profile>> {
  let response: MobileOnboardingActivationResponse;
  try { response = await dependencies.request(); }
  catch (cause) {
    if (cause && typeof cause === 'object' && 'kind' in cause) throw cause;
    throw { kind: 'network' } satisfies SessionFailure;
  }

  if (!response.ok) {
    const code = activationProblemCode(response.problem);
    if (code === 'policy_set_changed') {
      if (!dependencies.current()) return { kind: 'superseded' };
      const profile = await dependencies.refreshReview();
      return dependencies.current()
        ? { kind: 'policy_set_changed', profile }
        : { kind: 'superseded' };
    }
    if (code === 'username_unavailable') return { kind: 'username_unavailable' };
    throw sessionFailureFromResponse(response.status, response.problem);
  }

  let body: unknown;
  try { body = await response.json(); }
  catch { throw sessionFailureFromResponse(502); }
  const credential = (body as { data?: unknown } | null)?.data;
  if (!isValidSessionCredential(credential) || credential.nextAction !== 'home') {
    throw sessionFailureFromResponse(502);
  }
  const activeCredential: SessionExchangeCredential = { ...credential, nextAction: 'home' };
  try { await dependencies.persist(activeCredential); }
  catch (cause) {
    try { await dependencies.revokeSuperseded(activeCredential); }
    catch { /* Credential persistence failed; local disposal remains authoritative. */ }
    if (isSessionFailure(cause)) throw cause;
    throw { kind: 'local_storage', reason: 'malformed' } satisfies SessionFailure;
  }
  if (!dependencies.current()) {
    await dependencies.revokeSuperseded(activeCredential);
    return { kind: 'superseded' };
  }
  try { await dependencies.adopt(activeCredential); }
  catch (cause) {
    if (!dependencies.current()) {
      await dependencies.revokeSuperseded(activeCredential);
      return { kind: 'superseded' };
    }
    let handled: boolean;
    try { handled = await dependencies.handleAdoptionFailure(cause, activeCredential); }
    catch (handlerCause) {
      try { await dependencies.revokeSuperseded(activeCredential); }
      catch { /* The handler failure remains authoritative. */ }
      throw handlerCause;
    }
    if (!handled) {
      await dependencies.revokeSuperseded(activeCredential);
      return { kind: 'superseded' };
    }
    return { kind: 'adoption_failed' };
  }
  if (!dependencies.current()) {
    await dependencies.revokeSuperseded(activeCredential);
    return { kind: 'superseded' };
  }
  return { kind: 'activated' };
}
