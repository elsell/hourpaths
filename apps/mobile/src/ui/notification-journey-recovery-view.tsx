import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import type { NotificationJourneyIntent } from '../notification-journey-route-recovery';
import type { NotificationJourneyRecoveryState } from './notification-route-presentation';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeRouteScreen } from './native-route-presentation';
import { ActionButton, ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

export function NotificationJourneyRecoveryView({
  i18n,
  intent,
  onHome,
  onRetry,
  state,
}: {
  i18n: Translator;
  intent: NotificationJourneyIntent;
  onHome: () => void;
  onRetry: () => void;
  state: NotificationJourneyRecoveryState;
}) {
  const unavailableTitle = intent.kind === 'invitations'
    ? i18n.t('pathInvitation.unavailableHeading')
    : intent.kind === 'notification-settings'
      ? i18n.t('notification.settings.heading')
      : i18n.t('notification.unavailableHeading');

  return <NativeRouteScreen grouped>
    <View style={styles.state}>
      {state === 'loading' ? <View
        accessibilityLabel={i18n.t('notification.recoveryLoading')}
        accessibilityRole="progressbar"
        style={styles.loading}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
        <Text style={styles.secondary}>{i18n.t('notification.recoveryLoading')}</Text>
      </View> : <NativeContentUnavailable
        description={i18n.t(state === 'offline'
          ? 'notification.recoveryOfflineDescription'
          : 'notification.recoveryUnavailableDescription')}
        systemImage={state === 'offline' ? 'wifi.slash' : 'bell.slash'}
        title={state === 'offline' ? i18n.t('notification.recoveryOfflineHeading') : unavailableTitle}
      />}
      {state !== 'loading' ? <ActionButton
        label={i18n.t('common.retry')}
        onPress={onRetry}
        variant="secondary"
      /> : null}
      <ActionButton label={i18n.t('notification.recoveryHome')} onPress={onHome} variant="quiet" />
    </View>
  </NativeRouteScreen>;
}

const styles = StyleSheet.create({
  loading: { alignItems: 'center', gap: mobileTheme.spacing.sm },
  secondary: { color: mobileTheme.colors.textMuted, textAlign: 'center' },
  state: { gap: mobileTheme.spacing.sm, justifyContent: 'center', minHeight: 280 },
});
