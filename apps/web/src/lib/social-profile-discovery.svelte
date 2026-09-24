<script lang="ts">
  import type { Translator } from '@hourpaths/i18n';

  type SocialPublicProfile = Readonly<{
    userId: string;
    username: string;
    displayName: string;
    description?: string;
    followerCount: number;
    followingCount: number;
    relationship: 'self' | 'none' | 'requested' | 'following';
  }>;

  type SocialFollowRequest = Readonly<{ id: string; requester: SocialPublicProfile; createdAt: string }>;
  type SocialBlockIdentity = Readonly<{ userId: string; username: string; displayName: string }>;
  type SocialBlockReview = Readonly<{
    target: SocialBlockIdentity;
    sharedPaths: readonly Readonly<{ id: string; name: string }>[];
  }>;
  type SocialBlockedAccount = SocialBlockIdentity & Readonly<{ blockedAt: string }>;

  type SocialSearchState = 'hint' | 'loading' | 'empty' | 'error' | 'results';
  type SocialProfileState = 'idle' | 'loading' | 'error' | 'ready';

  export let i18n: Translator;
  export let query = '';
  export let searchState: SocialSearchState = 'hint';
  export let results: readonly SocialPublicProfile[] = [];
  export let nextCursor = '';
  export let loadingMore = false;
  export let selectedProfile: SocialPublicProfile | null = null;
  export let profileState: SocialProfileState = 'idle';
  export let relationshipBusy = false;
  export let relationshipError = false;
  export let followRequestsOpen = false;
  export let followRequestsState: 'idle' | 'loading' | 'empty' | 'error' | 'ready' = 'idle';
  export let followRequests: readonly SocialFollowRequest[] = [];
  export let followRequestsNextCursor = '';
  export let busyFollowRequestID = '';
  export let blockReview: SocialBlockReview | null = null;
  export let blockBusy = false;
  export let blockError = false;
  export let blockedAccountsOpen = false;
  export let blockedAccountsState: 'idle' | 'loading' | 'empty' | 'error' | 'ready' = 'idle';
  export let blockedAccounts: readonly SocialBlockedAccount[] = [];
  export let blockedAccountsNextCursor = '';
  export let unblockReview: SocialBlockedAccount | null = null;
  export let onQueryChange: (query: string) => void;
  export let onSearch: () => void;
  export let onSelectProfile: (userId: string) => void;
  export let onRetrySearch: () => void;
  export let onLoadMore: () => void;
  export let onCloseProfile: () => void;
  export let onRefreshProfile: () => void;
  export let onRelationshipAction: (action: 'follow' | 'cancel-request' | 'unfollow') => void;
  export let onOpenFollowRequests: () => void;
  export let onCloseFollowRequests: () => void;
  export let onLoadMoreFollowRequests: () => void;
  export let onReviewFollowRequest: (decision: 'accept' | 'reject', requestID: string) => void;
  export let onReviewBlock: (username: string) => void;
  export let onCancelBlock: () => void;
  export let onConfirmBlock: () => void;
  export let onOpenBlockedAccounts: () => void;
  export let onCloseBlockedAccounts: () => void;
  export let onRetryBlockedAccounts: () => void;
  export let onLoadMoreBlockedAccounts: () => void;
  export let onReviewUnblock: (username: string) => void;
  export let onCancelUnblock: () => void;
  export let onConfirmUnblock: () => void;

  $: queryIneligible = Array.from(query).filter((character) => !/\s/u.test(character)).length < 2;

  function submitSearch(event: SubmitEvent) {
    event.preventDefault();
    if (!queryIneligible) onSearch();
  }
</script>

