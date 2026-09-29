<script lang="ts">
  import { onMount } from 'svelte';
  import { createTranslator, type SupportedLocale } from '@hourpaths/i18n';
  import { applicationSession, applicationSessionExpired } from '$lib/auth';
  import { mountStudio } from '$lib/studio/bootstrap/mount';
  export let data: { locale: SupportedLocale; config: { apiURL: string } };
  let container: HTMLDivElement;
  let needsSession = false;
  const i18n = createTranslator([data.locale]);
  onMount(() => {
    const session = applicationSession();
    if (!session || applicationSessionExpired(session) || session.nextAction !== 'home') {
      needsSession = true;
      return;
    }
    return mountStudio(container, { apiURL: data.config.apiURL, locale: data.locale });
  });
</script>

{#if needsSession}<a href="/">{i18n.t('auth.signIn')}</a>{/if}
<div bind:this={container}></div>
