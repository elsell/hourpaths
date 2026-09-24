import type { Translator } from '@hourpaths/i18n';
import { ActivityDetailView } from './activity-detail-view';
import type { SocialFeedActivityDetailPresentation } from './social-feed-activity-detail-route-presentation';

function noop() {}

export function SocialFeedActivityDetailView({
  i18n,
  state,
}: {
  i18n: Translator;
  state: SocialFeedActivityDetailPresentation;
}) {
  return <ActivityDetailView
    activity={state.detail}
    archived={true}
    busy={state.busy}
    deletionBusy={false}
    hasMoreRevisions={Boolean(state.nextCursor)}
    manualBusy={false}
    onDelete={noop}
    onEdit={noop}
    onLoadMoreRevisions={state.loadMoreRevisions}
    onRetry={noop}
    onRetryDeletion={noop}
    onRetryRevisions={state.retryRevisions}
    participantLabel={state.event.participant.displayName}
    profileID={state.profileID}
    revisionErrorText={state.errorKey ? i18n.t(state.errorKey) : undefined}
    revisions={state.revisions}
  />;
}
