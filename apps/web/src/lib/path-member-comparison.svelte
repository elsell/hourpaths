<script lang="ts">
  import type { PathMember } from '@hourpaths/api-client';
  import type { Translator } from '@hourpaths/i18n';

  export let i18n: Translator;
  export let members: readonly PathMember[] = [];
  export let loading = false;
  export let loadingMore = false;
  export let failed = false;
  export let nextCursor = '';
  export let onLoadMore: () => void;
  export let onRetry: () => void;
  export let onSelect: (member: PathMember) => void;

  $: participants = members.filter((member) => member.role !== 'supporter');

  function duration(totalSeconds: number): string {
    const seconds = Math.max(0, Math.floor(totalSeconds));
    if (seconds < 60) return i18n.t('duration.seconds', { seconds: i18n.number(seconds) });
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return i18n.t('duration.minutesSeconds', { minutes: i18n.number(minutes), seconds: i18n.number(seconds % 60) });
    return i18n.t('duration.hoursMinutes', { hours: i18n.number(Math.floor(minutes / 60)), minutes: i18n.number(minutes % 60) });
  }
</script>

<section class="path-member-comparison" aria-labelledby="path-participant-comparison-heading">
  <h3 id="path-participant-comparison-heading">{i18n.t('pathMembers.heading')}</h3>
  {#if loading && participants.length === 0}
    <p role="status">{i18n.t('pathMembers.loading')}</p>
  {:else if failed && participants.length === 0}
    <div role="alert"><p>{i18n.t('pathMembers.unavailableDescription')}</p><button type="button" onclick={onRetry}>{i18n.t('common.retry')}</button></div>
  {:else if participants.length === 0}
    <p>{i18n.t('pathMembers.emptyDescription')}</p>
  {:else}
    <ul>
      {#each participants as member (member.userId)}
        <li><button type="button" onclick={() => onSelect(member)}>
          <span><strong>{member.displayName}</strong><small>@{member.username}</small></span>
          <span class="progresses">
            {#if member.intervalProgress}<span><small>{i18n.t('pathMembers.intervalProgressLabel')} · {i18n.t('pathMembers.progressValue', { accumulated: duration(member.intervalProgress.accumulatedSeconds), target: duration(member.intervalProgress.targetSeconds) })}</small><progress aria-label={i18n.t('pathMembers.intervalProgressLabel')} max={member.intervalProgress.targetSeconds} value={Math.min(member.intervalProgress.accumulatedSeconds, member.intervalProgress.targetSeconds)}></progress></span>{/if}
            {#if member.overallProgress}<span><small>{i18n.t('pathMembers.overallProgressLabel')} · {i18n.t('pathMembers.progressValue', { accumulated: duration(member.overallProgress.accumulatedSeconds), target: duration(member.overallProgress.targetSeconds) })}</small><progress aria-label={i18n.t('pathMembers.overallProgressLabel')} max={member.overallProgress.targetSeconds} value={Math.min(member.overallProgress.accumulatedSeconds, member.overallProgress.targetSeconds)}></progress></span>{/if}
          </span>
          <span class="role">{i18n.t(member.role === 'creator' ? 'pathMembers.creator' : member.role === 'administrator' ? 'pathMembers.administrators' : member.role === 'participant' ? 'pathMembers.participant' : 'pathMembers.supporter')} ›</span>
        </button></li>
      {/each}
    </ul>
    {#if failed}<p role="alert">{i18n.t('pathMembers.unavailableDescription')}</p><button type="button" onclick={onRetry}>{i18n.t('common.retry')}</button>{/if}
    {#if nextCursor}<button type="button" disabled={loadingMore} onclick={onLoadMore}>{i18n.t(loadingMore ? 'common.loading' : 'common.loadMore')}</button>{/if}
  {/if}
</section>

<style>
  .path-member-comparison { display: grid; gap: .6rem; }
  h3 { margin: 0; }
  ul { background: var(--surface, #202020); border-radius: .75rem; list-style: none; margin: 0; overflow: hidden; padding: 0; }
  li + li { border-top: 1px solid color-mix(in srgb, currentColor 16%, transparent); }
  li > button { align-items: center; background: transparent; border: 0; color: inherit; display: grid; gap: .35rem 1rem; grid-template-columns: minmax(8rem, 1fr) minmax(10rem, 1.4fr) auto; min-height: 3.5rem; padding: .6rem 1rem; text-align: left; width: 100%; }
  li > button > span:first-child, .progresses, .progresses > span { display: grid; min-width: 0; }
  small, .role { opacity: .7; }
  progress { accent-color: var(--accent, #ffd43b); width: 100%; }
  @media (max-width: 42rem) { li > button { grid-template-columns: minmax(0, 1fr) auto; } .progresses { grid-column: 1 / -1; } }
</style>
