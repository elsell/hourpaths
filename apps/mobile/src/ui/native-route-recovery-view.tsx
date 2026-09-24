import { getLocales } from 'expo-localization';
import { StyleSheet, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativePrimaryButton } from './native-primary-button';
import { StatusBanner } from './primitives';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function NativeRouteRecoveryView({
  onGoHome,
  onRetry,
  state,
}: {
  onGoHome?: () => void;
  onRetry?: () => void;
  state: 'loading' | 'offline' | 'unavailable';
}) {
  if (state === 'loading') {
    return <View style={styles.stack}>
      <StatusBanner text={i18n.t('common.loading')} tone="loading" />
      {onGoHome ? <NativePrimaryButton
        label={i18n.t('pathDetails.back')}
        onPress={onGoHome}
        systemImage="house"
        variant="plain"
      /> : null}
    </View>;
  }
  return <View style={styles.stack}>
    <NativeContentUnavailable
      description={i18n.t(state === 'offline'
        ? 'errors.temporarilyUnavailable'
        : 'pathDetails.unavailableDescription')}
      systemImage="exclamationmark.triangle"
      title={i18n.t('pathDetails.unavailableTitle')}
    />
    {onRetry ? <NativePrimaryButton
      label={i18n.t('common.retry')}
      onPress={onRetry}
      systemImage="arrow.clockwise"
      variant="plain"
    /> : null}
    {onGoHome ? <NativePrimaryButton
      label={i18n.t('pathDetails.back')}
      onPress={onGoHome}
      systemImage="house"
      variant="plain"
    /> : null}
  </View>;
}

const styles = StyleSheet.create({
  stack: {
    gap: mobileTheme.spacing.sm,
  },
});