{#if blockedAccountsOpen}
  <section class="profile-destination" aria-labelledby="blocked-accounts-heading">
    <nav class="destination-toolbar" aria-label={i18n.t('blocking.settingsHeading')}>
      <button class="back-button" type="button" onclick={() => onCloseBlockedAccounts()}>
        <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m15 5-7 7 7 7" /></svg>
        <span>{i18n.t('common.back')}</span>
      </button>
      <h2 id="blocked-accounts-heading">{i18n.t('blocking.settingsHeading')}</h2>
    </nav>
    {#if blockedAccountsState === 'loading'}
      <div class="state-card" role="status"><span class="activity-indicator" aria-hidden="true"></span><p>{i18n.t('common.loading')}</p></div>
    {:else if blockedAccountsState === 'error' && blockedAccounts.length === 0}
      <div class="state-card" role="alert">
        <div class="state-icon" aria-hidden="true">!</div>
        <h3>{i18n.t('blocking.unavailableHeading')}</h3>
        <button class="accent-action" type="button" onclick={() => onRetryBlockedAccounts()}>{i18n.t('common.retry')}</button>
      </div>
    {:else if blockedAccountsState === 'empty' || blockedAccounts.length === 0}
      <div class="state-card" role="status"><h3>{i18n.t('blocking.emptyHeading')}</h3><p>{i18n.t('blocking.emptyDescription')}</p></div>
    {:else}
      <ul class="profile-results blocked-account-results">
        {#each blockedAccounts as account (account.userId)}
          <li class="request-row">
            <span class="neutral-avatar" aria-hidden="true"></span>
            <span class="identity-copy"><strong>{account.displayName}</strong><span>@{account.username}</span></span>
            <button class="plain-action" type="button" disabled={blockBusy} onclick={() => onReviewUnblock(account.username)}>{i18n.t('blocking.unblock')}</button>
          </li>
        {/each}
      </ul>
      {#if blockedAccountsNextCursor}<button class="load-more" disabled={blockBusy} onclick={() => onLoadMoreBlockedAccounts()}>{i18n.t('blocking.loadMore')}</button>{/if}
      {#if blockedAccountsState === 'error'}<p class="mutation-error" role="alert">{i18n.t('blocking.unavailableDescription')}</p>{/if}
    {/if}
  </section>
{:else if followRequestsOpen}
  <section class="profile-destination" aria-labelledby="follow-requests-heading">
    <nav class="destination-toolbar" aria-label={i18n.t('social.followRequests')}>
      <button class="back-button" type="button" onclick={() => onCloseFollowRequests()}>
        <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m15 5-7 7 7 7" /></svg>
        <span>{i18n.t('common.back')}</span>
      </button>
      <h2 id="follow-requests-heading">{i18n.t('social.followRequests')}</h2>
    </nav>
    {#if followRequestsState === 'loading'}
      <div class="state-card" role="status"><span class="activity-indicator" aria-hidden="true"></span></div>
    {:else if followRequestsState === 'error' && followRequests.length === 0}
      <div class="state-card" role="alert"><div class="state-icon" aria-hidden="true">!</div><p>{i18n.t('social.followRequestsUnavailable')}</p></div>
    {:else if followRequests.length === 0}
      <div class="state-card" role="status"><h3>{i18n.t('social.followRequestsEmpty')}</h3><p>{i18n.t('social.followRequestsEmptyDescription')}</p></div>
    {:else}
      <ul class="profile-results request-results">
        {#each followRequests as request (request.id)}
          <li class="request-row">
            <span class="neutral-avatar" aria-hidden="true"></span>
            <span class="identity-copy"><strong>{request.requester.displayName}</strong><span>@{request.requester.username}</span></span>
            <span class="request-actions">
              <button class="approve-action" type="button" disabled={Boolean(busyFollowRequestID)} onclick={() => onReviewFollowRequest('accept', request.id)}>{i18n.t('social.approveRequest')}</button>
              <button class="plain-action" type="button" disabled={Boolean(busyFollowRequestID)} onclick={() => onReviewFollowRequest('reject', request.id)}>{i18n.t('social.declineRequest')}</button>
            </span>
          </li>
        {/each}
      </ul>
      {#if followRequestsNextCursor}<button class="load-more" disabled={Boolean(busyFollowRequestID)} onclick={() => onLoadMoreFollowRequests()}>{i18n.t('social.loadMore')}</button>{/if}
    {/if}
  </section>
{:else if profileState !== 'idle'}
  <section class="profile-destination" aria-labelledby="social-profile-heading">
    <nav class="destination-toolbar" aria-label={i18n.t('social.profileHeading')}>
      <button class="back-button" type="button" onclick={() => onCloseProfile()}>
        <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m15 5-7 7 7 7" /></svg>
        <span>{i18n.t('common.back')}</span>
      </button>
      <h2 id="social-profile-heading">{i18n.t('social.profileHeading')}</h2>
      {#if selectedProfile && selectedProfile.relationship !== 'self'}
        <button class="profile-overflow" type="button" aria-label={i18n.t('blocking.blockActionLabel', { username: selectedProfile.username })} disabled={blockBusy} onclick={() => onReviewBlock(selectedProfile.username)}>•••</button>
      {/if}
    </nav>

    {#if profileState === 'loading'}
      <div class="state-card" role="status" aria-live="polite">
        <span class="activity-indicator" aria-hidden="true"></span>
        <p>{i18n.t('common.loading')}</p>
      </div>
    {:else if profileState === 'error' || !selectedProfile}
      <div class="state-card" role="alert">
        <div class="state-icon" aria-hidden="true">!</div>
        <h3>{i18n.t('social.profileUnavailableHeading')}</h3>
        <p>{i18n.t('social.profileUnavailableDescription')}</p>
        <button class="accent-action" type="button" onclick={() => onRefreshProfile()}>{i18n.t('social.profileRefresh')}</button>
      </div>
    {:else}
      <article class="profile-card">
        <div class="neutral-avatar profile-avatar-large" role="img" aria-label={i18n.t('social.neutralAvatarLabel')}>
          <svg aria-hidden="true" viewBox="0 0 32 32">
            <circle cx="16" cy="11" r="5" />
            <path d="M6.5 27c.8-6 4-9 9.5-9s8.7 3 9.5 9" />
          </svg>
        </div>
        <h3>{selectedProfile.displayName}</h3>
        <p class="username">@{selectedProfile.username}</p>
        {#if selectedProfile.description}<p class="description">{selectedProfile.description}</p>{/if}
        {#if selectedProfile.relationship !== 'self'}
          <button
            class:selected-relationship={selectedProfile.relationship !== 'none'}
            class="profile-action"
            disabled={relationshipBusy}
            type="button"
            onclick={() => onRelationshipAction(selectedProfile.relationship === 'none' ? 'follow' : selectedProfile.relationship === 'requested' ? 'cancel-request' : 'unfollow')}
          >{i18n.t(selectedProfile.relationship === 'none' ? 'social.follow' : selectedProfile.relationship === 'requested' ? 'social.requested' : 'social.followingAction')}</button>
        {/if}
        {#if relationshipError}<p class="mutation-error" role="alert">{i18n.t('social.relationshipUnavailable')}</p>{/if}
        <dl class="profile-counts">
          <div>
            <dd>{i18n.number(selectedProfile.followerCount)}</dd>
            <dt>{i18n.t('social.profileFollowers')}</dt>
          </div>
          <div>
            <dd>{i18n.number(selectedProfile.followingCount)}</dd>
            <dt>{i18n.t('social.profileFollowing')}</dt>
          </div>
        </dl>
      </article>
    {/if}
  </section>
{:else}
  <section class="search-destination" aria-labelledby="social-following-heading">
    <div class="following-heading">
      <h2 id="social-following-heading">{i18n.t('social.following')}</h2>
      <span class="heading-actions">
        <button class="plain-action" type="button" onclick={() => onOpenBlockedAccounts()}>{i18n.t('blocking.settingsHeading')}</button>
        <button class="plain-action" type="button" onclick={() => onOpenFollowRequests()}>{i18n.t('social.followRequests')}</button>
      </span>
    </div>
    <form class="search-form" role="search" onsubmit={submitSearch}>
      <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m15.5 15.5 5 5" /></svg>
      <input
        type="search"
        value={query}
        aria-label={i18n.t('social.searchPlaceholder')}
        placeholder={i18n.t('social.searchPlaceholder')}
        autocomplete="off"
        enterkeyhint="search"
        oninput={(event) => onQueryChange(event.currentTarget.value)}
      />
      <button type="submit" disabled={queryIneligible} aria-label={i18n.t('social.searchPlaceholder')}>
        <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m9 6 6 6-6 6" /></svg>
      </button>
    </form>

    {#if searchState === 'hint'}
      <p class="hint" role="status">{i18n.t('social.searchHint')}</p>
    {:else if searchState === 'loading'}
      <div class="inline-state" role="status" aria-live="polite">
        <span class="activity-indicator" aria-hidden="true"></span>
        <span>{i18n.t('social.searchLoading')}</span>
      </div>
    {:else if searchState === 'empty'}
      <div class="state-card" role="status">
        <div class="state-icon" aria-hidden="true">⌕</div>
        <h3>{i18n.t('social.searchEmptyHeading')}</h3>
        <p>{i18n.t('social.searchEmptyDescription')}</p>
      </div>
    {:else if searchState === 'error'}
      <div class="state-card" role="alert">
        <div class="state-icon" aria-hidden="true">!</div>
        <h3>{i18n.t('social.searchUnavailableHeading')}</h3>
        <p>{i18n.t('social.searchUnavailableDescription')}</p>
        <button class="accent-action" type="button" onclick={() => onRetrySearch()}>{i18n.t('common.retry')}</button>
      </div>
    {:else if searchState === 'results'}
      <h3 class="results-heading">{i18n.t('social.searchResults')}</h3>
      <ul class="profile-results">
        {#each results as profile (profile.userId)}
          <li>
            <button class="profile-row" type="button" onclick={() => onSelectProfile(profile.userId)}>
              <span class="neutral-avatar" role="img" aria-label={i18n.t('social.neutralAvatarLabel')}>
                <svg aria-hidden="true" viewBox="0 0 32 32">
                  <circle cx="16" cy="11" r="5" />
                  <path d="M6.5 27c.8-6 4-9 9.5-9s8.7 3 9.5 9" />
                </svg>
              </span>
              <span class="identity-copy">
                <strong>{profile.displayName}</strong>
                <span>@{profile.username}</span>
              </span>
              <svg class="disclosure" aria-hidden="true" viewBox="0 0 24 24"><path d="m9 5 7 7-7 7" /></svg>
            </button>
          </li>
        {/each}
      </ul>
      {#if nextCursor}
        <button class="load-more" type="button" disabled={loadingMore} onclick={() => onLoadMore()}>
          {i18n.t(loadingMore ? 'social.loadingMore' : 'social.loadMore')}
        </button>
      {/if}
    {/if}
  </section>
{/if}

{#if blockReview}
  <div class="dialog-backdrop">
    <div class="confirmation-dialog" role="alertdialog" aria-modal="true" aria-labelledby="block-review-heading" aria-describedby="block-review-description">
      <h2 id="block-review-heading">{i18n.t('blocking.confirmTitle', { username: blockReview.target.username })}</h2>
      <p id="block-review-description">{i18n.t('blocking.confirmDescription')}</p>
      {#if blockReview.sharedPaths.length > 0}
        <p class="shared-path-warning">{i18n.t('blocking.sharedPathsWarning', { count: blockReview.sharedPaths.length, paths: blockReview.sharedPaths.map((path) => path.name).join(', '), username: blockReview.target.username })}</p>
        <ul class="shared-path-list">{#each blockReview.sharedPaths as path (path.id)}<li>{path.name}</li>{/each}</ul>
        <p>{i18n.t('blocking.leavePathsSeparately')}</p>
      {/if}
      {#if blockError}<p class="mutation-error" role="alert">{i18n.t('blocking.blockUnavailable')}</p>{/if}
      <div class="dialog-actions">
        <button class="plain-action" type="button" disabled={blockBusy} onclick={() => onCancelBlock()}>{i18n.t('common.cancel')}</button>
        <button class="destructive-action" type="button" disabled={blockBusy} onclick={() => onConfirmBlock()}>{i18n.t('blocking.blockAction')}</button>
      </div>
    </div>
  </div>
{/if}

{#if unblockReview}
  <div class="dialog-backdrop">
    <div class="confirmation-dialog" role="alertdialog" aria-modal="true" aria-labelledby="unblock-review-heading" aria-describedby="unblock-review-description">
      <h2 id="unblock-review-heading">{i18n.t('blocking.unblockConfirmTitle', { username: unblockReview.username })}</h2>
      <p id="unblock-review-description">{i18n.t('blocking.unblockConfirmDescription')}</p>
      {#if blockError}<p class="mutation-error" role="alert">{i18n.t('blocking.unblockUnavailable')}</p>{/if}
      <div class="dialog-actions">
        <button class="plain-action" type="button" disabled={blockBusy} onclick={() => onCancelUnblock()}>{i18n.t('common.cancel')}</button>
        <button class="destructive-action" type="button" disabled={blockBusy} onclick={() => onConfirmUnblock()}>{i18n.t('blocking.unblock')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  :global(*) { box-sizing: border-box; }
  .search-destination,.profile-destination{width:100%;max-width:42rem;margin:0 auto;color:var(--hp-label,#f5f5f7);font:400 1rem/1.35 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
  h2{margin:.25rem 1rem 1.25rem;font-size:2rem;line-height:1.1;letter-spacing:-.03em}h3,p{margin:0}.search-form{display:flex;align-items:center;gap:.5rem;margin:0 1rem 1rem;padding:.15rem .3rem .15rem .75rem;border:1px solid var(--hp-separator,#3a3a3c);border-radius:.8rem;background:var(--hp-secondary-background,#1c1c1e)}.search-form>svg{width:1.1rem;stroke:var(--hp-secondary-label,#a1a1a6);fill:none;stroke-width:2}.search-form input{min-width:0;flex:1;margin:0;padding:.6rem 0;border:0;outline:0;background:transparent;color:inherit;font:inherit}.search-form input::placeholder{color:var(--hp-secondary-label,#a1a1a6)}.search-form button{display:grid;width:2.25rem;height:2.25rem;place-items:center;border:0;border-radius:50%;background:var(--hp-accent,#ffd43b);color:#171717}.search-form button:disabled{opacity:.35}.search-form button svg,.back-button svg,.disclosure{width:1.15rem;fill:none;stroke:currentColor;stroke-linecap:round;stroke-linejoin:round;stroke-width:2}
  .hint,.results-heading{margin:.25rem 1rem;color:var(--hp-secondary-label,#a1a1a6)}.hint{font-size:.875rem}.results-heading{padding:.65rem 0 .35rem;font-size:.8125rem;font-weight:600;letter-spacing:.02em;text-transform:uppercase}.profile-results{margin:0;padding:0;list-style:none;border-block:1px solid var(--hp-separator,#3a3a3c)}.profile-results li+li{border-top:1px solid var(--hp-separator,#3a3a3c)}.profile-row{display:flex;width:100%;min-height:4rem;align-items:center;gap:.75rem;padding:.55rem 1rem;border:0;border-radius:0;background:transparent;color:inherit;text-align:left}.profile-row:active{background:var(--hp-tertiary-background,#2c2c2e)}.neutral-avatar{display:grid;width:2.75rem;height:2.75rem;flex:0 0 auto;place-items:center;overflow:hidden;border-radius:50%;background:var(--hp-tertiary-background,#2c2c2e);color:var(--hp-secondary-label,#a1a1a6)}.neutral-avatar svg{width:70%;fill:none;stroke:currentColor;stroke-linecap:round;stroke-width:1.8}.identity-copy{display:flex;min-width:0;flex:1;flex-direction:column}.identity-copy strong,.identity-copy span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.identity-copy strong{font-size:1rem}.identity-copy span{color:var(--hp-secondary-label,#a1a1a6);font-size:.875rem}.disclosure{width:1rem;color:var(--hp-secondary-label,#a1a1a6)}
  .inline-state{display:flex;align-items:center;justify-content:center;gap:.65rem;padding:2rem 1rem;color:var(--hp-secondary-label,#a1a1a6)}.activity-indicator{width:1rem;height:1rem;border:2px solid currentColor;border-right-color:transparent;border-radius:50%;animation:spin .8s linear infinite}.state-card{display:grid;justify-items:center;gap:.55rem;padding:3rem 1.5rem;text-align:center}.state-card p{max-width:25rem;color:var(--hp-secondary-label,#a1a1a6)}.state-icon{display:grid;width:3.25rem;height:3.25rem;place-items:center;border-radius:50%;background:var(--hp-tertiary-background,#2c2c2e);color:var(--hp-secondary-label,#a1a1a6);font-size:1.5rem}.accent-action,.load-more{margin-top:.5rem;border:0;background:transparent;color:var(--hp-accent,#ffd43b);font:600 1rem system-ui}.load-more{display:block;margin:1rem auto;padding:.6rem 1rem}.load-more:disabled{opacity:.5}
  .destination-toolbar{display:grid;grid-template-columns:1fr auto 1fr;align-items:center;min-height:3rem;margin:0 .5rem 1.5rem}.destination-toolbar h2{grid-column:2;margin:0;font-size:1.1rem;letter-spacing:0}.back-button{display:flex;grid-column:1;grid-row:1;align-items:center;justify-self:start;gap:.15rem;padding:.5rem;border:0;background:transparent;color:var(--hp-accent,#ffd43b);font:inherit}.profile-overflow{grid-column:3;grid-row:1;justify-self:end;padding:.5rem;border:0;background:transparent;color:var(--hp-label,#f5f5f7);font:700 1rem system-ui;letter-spacing:.08em}.profile-card{display:grid;justify-items:center;padding:1rem;text-align:center}.profile-avatar-large{width:7.5rem;height:7.5rem;margin-bottom:1.15rem}.profile-card h3{font-size:1.5rem}.username{margin-top:.2rem;color:var(--hp-secondary-label,#a1a1a6)}.description{max-width:30rem;margin-top:1rem}.profile-counts{display:flex;gap:3rem;margin:1.75rem 0 0}.profile-counts div{display:flex;flex-direction:column}.profile-counts dd{order:-1;margin:0;font-size:1.2rem;font-weight:700}.profile-counts dt{color:var(--hp-secondary-label,#a1a1a6);font-size:.8125rem;font-weight:500}
  .following-heading{display:flex;align-items:center;justify-content:space-between;padding-right:1rem}.following-heading h2{margin-right:1rem}.heading-actions{display:flex;align-items:center;gap:.25rem}.plain-action{border:0;background:transparent;color:var(--hp-accent,#ffd43b);font:600 .95rem system-ui}.profile-action,.approve-action{margin-top:1rem;padding:.6rem 1rem;border:0;border-radius:.7rem;background:var(--hp-accent,#ffd43b);color:#171717;font:600 1rem system-ui}.profile-action.selected-relationship{background:var(--hp-tertiary-background,#2c2c2e);color:var(--hp-label,#f5f5f7)}.profile-action:disabled,.approve-action:disabled,.plain-action:disabled{opacity:.5}.mutation-error{margin-top:.65rem;color:#ff453a}.request-row{display:flex;min-height:4.75rem;align-items:center;gap:.75rem;padding:.55rem 1rem}.request-actions{display:flex;align-items:center;gap:.35rem}.request-actions .approve-action{margin:0}.request-results li+li,.blocked-account-results li+li{border-top:1px solid var(--hp-separator,#3a3a3c)}
  .dialog-backdrop{position:fixed;z-index:20;inset:0;display:grid;place-items:center;padding:1.5rem;background:#0009}.confirmation-dialog{width:min(28rem,100%);padding:1.4rem;border:1px solid var(--hp-separator,#3a3a3c);border-radius:1rem;background:var(--hp-secondary-background,#1c1c1e);color:var(--hp-label,#f5f5f7);box-shadow:0 1.5rem 4rem #0008}.confirmation-dialog h2{margin:0 0 .75rem;font-size:1.25rem;letter-spacing:0}.confirmation-dialog p{margin:.5rem 0;color:var(--hp-secondary-label,#a1a1a6)}.shared-path-warning{font-weight:600;color:var(--hp-label,#f5f5f7)!important}.shared-path-list{margin:.75rem 0;padding-left:1.25rem}.dialog-actions{display:flex;justify-content:flex-end;gap:.5rem;margin-top:1.25rem}.destructive-action{padding:.6rem .9rem;border:0;border-radius:.65rem;background:#ff453a;color:white;font:600 1rem system-ui}.destructive-action:disabled{opacity:.5}
  button:focus-visible,input:focus-visible{outline:3px solid color-mix(in srgb,var(--hp-accent,#ffd43b) 65%,white);outline-offset:2px}@keyframes spin{to{transform:rotate(360deg)}}@media(prefers-reduced-motion:reduce){.activity-indicator{animation:none;border-right-color:currentColor}}@media(prefers-color-scheme:light){.search-destination,.profile-destination{--hp-label:#1c1c1e;--hp-secondary-label:#6e6e73;--hp-secondary-background:#f2f2f7;--hp-tertiary-background:#e5e5ea;--hp-separator:#c6c6c8}}
</style>
