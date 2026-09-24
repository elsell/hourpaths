import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../src/i18n';
import { scheduleSocialRouteBootstrap } from '../src/social-route-recovery';
import { FollowRequestListView } from '../src/ui/follow-request-list-view';
import { NativeRouteScreen } from '../src/ui/native-route-presentation';
import { useSocialProfileRoutePresentation } from '../src/ui/social-profile-route-presentation';
import { useSocialRouteRecovery } from '../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../src/ui/social-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function FollowRequestsScreen() {
  const presentation = useSocialProfileRoutePresentation();
  const recovery = useSocialRouteRecovery({ kind: 'follow-requests', routeKey: 'social:follow-requests' });
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleSocialRouteBootstrap(
      { kind: 'follow-requests', routeKey: 'social:follow-requests' },
      () => router.replace('/(tabs)/home'),
    );
  }, [presentation, recovery]);
  useEffect(() => {
    if (presentation?.followRequests.status === 'idle') presentation.loadFollowRequests();
  }, [presentation?.followRequests.status, presentation?.loadFollowRequests]);
  if (!presentation) return <SocialRouteRecoveryView
    i18n={i18n}
    onGoFollowing={recovery?.onGoFollowing}
    onGoHome={recovery?.onGoHome}
    onRetry={recovery?.onRetry}
    state={recovery?.state ?? 'loading'}
  />;
  return <NativeRouteScreen
    onRefresh={() => presentation.loadFollowRequests(true)}
    refreshing={presentation.followRequests.refreshing}
  >
    <FollowRequestListView
      i18n={i18n}
      onLoadMore={presentation.loadMoreFollowRequests}
      onRefresh={() => presentation.loadFollowRequests(true)}
      onReview={presentation.reviewFollowRequest}
      state={presentation.followRequests}
    />
  </NativeRouteScreen>;
}
