import { router } from 'expo-router';
import { useEffect, useRef } from 'react';
import {
  queueNotificationJourneyBootstrap,
  scheduleNotificationJourneyBootstrap,
} from '../src/notification-journey-route-recovery';
import { getLocales } from 'expo-localization';
import { createDeviceTranslator } from '../src/i18n';
import {
  dismissInvitationRoute,
  useNotificationJourneyRecovery,
  useInvitationRoutePresentation,
} from '../src/ui/notification-route-presentation';
import { NotificationJourneyRecoveryView } from '../src/ui/notification-journey-recovery-view';
import { NativeRouteScreen } from '../src/ui/native-route-presentation';
import { useNotificationJourneyRouteAncestry } from '../src/use-notification-journey-route-ancestry';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'invitations', routeKey: 'notifications:invitations' } as const;

export default function Invitations() {
  const presentation = useInvitationRoutePresentation();
  const recovery = useNotificationJourneyRecovery(intent.routeKey);
  const lastRecoverySessionKey = useRef<string | undefined>(undefined);
  if (recovery?.sessionKey) lastRecoverySessionKey.current = recovery.sessionKey;
  useNotificationJourneyRouteAncestry(intent);
  useEffect(() => () => dismissInvitationRoute(), []);
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleNotificationJourneyBootstrap(
      intent,
      () => router.replace('/(tabs)/home'),
      lastRecoverySessionKey.current,
    );
  }, [presentation, recovery]);

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

  return <NativeRouteScreen grouped>
    {presentation.content}
  </NativeRouteScreen>;
}
