import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../../src/i18n';
import { usePathRouteAncestry } from '../../../src/use-path-route-ancestry';
import {
  dismissNativeChildRoute,
  pathNudgeSettingsRouteKey,
  useNativeChildRoutePresentation,
} from '../../../src/ui/native-child-route-presentation';
import { NativeRouteScreen } from '../../../src/ui/native-route-presentation';
import { NativeRouteRecoveryView } from '../../../src/ui/native-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function PathNudgeSettings() {
  const { pathID = '' } = useLocalSearchParams<{ pathID: string }>();
  const routeKey = pathNudgeSettingsRouteKey(pathID);
  const presentation = useNativeChildRoutePresentation(routeKey);
  const navigation = useNavigation();
  usePathRouteAncestry({ kind: 'nudge-settings', pathID, routeKey });
  useEffect(() => () => dismissNativeChildRoute(routeKey), [routeKey]);
  useEffect(() => navigation.addListener('beforeRemove', (event) => {
    if (presentation?.dismissible === false) event.preventDefault();
  }), [navigation, presentation?.dismissible]);

  if (!presentation) return <>
    <Stack.Screen options={{ title: i18n.t('nudge.audience.heading') }} />
    <NativeRouteScreen grouped>
      <NativeRouteRecoveryView
        onGoHome={() => router.replace('/(tabs)/home')}
        state="loading"
      />
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
