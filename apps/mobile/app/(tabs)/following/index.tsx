import { getLocales } from 'expo-localization';
import { router, Stack, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useRef } from 'react';
import { AppState } from 'react-native';
import { createDeviceTranslator } from '../../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../../src/social-route-recovery';
import { FollowingHeaderActions } from '../../../src/ui/following-header-actions';
import { shouldRefreshActiveFollowing } from '../../../src/ui/social-active-following-presentation';
import { useSocialFeedRoutePresentation } from '../../../src/ui/social-feed-route-presentation';
import { SocialFeedView } from '../../../src/ui/social-feed-view';
import { useSocialRouteRecovery } from '../../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../../src/ui/social-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function FollowingScreen() {
  const presentation = useSocialFeedRoutePresentation();
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
      onSetReaction={presentation.setReaction}
      state={presentation.feed}
    /> : <SocialRouteRecoveryView
      i18n={i18n}
      onGoFollowing={recovery?.onGoFollowing}
      onGoHome={recovery?.onGoHome}
      onRetry={recovery?.onRetry}
      state={recovery?.state ?? 'loading'}
    />}
  </>;
}
