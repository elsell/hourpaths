<script lang="ts">
  import { onMount } from 'svelte';
  import { createSessionApiClient, generatedResponse, type OnboardingProfile } from '@hourpaths/api-client';
  import { isSessionFailure, validateSessionCredential, type ClientRuntimeConfig, type SessionFailure } from '@hourpaths/client-core';
  import { createTranslator, type MessageKey, type SupportedLocale, type Translator } from '@hourpaths/i18n';
  import { activateApplicationSession, applicationDestination, applicationSession, applicationSessionExpired, applicationSessionOperations, clearApplicationSession, revokeApplicationSession, revokeSupersededApplicationSession, webSessionFailure, type ApplicationSession } from '$lib/auth';
  import { buildOnboardingActivationInput, deviceOnboardingDefaults } from '$lib/onboarding';
  import { openWebPolicyLink } from '$lib/external-policy-link';
  import { replaceApplicationLocation } from '$lib/provider-auth';

  type PolicySet = OnboardingProfile['policies'];
  type OnboardingFailure = SessionFailure | { kind: 'http'; status: number; code: string };

  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  let session: ApplicationSession | null = null;
  let email = '';
  let displayName = '';
  let username = '';
  let profileVisibility: 'public' | 'private' | '' = '';
  let timeZone = '';
  let firstDayOfWeek = 1;
  let policyReviewToken = '';
  let policies: PolicySet | null = null;
  let atLeast16 = false;
  let termsAccepted = false;
  let privacyAcknowledged = false;
  let communityGuidelinesAccepted = false;
  let ready = false;
  let submitting = false;
  let errorKey: MessageKey | null = null;
  let onboardingErrorKey: 'onboarding.usernameUnavailable' | 'onboarding.policySetChanged' | null = null;
  let i18n: Translator;
  let weekdayOptions: Array<{ value: number; label: string }> = [];
  $: i18n = createTranslator([data.locale]);
  $: weekdayOptions = [1, 2, 3, 4, 5, 6, 7].map((value, index) => ({
    value,
    label: new Intl.DateTimeFormat(data.locale, { weekday: 'long', timeZone: 'UTC' })
      .format(new Date(Date.UTC(2024, 0, index + 1))),
  }));

  function disposeSession() {
    applicationSessionOperations.invalidate();
    clearApplicationSession();
    session = null;
  }

  async function loadOnboardingReview(current: ApplicationSession, seedEditable: boolean) {
    const ticket = applicationSessionOperations.issue();
    const profile = await validateSessionCredential<OnboardingProfile>(current, async (credential) => generatedResponse(
      await createSessionApiClient(data.config.apiURL, () => credential.token).onboarding(),
    ));
    if (!ticket.current()) return;
    email = profile.email;
    policies = profile.policies;
    policyReviewToken = profile.policyReviewToken;
    termsAccepted = false;
    privacyAcknowledged = false;
    communityGuidelinesAccepted = false;
    if (seedEditable) {
      displayName = profile.displayName;
      username = profile.usernameSuggestion;
    }
  }

  onMount(async () => {
    try {
      ({ timeZone, firstDayOfWeek } = deviceOnboardingDefaults());
      session = applicationSession();
      if (!session) { replaceApplicationLocation('/'); return; }
      const nextAction = session.nextAction ?? 'home';
      if (nextAction !== 'onboarding') { replaceApplicationLocation(applicationDestination(nextAction)); return; }
      if (applicationSessionExpired(session)) { disposeSession(); replaceApplicationLocation('/'); return; }
      await loadOnboardingReview(session, true);
    } catch (cause) {
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      if (presentation.discardCredential) disposeSession();
      errorKey = cause instanceof Error && cause.message === 'device time zone unavailable'
        ? 'onboarding.timeZoneUnavailable'
        : cause instanceof Error && cause.message === 'device locale unavailable'
          ? 'onboarding.localeUnavailable'
        : presentation.message;
    } finally { ready = true; }
  });

  async function confirmOnboarding(event: SubmitEvent) {
    event.preventDefault();
    const current = session;
    if (!current || !policies || submitting) return;
    if (applicationSessionExpired(current)) { disposeSession(); replaceApplicationLocation('/'); return; }
    let activationInput;
    try {
      activationInput = buildOnboardingActivationInput({
        username, displayName, profileVisibility, timeZone, firstDayOfWeek, policyReviewToken,
        atLeast16, termsAccepted, privacyAcknowledged, communityGuidelinesAccepted,
      });
    } catch {
      errorKey = 'errors.validationFailed';
      return;
    }
    const ticket = applicationSessionOperations.issue();
    submitting = true;
    errorKey = null;
    onboardingErrorKey = null;
    try {
      const result = await activateApplicationSession(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activateOnboarding(activationInput),
      ), ticket);
      if (!result.adopted) {
        revokeSupersededApplicationSession(data.config, result.session);
        return;
      }
      session = result.session;
      replaceApplicationLocation('/');
    } catch (cause) {
      const failure: OnboardingFailure = isSessionFailure(cause)
        ? cause as OnboardingFailure
        : { kind: 'network' };
      if (!ticket.current()) return;
      if (failure.kind === 'http' && failure.code === 'policy_set_changed') {
        onboardingErrorKey = 'onboarding.policySetChanged';
        submitting = false;
        try { await loadOnboardingReview(current, false); }
        catch (refreshCause) {
          const refreshFailure: SessionFailure = isSessionFailure(refreshCause) ? refreshCause : { kind: 'network' };
          const presentation = webSessionFailure(refreshFailure);
          if (presentation.discardCredential) disposeSession();
          errorKey = presentation.message;
        }
      } else if (failure.kind === 'http' && failure.code === 'username_unavailable') {
        onboardingErrorKey = 'onboarding.usernameUnavailable';
      } else {
        const presentation = webSessionFailure(failure as SessionFailure);
        if (presentation.discardCredential) disposeSession();
        errorKey = presentation.message;
      }
    } finally {
      if (ticket.current()) submitting = false;
    }
  }

  async function signOut() {
    await revokeApplicationSession(data.config, session);
    session = null;
    replaceApplicationLocation('/');
  }

  function openPolicyLink(url: string) {
    errorKey = null;
    openWebPolicyLink(url, () => { errorKey = 'errors.temporarilyUnavailable'; });
  }
