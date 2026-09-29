<script lang="ts">
  import { onMount } from 'svelte';
  import type { ClientRuntimeConfig } from '@hourpaths/client-core';
  import { createTranslator, type SupportedLocale } from '@hourpaths/i18n';
  import { completeProviderSignOut } from '$lib/provider-auth';
  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  const i18n = createTranslator([data.locale]);
  let failed = false;
  onMount(() => {
    void completeProviderSignOut(data.config.oidcIssuer, data.config.oidcClientId).catch(() => { failed = true; });
  });
</script>
{#if failed}<p role="alert">{i18n.t('errors.signInFailed')}</p><a href="/">{i18n.t('auth.signIn')}</a>{/if}
