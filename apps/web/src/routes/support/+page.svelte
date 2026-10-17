<script lang="ts">
 import { onMount } from 'svelte';
 import type { ClientRuntimeConfig } from '@hourpaths/client-core';
 import type { SupportedLocale } from '@hourpaths/i18n';
 import { mountSupport } from '$lib/studio/bootstrap/mount';
 export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
 let container: HTMLDivElement;
 onMount(()=>mountSupport(container,data.config.apiURL,data.locale));
</script>
<svelte:head><meta name="referrer" content="no-referrer"/><meta name="robots" content="noindex,nofollow"/></svelte:head>
<div bind:this={container}></div>
