import { createSessionApiClient, generatedResponse, type OnboardingProfile } from '@hourpaths/api-client';
import { isSessionFailure, validateSessionCredential, validateSessionMutation, type ClientRuntimeConfig, type SessionOperationTicket } from '@hourpaths/client-core';
import { activateApplicationSession, applicationSession, applicationSessionExpired, applicationSessionOperations, beginApplicationSignIn, clearApplicationSession, declineApplicationRecovery, exchangeApplicationSession, revokeApplicationSession, revokeSupersededApplicationSession, webSessionFailure, type ApplicationSession } from '../../../auth';
import { completeProviderSignIn, replaceApplicationLocation } from '../../../provider-auth';
import { deviceOnboardingDefaults } from '../../../onboarding';
import { openWebPolicyLink } from '../../../external-policy-link';
import { EntryFailure, type EntryContext, type EntryReview, type PolicyLink } from '../domain/entry';
import type { EntryService } from '../ports/entry-service';

function required(value: unknown): string { if (typeof value !== 'string' || !value.trim()) throw new EntryFailure('unavailable'); return value; }
function seed(value: unknown): string { if (typeof value !== 'string') throw new EntryFailure('unavailable'); return value; }
function link(value: unknown): string { const url = new URL(required(value)); if (url.protocol !== 'https:' || !url.hostname || url.username || url.password) throw new EntryFailure('unavailable'); return url.href; }
function policy(value: { url: string; version: string }): PolicyLink { return { url: link(value?.url), version: required(value?.version) }; }
export function entryReview(value: OnboardingProfile): EntryReview {
  return { email: seed(value.email), displayName: seed(value.displayName), username: seed(value.usernameSuggestion), token: required(value.policyReviewToken), policies: {
    terms: policy(value.policies?.termsOfService), privacy: policy(value.policies?.privacyPolicy), guidelines: policy(value.policies?.communityGuidelines), support: link(value.policies?.supportUrl),
  } };
}
export function browserEntryService(config: ClientRuntimeConfig, now: () => number = () => Date.now()): EntryService {
  let current: ApplicationSession | null = null, disposed = false;
  let issued: SessionOperationTicket | null = null;
  function same() { try { return !disposed && !!current && applicationSession()?.token === current.token; } catch { return false; } }
  function cancelTicket() { if (issued?.current()) applicationSessionOperations.invalidate(); issued = null; }
  function ticket(): SessionOperationTicket { const own = applicationSessionOperations.issue(); issued = own; return { ...own, current: () => !disposed && own.current() }; }
  function context(): EntryContext {
    if (!current) return { kind: 'entry' };
    return { kind: current.nextAction === 'onboarding' ? 'onboarding' : current.nextAction === 'duplicate_email_recovery' ? 'recovery' : 'home', expiresAt: Date.parse(current.expiresAt) };
  }
  function expire() {
    if (same()) { cancelTicket(); clearApplicationSession(); }
    current = null;
  }
  function authorized(destination?: ApplicationSession['nextAction']) {
    if (!same()) throw new EntryFailure('superseded');
    if (applicationSessionExpired(current!, now())) { expire(); throw new EntryFailure('expired'); }
    if (destination && current!.nextAction !== destination) throw new EntryFailure('superseded');
    return current!;
  }
  function failure(cause: unknown): never {
    if (cause instanceof EntryFailure) throw cause;
    if (isSessionFailure(cause)) {
      const code: string | undefined = cause.kind === 'http' ? cause.code : undefined;
      if (cause.kind === 'http' && cause.code === 'invalid_identity_token') throw new EntryFailure('identity');
      if (cause.kind === 'http' && code === 'username_unavailable') throw new EntryFailure('username');
      if (cause.kind === 'http' && code === 'policy_set_changed') throw new EntryFailure('policy');
      const classified = webSessionFailure(cause);
      if (classified.discardCredential) { expire(); throw new EntryFailure(cause.kind === 'local_storage' ? 'storage' : 'expired'); }
      if (!classified.retryable && cause.kind === 'http') throw new EntryFailure(cause.status === 403 ? 'forbidden' : cause.status === 400 || cause.status === 422 ? 'validation' : 'rejected');
      if (cause.kind === 'http' && cause.status === 429) throw new EntryFailure('rateLimited');
    }
    throw new EntryFailure('unavailable');
  }
  return {
    async restore() {
      if (disposed) throw new EntryFailure('superseded');
      try { current = applicationSession(); if (current && applicationSessionExpired(current, now())) { expire(); throw new EntryFailure('expired'); } return context(); }
      catch (cause) { return failure(cause); }
    },
    async callback() {
      const own = ticket();
      try {
        const identity = await completeProviderSignIn(config.oidcIssuer, config.oidcClientId);
        if (!own.current()) throw new EntryFailure('superseded');
        const next = await exchangeApplicationSession(identity, config, own);
        if (!own.current() || !next) throw new EntryFailure('superseded');
        current = applicationSession(); if (!current || current.nextAction !== next) throw new EntryFailure('unavailable');
        return context();
      } catch (cause) { if (!own.current()) throw new EntryFailure('superseded'); if (isSessionFailure(cause)) return failure(cause); throw cause instanceof EntryFailure ? cause : new EntryFailure('callback'); }
    },
    defaults() {
      try { return deviceOnboardingDefaults(); }
      catch (cause) {
        const timeZoneUnavailable = cause instanceof Error && cause.message === 'device time zone unavailable';
        throw new EntryFailure(timeZoneUnavailable ? 'timeZone' : 'locale');
      }
    },
    async review() {
      const session = authorized('onboarding');
      try {
        const value = await validateSessionCredential<OnboardingProfile>(session, async credential => generatedResponse(await createSessionApiClient(config.apiURL, () => credential.token).onboarding()));
        authorized('onboarding'); if (current!.token !== session.token) throw new EntryFailure('superseded');
        return entryReview(value);
      } catch (cause) { if (!same() || current!.token !== session.token) throw new EntryFailure('superseded'); return failure(cause); }
    },
    async activate(input) {
      const session = authorized('onboarding'), own = ticket();
      try {
        const result = await activateApplicationSession(session, async credential => generatedResponse(await createSessionApiClient(config.apiURL, () => credential.token).activateOnboarding({
          username: input.username, displayName: input.displayName, profileVisibility: input.visibility, timeZone: input.timeZone, firstDayOfWeek: input.firstDayOfWeek,
          policyReviewToken: input.reviewToken, atLeast16: input.age, termsAccepted: input.terms, privacyAcknowledged: input.privacy, communityGuidelinesAccepted: input.guidelines,
        })), own);
        if (!result.adopted || !own.current()) { revokeSupersededApplicationSession(config, result.session); throw new EntryFailure('superseded'); }
        current = result.session; return context();
      } catch (cause) { if (!own.current()) throw new EntryFailure('superseded'); return failure(cause); }
    },
    async decline() {
      const session = authorized('duplicate_email_recovery'), own = ticket();
      try {
        await validateSessionMutation(async () => generatedResponse(await createSessionApiClient(config.apiURL, () => session.token).declineDuplicateEmailRecovery()));
        authorized('duplicate_email_recovery');
        const replacement = declineApplicationRecovery(session, own);
        if (!replacement || !own.current()) throw new EntryFailure('superseded');
        current = replacement; return context();
      } catch (cause) { if (!own.current()) throw new EntryFailure('superseded'); return failure(cause); }
    },
    async begin() { if (disposed) return; try { await beginApplicationSignIn(config); } catch { throw new EntryFailure('signIn'); } },
    async signOut() { const previous = same() ? current : null; cancelTicket(); if (previous) await revokeApplicationSession(config, previous); current = null; },
    current: same, expire,
    navigate(destination) {
      if (disposed) return;
      if (destination !== 'entry') authorized();
      replaceApplicationLocation(destination === 'home' || destination === 'entry' ? '/studio' : destination === 'onboarding' ? '/onboarding' : '/account-recovery');
    },
    openPolicy(url) { if (!disposed) openWebPolicyLink(url, () => { throw new EntryFailure('unavailable'); }); },
    dispose() { disposed = true; cancelTicket(); current = null; },
  };
}
