import { getLocales } from 'expo-localization';
import { router, Stack, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { AppState } from 'react-native';
import { createDeviceTranslator } from '../../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../../src/social-route-recovery';
import { FollowingHeaderActions } from '../../../src/ui/following-header-actions';
import { shouldRefreshActiveFollowing } from '../../../src/ui/social-active-following-presentation';
import { useSocialFeedRoutePresentation } from '../../../src/ui/social-feed-route-presentation';
import { LiveActivityViewer } from '../../../src/ui/live-activity-viewer';
import { livePagesFromActivePeople } from '../../../src/live-pages-from-active-people';
import { SocialFeedView } from '../../../src/ui/social-feed-view';
import { useSocialRouteRecovery } from '../../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../../src/ui/social-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function FollowingScreen() {
  const presentation = useSocialFeedRoutePresentation();
  const [liveTimerID, setLiveTimerID] = useState<string | null>(null);
  useEffect(() => {
    if (!presentation || presentation.active.status === 'error') setLiveTimerID(null);
  }, [presentation?.active.status, Boolean(presentation)]);
  useEffect(() => {
    if (!liveTimerID) return;
    const refresh = () => { if (AppState.currentState === 'active') presentationRef.current?.refreshActive(); };
    refresh();
    const timer = setInterval(refresh, 15000);
    const subscription = AppState.addEventListener('change', (state) => { if (state === 'active') refresh(); });
    return () => { clearInterval(timer); subscription.remove(); };
  }, [liveTimerID]);
  const livePages = livePagesFromActivePeople(presentation?.active.items ?? [], i18n);
  const recovery = useSocialRouteRecovery({ kind: 'following', routeKey: 'social:following' });
  const presentationRef = useRef(presentation);
  const focusedRef = useRef(false);
  const appStateRef = useRef(AppState.currentState);

  presentationRef.current = presentation;
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleSocialRouteBootstrap(
      { kind: 'following', routeKey: 'social:following' },
      () => router.replace('/(tabs)/home'),
    );
  }, [presentation, recovery]);

  useEffect(() => {
    if (presentation?.feed.status === 'idle') presentation.loadFeed();
    if (presentation?.active.status === 'idle') presentation.loadActive();
  }, [presentation]);

  useFocusEffect(useCallback(() => {
    focusedRef.current = true;
    const current = presentationRef.current;
    if (current?.active.status !== 'idle') current?.refreshActive();
    if (current?.feed.status === 'ready') current.refresh();
    return () => {
      focusedRef.current = false;
    };
  }, []));

  useEffect(() => {
    const subscription = AppState.addEventListener('change', (nextState) => {
      const shouldRefresh = shouldRefreshActiveFollowing(
        appStateRef.current,
        nextState,
        focusedRef.current,
      );
      appStateRef.current = nextState;
      if (shouldRefresh) presentationRef.current?.refreshActive();
    });
    return () => subscription.remove();
  }, []);

  return <>
    <Stack.Screen />
    <FollowingHeaderActions
      followRequestsAccessibilityLabel={i18n.t('social.followRequestsOpen')}
      followRequestsLabel={i18n.t('social.followRequests')}
      onOpenFollowRequests={() => router.push('/follow-requests')}
      onOpenPeople={() => router.push('/following/people')}
      peopleAccessibilityLabel={i18n.t('social.peopleOpen')}
    />
    {presentation ? <SocialFeedView
      active={presentation.active}
      onOpenActive={(item) => { if (item.timers[0]) setLiveTimerID(item.timers[0].id); }}
      onOpenProfile={(username) => router.push({ pathname: '/profile/[username]', params: { username } })}
      onDismissInteractionNotice={presentation.dismissInteractionNotice}
      i18n={i18n}
      onLoadMoreActive={presentation.loadMoreActive}
      onLoadMore={presentation.loadMore}
      onOpen={presentation.openActivity}
      onOpenComments={presentation.openComments}
      onRemoveReaction={presentation.removeReaction}
      onRefresh={() => {
        presentation.refreshActive();
        presentation.refresh();
      }}
      onRetry={presentation.retry}
      onRetryActive={presentation.retryActive}
      loadReactionPeople={presentation.loadReactionPeople} onSetReaction={presentation.setReaction}
      state={presentation.feed}
    /> : <SocialRouteRecoveryView
      i18n={i18n}
      onGoFollowing={recovery?.onGoFollowing}
      onGoHome={recovery?.onGoHome}
      onRetry={recovery?.onRetry}
      state={recovery?.state ?? 'loading'}
    />}
    {liveTimerID && presentation && presentation.active.status !== 'error' ? <LiveActivityViewer
      pages={livePages} initialTimerId={liveTimerID} translator={i18n}
      onClose={() => setLiveTimerID(null)} onRefresh={presentation.refreshActive}
      onOpenProfile={(personID) => {
        const person = presentation.active.items.find(({ participant }) => participant.userId === personID)?.participant;
        setLiveTimerID(null);
        if (person) router.push({ pathname: '/profile/[username]', params: { username: person.username } });
      }}
    /> : null}
  </>;
}
