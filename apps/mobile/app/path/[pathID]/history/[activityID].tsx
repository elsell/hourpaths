import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../../../src/i18n';
import { usePathRouteAncestry } from '../../../../src/use-path-route-ancestry';
import {
  activityDetailRouteKey,
  dismissNativeChildRoute,
  useNativeChildRoutePresentation,
} from '../../../../src/ui/native-child-route-presentation';
import { NativeRouteScreen } from '../../../../src/ui/native-route-presentation';
import { NativeRouteRecoveryView } from '../../../../src/ui/native-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function ActivityDetail() {
  const { activityID = '', pathID = '' } = useLocalSearchParams<{ activityID: string; pathID: string }>();
  const routeKey = activityDetailRouteKey(pathID, activityID);
  const presentation = useNativeChildRoutePresentation(routeKey);
  const navigation = useNavigation();
  usePathRouteAncestry({ activityID, kind: 'activity', pathID, routeKey });
  useEffect(() => () => dismissNativeChildRoute(routeKey), [routeKey]);
  useEffect(() => navigation.addListener('beforeRemove', (event) => {
    if (presentation?.dismissible === false) event.preventDefault();
  }), [navigation, presentation?.dismissible]);

  if (!presentation) return <>
    <Stack.Screen options={{ title: i18n.t('pathDetails.activityHeading') }} />
    <NativeRouteScreen><NativeRouteRecoveryView onGoHome={() => router.replace('/(tabs)/home')} state="loading" /></NativeRouteScreen>
  </>;

  return <NativeRouteScreen>
    <Stack.Screen options={{
      gestureEnabled: presentation.dismissible,
      headerBackVisible: presentation.dismissible,
      title: presentation.title,
    }} />
    {presentation.content}
  </NativeRouteScreen>;
}
