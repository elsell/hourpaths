import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import type { SettingsJourneyIntent } from '../settings-journey-route-recovery';
import type { SettingsJourneyRecoveryState } from './settings-journey-route-presentation';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeRouteScreen } from './native-route-presentation';
import { ActionButton, ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

export function SettingsJourneyRecoveryView({
  i18n,
  intent: _intent,
  onHome,
  onRetry,
  state,
}: {
  i18n: Translator;
  intent: SettingsJourneyIntent;
  onHome: () => void;
  onRetry: () => void;
  state: SettingsJourneyRecoveryState;
}) {
  return <NativeRouteScreen grouped>
    <View style={styles.state}>
      {state === 'loading' ? <View
        accessibilityLabel={i18n.t('settings.recoveryLoading')}
        accessibilityRole="progressbar"
        style={styles.loading}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
        <Text style={styles.secondary}>{i18n.t('settings.recoveryLoading')}</Text>
      </View> : <NativeContentUnavailable
        description={i18n.t(state === 'offline'
          ? 'settings.recoveryOfflineDescription'
          : 'settings.recoveryUnavailableDescription')}
        systemImage={state === 'offline' ? 'wifi.slash' : 'gearshape'}
        title={i18n.t(state === 'offline'
          ? 'settings.recoveryOfflineHeading'
          : 'settings.recoveryUnavailableHeading')}
      />}
      {state !== 'loading' ? <ActionButton label={i18n.t('common.retry')} onPress={onRetry} variant="secondary" /> : null}
      <ActionButton label={i18n.t('settings.recoveryHome')} onPress={onHome} variant="quiet" />
    </View>
  </NativeRouteScreen>;
}

const styles = StyleSheet.create({
  loading: { alignItems: 'center', gap: mobileTheme.spacing.sm },
  secondary: { color: mobileTheme.colors.textMuted, textAlign: 'center' },
  state: { gap: mobileTheme.spacing.sm, justifyContent: 'center', minHeight: 280 },
});
