import { useReportAction } from '../../src/ui/report-composer';
import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../src/i18n';
import { usePathRouteAncestry } from '../../src/use-path-route-ancestry';
import { PathHeaderMenu } from '../../src/ui/path-header-menu';
import { NativeRouteRecoveryView } from '../../src/ui/native-route-recovery-view';
import { NativeRouteScreen, retainNativeRoute, useNativeRoutePresentation } from '../../src/ui/native-route-presentation';

const i18n = createDeviceTranslator(getLocales);

export default function PathDetails() {
  const { pathID = '' } = useLocalSearchParams<{ pathID: string }>();
  const presentation = useNativeRoutePresentation(pathID);
  const report = useReportAction({ kind: 'path', id: pathID }, i18n);
  usePathRouteAncestry({ kind: 'path', pathID, routeKey: `path:${pathID}` });
  useEffect(() => retainNativeRoute(pathID), [pathID]);

  if (!presentation) return <>
    <Stack.Screen options={{ title: i18n.t('pathDetails.genericTitle') }} />
    <NativeRouteScreen>
      <NativeRouteRecoveryView onGoHome={() => router.replace('/(tabs)/home')} state="loading" />
    </NativeRouteScreen>
  </>;
  const activePresentation = presentation;

  return <>
    <Stack.Screen options={{ title: activePresentation.title }}>
      <PathHeaderMenu
        accessibilityLabel={i18n.t('pathDetails.moreActions')}
        actions={[...activePresentation.actions, report.action]}
      />
    </Stack.Screen>
    <NativeRouteScreen>
      {activePresentation.content}
      {report.sheet}
    </NativeRouteScreen>
  </>;
}
