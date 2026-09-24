<script lang="ts">
  import { onMount } from 'svelte';
  import { isSessionFailure, type ClientRuntimeConfig } from '@hourpaths/client-core';
  import { createTranslator, type MessageKey, type SupportedLocale, type Translator } from '@hourpaths/i18n';
  import { applicationDestination, completeApplicationSignIn, exchangeSessionFailureMessage } from '$lib/auth';
  import { replaceApplicationLocation } from '$lib/provider-auth';

  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  let errorKey: MessageKey | null = null;
  let i18n: Translator;
  $: i18n = createTranslator([data.locale]);

  onMount(async () => {
    try {
      const nextAction = await completeApplicationSignIn(data.config);
      if (nextAction) replaceApplicationLocation(applicationDestination(nextAction));
      else errorKey = 'errors.callbackFailed';
    }
    catch (cause) {
      if (!isSessionFailure(cause)) errorKey = 'errors.callbackFailed';
      else errorKey = exchangeSessionFailureMessage(cause);
    }
  });
</script>

{#if errorKey}<p role="alert">{i18n.t(errorKey)}</p>{:else}<p>{i18n.t('auth.signingIn')}</p>{/if}
