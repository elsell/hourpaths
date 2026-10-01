<script lang="ts">
  import { onMount } from 'svelte';
  import type { ClientRuntimeConfig } from '@hourpaths/client-core';
  import type { SupportedLocale } from '@hourpaths/i18n';
  import { mountStudio } from '$lib/studio/bootstrap/mount';
  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  let container: HTMLDivElement;
  onMount(() => mountStudio(container, { apiURL: data.config.apiURL, config: data.config, locale: data.locale }));
</script>

<div bind:this={container}></div>
