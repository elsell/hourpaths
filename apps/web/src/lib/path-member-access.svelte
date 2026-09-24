<script lang="ts">
  import type { ActivityDetail, PathMember } from '@hourpaths/api-client';
  import type { PathMemberRemovalReview } from '@hourpaths/client-core';
  import type { Translator } from '@hourpaths/i18n';

  export let i18n: Translator;
  export let archived = false;
  export let activities: readonly ActivityDetail[] = [];
  export let activitiesFailed = false;
  export let activitiesLoading = false;
  export let activitiesNextCursor = '';
  type PresentedPathMember = PathMember & {
    canChangeRole: boolean;
    canGrantAdministrator: boolean;
    canRevokeAdministrator: boolean;
    canStepDownAdministrator: boolean;
    isViewer: boolean;
  };
  export let members: readonly PresentedPathMember[] = [];
  export let loading = false;
  export let loadingMore = false;
  export let failed = false;
  export let nextCursor = '';
  export let selected: PresentedPathMember | null = null;
  export let review: PathMemberRemovalReview | null = null;
  export let reviewLoading = false;
  export let removalBusy = false;
  export let removalError = false;
  export let pendingRole: 'administrator' | 'participant' | 'supporter' | null = null;
  export let roleChangeBusy = false;
  export let roleChangeError = false;
  export let unblockBusy = false;
  export let unblockError = false;
  export let onClose: () => void;
  export let onRefresh: () => void;
  export let onLoadMore: () => void;
  export let onLoadMoreActivities: () => void;
  export let onOpenActivity: (activityID: string) => void;
  export let onSelect: (member: PresentedPathMember) => void;
  export let onBackToMembers: () => void;
  export let onRetryReview: () => void;
  export let onUnblock: () => void;
  export let onRemove: () => void;
  export let onCancelRoleChange: () => void;
  export let onChooseRole: (role: 'administrator' | 'participant' | 'supporter') => void;
  export let onConfirmRoleChange: () => void;

  function duration(totalSeconds: number): string {
    const seconds = Math.max(0, Math.floor(totalSeconds));
    if (seconds < 60) return i18n.t('duration.seconds', { seconds: i18n.number(seconds) });
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return i18n.t('duration.minutesSeconds', {
      minutes: i18n.number(minutes), seconds: i18n.number(seconds % 60),
    });
    return i18n.t('duration.hoursMinutes', {
      hours: i18n.number(Math.floor(minutes / 60)), minutes: i18n.number(minutes % 60),
    });
  }
</script>

