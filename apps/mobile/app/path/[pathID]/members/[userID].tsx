import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../../../src/i18n';
import { usePathRouteAncestry } from '../../../../src/use-path-route-ancestry';
import {
  dismissNativeChildRoute,
  pathMemberRemovalRouteKey,
  useNativeChildRoutePresentation,
} from '../../../../src/ui/native-child-route-presentation';
import { NativeRouteScreen } from '../../../../src/ui/native-route-presentation';
import { NativeRouteRecoveryView } from '../../../../src/ui/native-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function PathMemberRemoval() {
  const { pathID = '', userID = '' } = useLocalSearchParams<{ pathID: string; userID: string }>();
  const routeKey = pathMemberRemovalRouteKey(pathID, userID);
  const presentation = useNativeChildRoutePresentation(routeKey);
  const navigation = useNavigation();
  usePathRouteAncestry({ kind: 'member', pathID, routeKey: `path:${pathID}:member:${userID}`, userID });
  useEffect(() => () => dismissNativeChildRoute(routeKey), [routeKey]);
  useEffect(() => navigation.addListener('beforeRemove', (event) => {
    if (presentation?.dismissible === false) event.preventDefault();
  }), [navigation, presentation?.dismissible]);

  if (!presentation) return <>
    <Stack.Screen options={{ title: i18n.t('pathMembers.memberHeading') }} />
    <NativeRouteScreen grouped>
      <NativeRouteRecoveryView onGoHome={() => router.replace('/(tabs)/home')} state="loading" />
    </NativeRouteScreen>
  </>;
  return <NativeRouteScreen grouped>
    <Stack.Screen options={{
      gestureEnabled: presentation.dismissible,
      headerBackVisible: presentation.dismissible,
      title: presentation.title,
    }} />
    {presentation.content}
  </NativeRouteScreen>;
}
