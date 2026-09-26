import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../../../src/social-route-recovery';
import { useSocialInteractionRouteAncestry } from '../../../../src/use-social-interaction-route-ancestry';
import { NativeRouteScreen } from '../../../../src/ui/native-route-presentation';
import {
  socialFeedActivityDetailRouteKey,
  useSocialFeedActivityDetailRoutePresentation,
} from '../../../../src/ui/social-feed-activity-detail-route-presentation';
import { SocialFeedActivityDetailView } from '../../../../src/ui/social-feed-activity-detail-view';
import { useSocialRouteRecovery } from '../../../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../../../src/ui/social-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function SocialFeedActivityDetailScreen() {
  const { activityID = '', pathID = '' } = useLocalSearchParams<{ activityID: string; pathID: string }>();
  const presentation = useSocialFeedActivityDetailRoutePresentation(
    socialFeedActivityDetailRouteKey(pathID, activityID),
  );
  const target = { activityID, kind: 'activity' as const, pathID, routeKey: `social:activity:${pathID}:${activityID}` };
  const recovery = useSocialRouteRecovery(target);
  useSocialInteractionRouteAncestry(target);
  useEffect(() => {
    if (presentation || recovery || !pathID || !activityID) return;
    return scheduleSocialRouteBootstrap(target, () => router.replace('/(tabs)/home'));
  }, [activityID, pathID, presentation, recovery]);
  if (!presentation) return <SocialRouteRecoveryView
    i18n={i18n}
    onGoFollowing={recovery?.onGoFollowing}
    onGoHome={recovery?.onGoHome}
    onRetry={recovery?.onRetry}
    state={recovery?.state ?? 'loading'}
  />;

  return <NativeRouteScreen>
    <Stack.Screen options={{ title: presentation.event.path.name }} />
    <SocialFeedActivityDetailView i18n={i18n} state={presentation} />
  </NativeRouteScreen>;
}