<section class="access-destination" aria-labelledby="path-members-heading">
  <nav class="access-toolbar" aria-label={i18n.t('pathMembers.heading')}>
    <button type="button" onclick={() => selected ? onBackToMembers() : onClose()}>{i18n.t('common.back')}</button>
    <h2 id="path-members-heading">{i18n.t(selected ? 'pathMembers.memberHeading' : 'pathMembers.heading')}</h2>
    {#if !selected}<button type="button" disabled={loading} onclick={() => onRefresh()}>{i18n.t('common.refresh')}</button>{/if}
  </nav>

  {#if selected}
    <dl class="review-facts">
      <div><dt>{i18n.t('pathMembers.personLabel')}</dt><dd>{i18n.t('pathMembers.identity', { displayName: selected.displayName, username: selected.username })}</dd></div>
      <div><dt>{i18n.t('pathMembers.roleLabel')}</dt><dd>{i18n.t(selected.role === 'creator' ? 'pathMembers.creator' : selected.role === 'administrator' ? 'pathMembers.administrators' : selected.role === 'participant' ? 'pathMembers.participant' : 'pathMembers.supporter')}</dd></div>
      <div><dt>{i18n.t('pathMembers.sessionsLabel')}</dt><dd>{i18n.number(selected.sessionCount)}</dd></div>
      <div><dt>{i18n.t('pathMembers.totalTimeLabel')}</dt><dd>{duration(selected.totalTrackedSeconds)}</dd></div>
      {#if selected.intervalProgress}<div class="progress-fact">
        <dt>{i18n.t('pathMembers.intervalProgressLabel')}</dt>
        <dd><span>{i18n.t('pathMembers.progressValue', { accumulated: duration(selected.intervalProgress.accumulatedSeconds), target: duration(selected.intervalProgress.targetSeconds) })}</span><progress aria-label={i18n.t('pathMembers.intervalProgressLabel')} max={selected.intervalProgress.targetSeconds} value={Math.min(selected.intervalProgress.accumulatedSeconds, selected.intervalProgress.targetSeconds)}></progress></dd>
      </div>{/if}
      {#if selected.overallProgress}<div class="progress-fact">
        <dt>{i18n.t('pathMembers.overallProgressLabel')}</dt>
        <dd><span>{i18n.t('pathMembers.progressValue', { accumulated: duration(selected.overallProgress.accumulatedSeconds), target: duration(selected.overallProgress.targetSeconds) })}</span><progress aria-label={i18n.t('pathMembers.overallProgressLabel')} max={selected.overallProgress.targetSeconds} value={Math.min(selected.overallProgress.accumulatedSeconds, selected.overallProgress.targetSeconds)}></progress></dd>
      </div>{/if}
      {#if review}<div><dt>{i18n.t('pathMembers.timerLabel')}</dt><dd>{i18n.t(review.runningTimer ? 'pathMembers.timerRunning' : 'pathMembers.timerNotRunning')}</dd></div>{/if}
    </dl>
    {#if selected.canChangeRole && review}
      <section class="role-card" aria-labelledby="path-member-role-heading">
        <label id="path-member-role-heading" for="path-member-role">{i18n.t('pathMembers.changeRole')}</label>
        <select id="path-member-role" aria-label={i18n.t('pathMembers.changeRole')} disabled={removalBusy || unblockBusy || roleChangeBusy || pendingRole !== null} value={selected.role} onchange={(event) => onChooseRole(event.currentTarget.value as 'participant' | 'supporter')}>
          <option value="participant">{i18n.t('pathMembers.participant')}</option>
          <option value="supporter">{i18n.t('pathMembers.supporter')}</option>
        </select>
        <p>{i18n.t(selected.role === 'supporter' ? 'pathMembers.roleSupporterEffect' : 'pathMembers.roleParticipantEffect')}</p>
      </section>
      {#if pendingRole === 'supporter'}
        <section class="warning" role="alert" aria-labelledby="path-member-role-warning-heading">
          <h3 id="path-member-role-warning-heading">{i18n.t('pathMembers.confirmSupporter')}</h3>
          <p>{i18n.t('pathMembers.roleChangeWarning', { displayName: selected.displayName })}</p>
          <p>{i18n.t('pathMembers.roleChangeTimerWarning')}</p>
          <div class="compact-actions">
            <button class="destructive" type="button" disabled={roleChangeBusy} onclick={() => onConfirmRoleChange()}>{i18n.t(roleChangeBusy ? 'pathMembers.changingRole' : 'pathMembers.confirmSupporter')}</button>
            <button type="button" disabled={roleChangeBusy} onclick={() => onCancelRoleChange()}>{i18n.t('common.cancel')}</button>
          </div>
        </section>
      {/if}
    {/if}
    {#if selected.canGrantAdministrator || selected.canRevokeAdministrator || selected.canStepDownAdministrator}
      <section class="role-card" aria-labelledby="path-member-administrator-heading">
        <div>
          <strong id="path-member-administrator-heading">{i18n.t('pathMembers.administrators')}</strong>
          <p>{i18n.t('pathMembers.roleAdministratorEffect')}</p>
        </div>
        {#if selected.canGrantAdministrator}
          <button type="button" disabled={removalBusy || unblockBusy || roleChangeBusy || pendingRole !== null} onclick={() => onChooseRole('administrator')}>{i18n.t('pathMembers.confirmGrantAdministrator')}</button>
        {:else if selected.canRevokeAdministrator}
          <button type="button" disabled={removalBusy || unblockBusy || roleChangeBusy || pendingRole !== null} onclick={() => onChooseRole('participant')}>{i18n.t('pathMembers.confirmRevokeAdministrator')}</button>
        {:else if selected.canStepDownAdministrator}
          <button type="button" disabled={removalBusy || unblockBusy || roleChangeBusy || pendingRole !== null} onclick={() => onChooseRole('participant')}>{i18n.t('pathMembers.confirmStepDownAdministrator')}</button>
        {/if}
      </section>
      {#if pendingRole === 'administrator' || (pendingRole === 'participant' && selected.role === 'administrator')}
        {@const selfStepDown = selected.canStepDownAdministrator && selected.isViewer}
        {@const grant = pendingRole === 'administrator'}
        <section class="warning" role="alert" aria-labelledby="path-member-administrator-warning-heading">
          <h3 id="path-member-administrator-warning-heading">{i18n.t(grant ? 'pathMembers.confirmGrantAdministrator' : selfStepDown ? 'pathMembers.confirmStepDownAdministrator' : 'pathMembers.confirmRevokeAdministrator')}</h3>
          <p>{i18n.t(grant ? 'pathMembers.grantAdministratorWarning' : selfStepDown ? 'pathMembers.stepDownAdministratorWarning' : 'pathMembers.revokeAdministratorWarning', { displayName: selected.displayName })}</p>
          <div class="compact-actions">
            <button type="button" disabled={roleChangeBusy} onclick={() => onConfirmRoleChange()}>{i18n.t(roleChangeBusy ? 'pathMembers.changingRole' : grant ? 'pathMembers.confirmGrantAdministrator' : selfStepDown ? 'pathMembers.confirmStepDownAdministrator' : 'pathMembers.confirmRevokeAdministrator')}</button>
            <button type="button" disabled={roleChangeBusy} onclick={() => onCancelRoleChange()}>{i18n.t('common.cancel')}</button>
          </div>
        </section>
      {/if}
    {/if}
    {#if selected.canRemove && reviewLoading}
      <div class="state-card" role="status">{i18n.t('pathMembers.reviewLoading')}</div>
    {:else if selected.canRemove && !review}
      <div class="state-card" role="alert">
        <h3>{i18n.t('pathMembers.reviewUnavailableHeading')}</h3>
        <p>{i18n.t('pathMembers.reviewUnavailableDescription')}</p>
        <button type="button" onclick={() => onRetryReview()}>{i18n.t('common.retry')}</button>
      </div>
    {:else if review}
      <div class="warning" role="alert">
        {#if review.role === 'participant'}
          <p>{i18n.t('pathMembers.participantActivityWarning')}</p>
          <p>{i18n.t('pathMembers.participantSocialWarning')}</p>
          <p>{i18n.t('pathMembers.offlineWarning')}</p>
          <p>{i18n.t('pathMembers.runningTimerWarning')}</p>
          <p>{i18n.t('pathMembers.reinviteWarning')}</p>
        {:else}
          <p>{i18n.t('pathMembers.supporterWarning')}</p>
        {/if}
      </div>
      <button class="destructive" type="button" disabled={removalBusy || unblockBusy || !selected.canRemove} onclick={() => onRemove()}>
        {i18n.t(removalBusy ? 'pathMembers.removing' : review.role === 'participant' ? 'pathMembers.removeParticipantAndData' : 'pathMembers.removeSupporter')}
      </button>
    {/if}
    {#if selected.blockedByViewer}<button type="button" disabled={removalBusy || unblockBusy} onclick={() => onUnblock()}>{i18n.t(unblockBusy ? 'common.loading' : 'blocking.unblock')}</button>{/if}
    {#if removalError}<p role="alert">{i18n.t('pathMembers.removalUnavailable')}</p>{/if}
    {#if unblockError}<p role="alert">{i18n.t('blocking.unblockUnavailable')}</p>{/if}
    {#if roleChangeError}<p role="alert">{i18n.t('pathMembers.roleChangeUnavailable')}</p>{/if}
    <section aria-labelledby="path-member-sessions-heading">
      <h3 id="path-member-sessions-heading">{i18n.t('pathDetails.history')}</h3>
      {#if activities.length === 0 && !activitiesLoading && !activitiesFailed}<p>{i18n.t('pathDetails.historyEmpty')}</p>{/if}
      {#if activities.length > 0}<ul class="member-list">
        {#each activities as detail (detail.activity.id)}
          <li><button class="member-row" type="button" onclick={() => onOpenActivity(detail.activity.id)}>
            <span><strong>{i18n.date(new Date(detail.activity.startedAt), { dateStyle: 'medium', timeZone: detail.activity.occurrenceTimeZone })}</strong><small>{i18n.time(new Date(detail.activity.startedAt), { timeStyle: 'short', timeZone: detail.activity.occurrenceTimeZone })}{#if detail.version > 1} · {i18n.t('pathDetails.edited')}{/if}</small></span>
            <span>{duration(detail.activity.durationSeconds)} ›</span>
          </button></li>
        {/each}
      </ul>{/if}
      {#if activitiesFailed}<p role="alert">{i18n.t('pathMembers.unavailableDescription')}</p><button type="button" onclick={() => onLoadMoreActivities()}>{i18n.t('common.retry')}</button>{/if}
      {#if activitiesNextCursor}<button type="button" disabled={activitiesLoading} onclick={() => onLoadMoreActivities()}>{i18n.t(activitiesLoading ? 'common.loading' : 'common.loadMore')}</button>{/if}
      {#if activitiesLoading}<p role="status">{i18n.t('common.loading')}</p>{/if}
    </section>
  {:else if loading && members.length === 0}
    <div class="state-card" role="status">{i18n.t('pathMembers.loading')}</div>
  {:else if failed && members.length === 0}
    <div class="state-card" role="alert">
      <h3>{i18n.t('pathMembers.unavailableHeading')}</h3>
      <p>{i18n.t('pathMembers.unavailableDescription')}</p>
      <button type="button" onclick={() => onRefresh()}>{i18n.t('common.retry')}</button>
    </div>
  {:else if members.length === 0}
    <div class="state-card" role="status"><h3>{i18n.t('pathMembers.emptyHeading')}</h3><p>{i18n.t('pathMembers.emptyDescription')}</p></div>
  {:else}
    {#if archived}<p class="archived-note">{i18n.t('pathArchive.readOnly')}</p>{/if}
    <ul class="member-list">
      {#each members as member (member.userId)}
        <li>
          <button class="member-row" type="button" onclick={() => onSelect(member)}>
            <span><strong>{member.displayName}</strong><small>@{member.username}</small>
              {#if member.intervalProgress || member.overallProgress}<span class="progress-comparison">
                {#if member.intervalProgress}<span><small>{i18n.t('pathMembers.progressValue', { accumulated: duration(member.intervalProgress.accumulatedSeconds), target: duration(member.intervalProgress.targetSeconds) })}</small><progress aria-label={i18n.t('pathMembers.intervalProgressLabel')} max={member.intervalProgress.targetSeconds} value={Math.min(member.intervalProgress.accumulatedSeconds, member.intervalProgress.targetSeconds)}></progress></span>{/if}
                {#if member.overallProgress}<span><small>{i18n.t('pathMembers.progressValue', { accumulated: duration(member.overallProgress.accumulatedSeconds), target: duration(member.overallProgress.targetSeconds) })}</small><progress aria-label={i18n.t('pathMembers.overallProgressLabel')} max={member.overallProgress.targetSeconds} value={Math.min(member.overallProgress.accumulatedSeconds, member.overallProgress.targetSeconds)}></progress></span>{/if}
              </span>{/if}
            </span>
            <span>{i18n.t(member.role === 'creator' ? 'pathMembers.creator' : member.role === 'administrator' ? 'pathMembers.administrators' : member.role === 'participant' ? 'pathMembers.participant' : 'pathMembers.supporter')} ›</span>
          </button>
        </li>
      {/each}
    </ul>
    {#if failed}<p role="alert">{i18n.t('pathMembers.unavailableDescription')}</p>{/if}
    {#if nextCursor}<button type="button" disabled={loadingMore} onclick={() => onLoadMore()}>{i18n.t(loadingMore ? 'common.loading' : 'common.loadMore')}</button>{/if}
  {/if}
</section>

<style>
  .access-destination { display: grid; gap: 1rem; max-width: 48rem; margin: 0 auto; }
  .access-toolbar { align-items: center; display: grid; grid-template-columns: 1fr auto 1fr; }
  .access-toolbar h2 { margin: 0; text-align: center; }
  .access-toolbar button:last-child { justify-self: end; }
  .member-list { background: var(--surface, #202020); border-radius: .75rem; list-style: none; margin: 0; overflow: hidden; padding: 0; }
  .member-list li + li { border-top: 1px solid color-mix(in srgb, currentColor 16%, transparent); }
  .member-row { align-items: center; background: transparent; border: 0; color: inherit; display: flex; justify-content: space-between; min-height: 3.5rem; padding: .55rem 1rem; text-align: left; width: 100%; }
  .member-row > span:first-child { display: grid; min-width: 0; }
  .member-row small, .member-row > span:last-child { opacity: .68; }
  .progress-comparison { display: grid; gap: .2rem; margin-top: .35rem; }
  .progress-comparison > span { display: grid; gap: .1rem; }
  .progress-comparison progress { accent-color: var(--accent, #ffd43b); width: min(15rem, 100%); }
  .review-facts { background: var(--surface, #202020); border-radius: .75rem; margin: 0; overflow: hidden; }
  .review-facts div { display: flex; gap: 1rem; justify-content: space-between; padding: .75rem 1rem; }
  .review-facts div + div { border-top: 1px solid color-mix(in srgb, currentColor 16%, transparent); }
  .review-facts dd { margin: 0; text-align: right; }
  .review-facts .progress-fact { align-items: center; }
  .progress-fact dd { display: grid; gap: .3rem; min-width: min(15rem, 58%); }
  .progress-fact progress { accent-color: var(--accent, #ffd43b); width: 100%; }
  .warning, .state-card, .role-card { background: var(--surface, #202020); border-radius: .75rem; padding: 1rem; }
  .role-card { align-items: center; display: grid; grid-template-columns: 1fr auto; gap: .35rem 1rem; }
  .role-card label { font-weight: 600; }
  .role-card select { min-height: 2.75rem; }
  .role-card p { grid-column: 1 / -1; margin: 0; opacity: .7; }
  .compact-actions { display: flex; flex-wrap: wrap; gap: .5rem; }
  .warning p:first-child { margin-top: 0; }
  .warning p:last-child { margin-bottom: 0; }
  .destructive { color: #ff6961; min-height: 2.75rem; }
  .archived-note { opacity: .75; }
</style>
