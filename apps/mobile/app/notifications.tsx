import { getLocales } from 'expo-localization';
import { router, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useRef } from 'react';
import { AppState } from 'react-native';
import { createDeviceTranslator } from '../src/i18n';
import {
  queueNotificationJourneyBootstrap,
  scheduleNotificationJourneyBootstrap,
} from '../src/notification-journey-route-recovery';
import {
  dismissNotificationRoute,
  useNotificationJourneyRecovery,
  useNotificationRoutePresentation,
} from '../src/ui/notification-route-presentation';
import { NotificationJourneyRecoveryView } from '../src/ui/notification-journey-recovery-view';
import { NativeRouteScreen } from '../src/ui/native-route-presentation';
import { ThemedText as Text } from '../src/ui/primitives';
import { SettingsNavigationRow, SettingsSection } from '../src/ui/settings-list';
import { useNotificationJourneyRouteAncestry } from '../src/use-notification-journey-route-ancestry';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'notifications', routeKey: 'notifications:history' } as const;

export default function Notifications() {
  const presentation = useNotificationRoutePresentation();
  const recovery = useNotificationJourneyRecovery(intent.routeKey);
  const lastRecoverySessionKey = useRef<string | undefined>(undefined);
  if (recovery?.sessionKey) lastRecoverySessionKey.current = recovery.sessionKey;
  useNotificationJourneyRouteAncestry(intent);
  const refreshRef = useRef<(() => void) | undefined>(undefined);
  const hasFocused = useRef(false);
  refreshRef.current = presentation?.refresh;
  useEffect(() => () => dismissNotificationRoute(), []);
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleNotificationJourneyBootstrap(
      intent,
      () => router.replace('/(tabs)/home'),
      lastRecoverySessionKey.current,
    );
  }, [presentation, recovery]);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      if (state === 'active') refreshRef.current?.();
    });
    return () => subscription.remove();
  }, []);
  useFocusEffect(useCallback(() => {
    if (hasFocused.current) refreshRef.current?.();
    else hasFocused.current = true;
  }, []));

  if (!presentation) return <NotificationJourneyRecoveryView
    i18n={i18n}
    intent={intent}
    onHome={() => router.replace('/(tabs)/home')}
    onRetry={recovery?.retry ?? (() => {
      queueNotificationJourneyBootstrap(intent, lastRecoverySessionKey.current);
      router.replace('/(tabs)/home');
    })}
    state={recovery?.state ?? (presentation === undefined ? 'loading' : 'unavailable')}
  />;

  return <>
    <NativeRouteScreen
      grouped
      onRefresh={presentation.refresh}
      refreshing={presentation.refreshing}
    >
      {presentation.busy ? <Text accessibilityLiveRegion="polite" accessibilityRole="progressbar">
        {i18n.t('notification.updating')}
      </Text> : null}
      <SettingsSection>
        <SettingsNavigationRow
          accessibilityLabel={i18n.t('pathInvitation.pendingHeading')}
          label={i18n.t('pathInvitation.pendingHeading')}
          onPress={presentation.openInvitations}
          value={presentation.invitationCount === undefined
            ? undefined
            : i18n.number(presentation.invitationCount)}
        />
      </SettingsSection>
      {presentation.content}
    </NativeRouteScreen>
  </>;
}
