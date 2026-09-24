import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativePrimaryButton } from './native-primary-button';
import type { SocialRouteRecoveryState } from './social-route-recovery-presentation';
import { ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

export function SocialRouteRecoveryView({
  i18n,
  onGoFollowing,
  onGoHome,
  onRetry,
  state,
}: {
  i18n: Translator;
  onGoFollowing?: () => void;
  onGoHome?: () => void;
  onRetry?: () => void;
  state: SocialRouteRecoveryState;
}) {
  if (state === 'loading') return <View
    accessibilityLabel={i18n.t('common.loading')}
    accessibilityRole="progressbar"
    style={styles.state}
  >
    <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
    <Text style={styles.status}>{i18n.t('common.loading')}</Text>
  </View>;
  return <View style={styles.state}>
    <NativeContentUnavailable
      description={i18n.t(state === 'offline'
        ? 'errors.temporarilyUnavailable'
        : 'social.feedUnavailableDescription')}
      systemImage={state === 'offline' ? 'wifi.exclamationmark' : 'person.2.slash'}
      title={i18n.t('social.feedUnavailableHeading')}
    />
    {onRetry ? <NativePrimaryButton
      label={i18n.t('common.retry')}
      onPress={onRetry}
      systemImage="arrow.clockwise"
      variant="plain"
    /> : null}
    {onGoFollowing ? <NativePrimaryButton
      label={i18n.t('social.following')}
      onPress={onGoFollowing}
      systemImage="person.2"
      variant="plain"
    /> : null}
    {onGoHome ? <NativePrimaryButton
      label={i18n.t('home.heading')}
      onPress={onGoHome}
      systemImage="house"
      variant="plain"
    /> : null}
  </View>;
}

const styles = StyleSheet.create({
  state: { flex: 1, gap: mobileTheme.spacing.sm, justifyContent: 'center', padding: mobileTheme.spacing.lg },
  status: { color: mobileTheme.colors.textMuted, textAlign: 'center' },
});
