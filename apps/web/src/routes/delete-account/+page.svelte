<script lang="ts">
  import { createTranslator, type SupportedLocale } from '@hourpaths/i18n';
  import '$lib/studio/presentation/studio.css';
  export let data: { locale: SupportedLocale };
  $: i18n = createTranslator([data.locale]);
</script>

<svelte:head><title>{i18n.t('accountDelete.heading')}</title></svelte:head>
<div class="studio studio-entry"><main class="studio-entry-card">
  <h1>{i18n.t('accountDelete.heading')}</h1>
  <p>{i18n.t('accountDelete.intro')}</p>
  <p>{i18n.t('accountDelete.warning')}</p>
  <p>{i18n.t('accountDelete.paths')}</p>
  <p>{i18n.t('accountDelete.timers')}</p>
  <p>{i18n.t('accountDelete.retention')}</p>
  <a class="studio-primary" href="/studio/delete-account">{i18n.t('accountDelete.heading')}</a>
</main></div>
