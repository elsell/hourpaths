<script lang="ts">
  import { onMount } from 'svelte';
  import { createSessionApiClient, generatedResponse } from '@hourpaths/api-client';
  import { isSessionFailure, validateSessionMutation, type ClientRuntimeConfig, type SessionFailure } from '@hourpaths/client-core';
  import { createTranslator, type MessageKey, type SupportedLocale, type Translator } from '@hourpaths/i18n';
  import { applicationDestination, applicationSession, applicationSessionExpired, applicationSessionOperations, clearApplicationSession, declineApplicationRecovery, revokeApplicationSession, webSessionFailure, type ApplicationSession } from '$lib/auth';
  import { replaceApplicationLocation } from '$lib/provider-auth';

  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  let session: ApplicationSession | null = null;
  let ready = false;
  let errorKey: MessageKey | null = null;
  let i18n: Translator;
  $: i18n = createTranslator([data.locale]);

  function expireSession() {
    applicationSessionOperations.invalidate();
    clearApplicationSession();
    session = null;
    replaceApplicationLocation('/');
  }

  onMount(() => {
    let expiryTimer: ReturnType<typeof setTimeout> | undefined;
    try {
      session = applicationSession();
      const nextAction = session?.nextAction ?? 'home';
      if (nextAction !== 'duplicate_email_recovery') {
        replaceApplicationLocation(applicationDestination(nextAction));
      } else if (session && applicationSessionExpired(session)) {
        expireSession();
      } else if (session) {
        expiryTimer = setTimeout(expireSession, Math.max(0, Date.parse(session.expiresAt) - Date.now()));
      }
    } catch (cause) {
      const failure: SessionFailure = isSessionFailure(cause)
        ? cause
        : { kind: 'local_storage', reason: 'malformed' };
      const presentation = webSessionFailure(failure);
      if (presentation.discardCredential) {
        applicationSessionOperations.invalidate();
        clearApplicationSession();
        session = null;
      }
      errorKey = presentation.message;
    } finally {
      ready = true;
    }
    return () => { if (expiryTimer !== undefined) clearTimeout(expiryTimer); };
  });

  async function signOut() {
    await revokeApplicationSession(data.config, session);
    session = null;
    replaceApplicationLocation('/');
  }

  async function declineRecovery() {
    const recoverySession = session;
    if (!recoverySession || recoverySession.nextAction !== 'duplicate_email_recovery') return;
    if (applicationSessionExpired(recoverySession)) {
      expireSession();
      return;
    }
    const ticket = applicationSessionOperations.issue();
    let serverDeclined = false;
    try {
      await validateSessionMutation(async () => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => recoverySession.token).declineDuplicateEmailRecovery(),
      ));
      serverDeclined = true;
      const replacement = declineApplicationRecovery(recoverySession, ticket);
      if (!replacement) return;
      session = replacement;
      replaceApplicationLocation('/onboarding');
    } catch (cause) {
      const failure: SessionFailure = isSessionFailure(cause)
        ? cause
        : { kind: 'local_storage', reason: 'malformed' };
      const presentation = webSessionFailure(failure);
      if (!serverDeclined && presentation.discardCredential) {
        expireSession();
      }
      errorKey = presentation.message;
    }
  }
</script>

<main>
  <h1>{i18n.t('duplicateEmailRecovery.heading')}</h1>
  {#if !ready}
    <p>{i18n.t('common.loading')}</p>
  {:else if session}
    <p>{i18n.t('duplicateEmailRecovery.notice')}</p>
    <button type="button" onclick={() => void declineRecovery()}>{i18n.t('duplicateEmailRecovery.decline')}</button>
    <button type="button" onclick={signOut}>{i18n.t('auth.signOut')}</button>
  {/if}
  {#if errorKey}<p role="alert">{i18n.t(errorKey)}</p>{/if}
</main>

<style>main{font:16px system-ui;max-width:42rem;margin:10vh auto;padding:2rem}button{padding:.65rem 1rem;border:0;border-radius:.5rem;background:#111;color:white}</style>
