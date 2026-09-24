import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../../../src/i18n';
import { usePathRouteAncestry } from '../../../../src/use-path-route-ancestry';
import {
  dismissNativeChildRoute,
  pathMembersRouteKey,
  useNativeChildRoutePresentation,
} from '../../../../src/ui/native-child-route-presentation';
import { NativeRouteScreen } from '../../../../src/ui/native-route-presentation';
import { NativeRouteRecoveryView } from '../../../../src/ui/native-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function PathMembers() {
  const { pathID = '' } = useLocalSearchParams<{ pathID: string }>();
  const routeKey = pathMembersRouteKey(pathID);
  const presentation = useNativeChildRoutePresentation(routeKey);
  usePathRouteAncestry({ kind: 'members', pathID, routeKey: `path:${pathID}:members` });
  useEffect(() => () => dismissNativeChildRoute(routeKey), [routeKey]);

  if (!presentation) return <>
    <Stack.Screen options={{ title: i18n.t('pathMembers.heading') }} />
    <NativeRouteScreen grouped>
      <NativeRouteRecoveryView onGoHome={() => router.replace('/(tabs)/home')} state="loading" />
    </NativeRouteScreen>
  </>;
  return <NativeRouteScreen
    grouped={presentation.grouped}
    onRefresh={presentation.onRefresh}
    refreshing={presentation.refreshing}
  >
    <Stack.Screen options={{ title: presentation.title }} />
    {presentation.content}
  </NativeRouteScreen>;
}
