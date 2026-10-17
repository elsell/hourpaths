<script lang="ts">
  import { createTranslator } from '@hourpaths/i18n';
  import { policyVersion } from '$lib/policies/documents';
  import '$lib/studio/presentation/studio.css';
  import type { PageData } from './$types';
  export let data: PageData;
  const i18n = createTranslator([data.locale]);
</script>

<svelte:head>
  <title>{i18n.t(data.document.title)} · {i18n.t('app.title')}</title>
  <meta name="description" content={i18n.t('publicPolicy.operator')} />
</svelte:head>

<div class="studio studio-policy">
  <header class="studio-policy-header">
    <a class="studio-brand" href="/">{i18n.t('app.title')}</a>
    <a href="/support">{i18n.t('support.title')}</a>
  </header>
  <main id="policy-content" tabindex="-1">
    <p class="studio-policy-meta">{i18n.t('publicPolicy.version', { version: policyVersion })}</p>
    <h1>{i18n.t(data.document.title)}</h1>
    <p class="studio-policy-meta">{i18n.t('publicPolicy.effective', { date: data.effectiveDate })}</p>
    <p>{i18n.t('publicPolicy.operator')}</p>
    {#each data.document.sections as section}
      <section>
        <h2>{i18n.t(section.title)}</h2>
        <p>{i18n.t(section.body)}</p>
      </section>
    {/each}
    <section>
      <h2>{i18n.t('publicPolicy.contact.title')}</h2>
      <p>{i18n.t('publicPolicy.contact.body')}</p>
      <a href="/support">{i18n.t('support.title')}</a>
    </section>
  </main>
  <footer>
    <nav class="studio-policy-links" aria-label={i18n.t('publicPolicy.navigation')}>
      <a href="/legal/terms">{i18n.t('publicPolicy.terms.title')}</a>
      <a href="/legal/privacy">{i18n.t('publicPolicy.privacy.title')}</a>
      <a href="/community-guidelines">{i18n.t('publicPolicy.guidelines.title')}</a>
      <a href="/support">{i18n.t('support.title')}</a>
    </nav>
  </footer>
</div>
