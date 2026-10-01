<script lang="ts">
  import { onMount } from 'svelte';
  import type { ClientRuntimeConfig } from '@hourpaths/client-core';
  import type { SupportedLocale } from '@hourpaths/i18n';
  import { mountAccountEntry } from '$lib/studio/bootstrap/mount';
  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  let container: HTMLDivElement;
  onMount(() => mountAccountEntry(container, { config: data.config, locale: data.locale, callback: true }));
</script>

<div bind:this={container}></div>
