<script lang="ts">
  import { subscribeBrowserFocus } from './notification-convergence-browser';
  import { onMount } from 'svelte';
  import { createPathAppearancePort } from '@hourpaths/api-client';
  import { createPathAppearanceStore, pathColorKeys, defaultPathAppearance, validPathEmoji, type PathAppearance } from '@hourpaths/client-core';
  import type { Translator } from '@hourpaths/i18n';
  export let apiURL: string;
  export let token: string;
  export let pathID: string;
  export let i18n: Translator;
  let appearance = defaultPathAppearance(pathID);
  let failed = false;
  let editing = false;
  let busy = false;
  let saveFailed = false;
  let revision = 0;
  let draft: PathAppearance = appearance;
  let disposed = false;
  const store = createPathAppearanceStore(createPathAppearancePort(apiURL, () => token), () => crypto.randomUUID(), () => {
    appearance = store.appearance(pathID);
    failed = store.hasFailures();
  });
  const refresh = () => { void store.refresh([pathID]); };
  onMount(() => {
    refresh();
    const unsubscribe = subscribeBrowserFocus(refresh);
    return () => { disposed = true; store.dispose(); unsubscribe(); };
  });
  async function openEditor() {
    if (!store.isLoaded(pathID)) await store.refresh([pathID]);
    if (disposed || !store.isLoaded(pathID)) return;
    draft = { ...store.appearance(pathID) }; revision = store.revision(pathID); editing = true; saveFailed = false;
  }
  async function save() {
    if (busy) return;
    busy = true; saveFailed = false;
    try { if (await store.save(pathID, { ...draft, emoji: draft.emoji.trim() }, revision) && !disposed) editing = false; }
    catch { if (!disposed) { saveFailed = true; revision = store.revision(pathID); } }
    finally { if (!disposed) busy = false; }
  }
</script>
<div class={`appearance tone-${appearance.color}`}>
  <span class="emoji" aria-hidden="true">{appearance.emoji}</span>
  <slot />
  <button class="edit" onclick={() => void openEditor()}>{i18n.t('home.appearance.title')}</button>
  {#if failed}<p role="status">{i18n.t('home.appearance.loadFailed')} <button onclick={refresh}>{i18n.t('common.retry')}</button></p>{/if}
  {#if editing}
    <form onsubmit={(event) => { event.preventDefault(); void save(); }}>
      <fieldset disabled={busy}>
        <legend>{i18n.t('home.appearance.title')}</legend>
        <label>{i18n.t('home.appearance.color')} <select bind:value={draft.color}>{#each pathColorKeys as color}<option value={color}>{i18n.t(`home.appearance.color.${color}`)}</option>{/each}</select></label>
        <label>{i18n.t('home.appearance.emoji')} <input bind:value={draft.emoji} maxlength="64" /></label>
        {#if !validPathEmoji(draft.emoji.trim())}<p role="alert">{i18n.t('home.appearance.invalidEmoji')}</p>{/if}
        {#if saveFailed}<p role="alert">{i18n.t('home.appearance.saveFailed')}</p>{/if}
        <button type="button" onclick={() => editing = false}>{i18n.t('common.cancel')}</button>
        <button type="submit" disabled={!validPathEmoji(draft.emoji.trim())}>{i18n.t(saveFailed ? 'common.retry' : 'common.save')}</button>
      </fieldset>
    </form>
  {/if}
</div>
<style>
  .tone-coral { background: #FFD8D6; color: #351E25; }
  .tone-lavender { background: #DFDEFF; color: #252044; }
  .tone-gold { background: #FFF0B8; color: #372C10; }
  .tone-mint { background: #CCF4DE; color: #173C2C; }
  .tone-blue { background: #D5E9FF; color: #19324E; }
  .tone-pink { background: #F0DBFF; color: #392044; }

  .appearance { padding: 1rem; border-radius: 1.25rem; }
  .emoji { display: block; font-size: 1.75rem; margin-bottom: .5rem; }
  .appearance :global(.path-link) { color: inherit; }
  .edit { display: block; margin-top: .75rem; background: transparent; color: inherit; }
  button,input,select { font: inherit; padding: .6rem; border-radius: .5rem; }
  button { cursor: pointer; }
  fieldset { border: 1px solid currentColor; border-radius: .75rem; margin-top: .75rem; }
  label { display: block; margin: .5rem 0; }
</style>