</script>

<main>
  <h1>{i18n.t('onboarding.heading')}</h1>
  {#if !ready}<p>{i18n.t('common.loading')}</p>
  {:else if session && policies}
    <form onsubmit={confirmOnboarding}>
      <label>{i18n.t('onboarding.privateEmailLabel')}<input type="email" value={email} readonly /></label>
      <p>{i18n.t('onboarding.privateEmailNotice')}</p>
      <label>{i18n.t('onboarding.displayNameLabel')}<input bind:value={displayName} autocomplete="name" required onblur={() => displayName = displayName.trim()} /></label>
      <label>{i18n.t('onboarding.usernameLabel')}<input bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" minlength="3" maxlength="64" pattern="[A-Za-z0-9_.]+" required aria-invalid={onboardingErrorKey === 'onboarding.usernameUnavailable'} aria-describedby="username-notice username-error" oninput={() => onboardingErrorKey = null} onblur={() => username = username.trim()} /></label>
      <p id="username-notice">{i18n.t('onboarding.usernameNotice')}</p>
      {#if onboardingErrorKey === 'onboarding.usernameUnavailable'}<p id="username-error" role="alert">{i18n.t(onboardingErrorKey)}</p>{/if}
      <label>{i18n.t('onboarding.visibilityLabel')}<select bind:value={profileVisibility} required><option value="" disabled>{i18n.t('onboarding.visibilityChoose')}</option><option value="public">{i18n.t('onboarding.visibilityPublic')}</option><option value="private">{i18n.t('onboarding.visibilityPrivate')}</option></select></label>
      <label>{i18n.t('onboarding.weekStartLabel')}<select bind:value={firstDayOfWeek}>{#each weekdayOptions as day}<option value={day.value}>{day.label}</option>{/each}</select></label>
      <fieldset>
        <legend>{i18n.t('onboarding.heading')}</legend>
        <label><input type="checkbox" bind:checked={atLeast16} required />{i18n.t('onboarding.ageAttestation')}</label>
        <label><input type="checkbox" bind:checked={termsAccepted} required />{i18n.t('onboarding.termsAcceptance')} <button type="button" onclick={() => openPolicyLink(policies!.termsOfService.url)}>{i18n.t('onboarding.termsLink', { version: policies.termsOfService.version })}</button></label>
        <label><input type="checkbox" bind:checked={privacyAcknowledged} required />{i18n.t('onboarding.privacyAcknowledgement')} <button type="button" onclick={() => openPolicyLink(policies!.privacyPolicy.url)}>{i18n.t('onboarding.privacyLink', { version: policies.privacyPolicy.version })}</button></label>
        <label><input type="checkbox" bind:checked={communityGuidelinesAccepted} required />{i18n.t('onboarding.guidelinesAcceptance')} <button type="button" onclick={() => openPolicyLink(policies!.communityGuidelines.url)}>{i18n.t('onboarding.guidelinesLink', { version: policies.communityGuidelines.version })}</button></label>
      </fieldset>
      <p><button type="button" onclick={() => openPolicyLink(policies!.supportUrl)}>{i18n.t('onboarding.supportLink')}</button></p>
      <button type="submit" disabled={submitting}>{submitting ? i18n.t('onboarding.confirming') : i18n.t('onboarding.confirm')}</button>
      <button type="button" onclick={signOut}>{i18n.t('auth.signOut')}</button>
    </form>
  {/if}
  {#if onboardingErrorKey === 'onboarding.policySetChanged'}<p role="alert">{i18n.t(onboardingErrorKey)}</p>{/if}
  {#if errorKey}<p role="alert">{i18n.t(errorKey)}</p>{/if}
</main>

<style>
  main{font:16px system-ui;max-width:42rem;margin:6vh auto;padding:2rem}form{display:grid;gap:1rem}label{display:grid;gap:.4rem}fieldset label{grid-template-columns:auto 1fr;align-items:start}input,select,button{font:inherit;padding:.65rem}button{border:0;border-radius:.5rem;background:#111;color:white}button[disabled]{opacity:.6}
</style>
