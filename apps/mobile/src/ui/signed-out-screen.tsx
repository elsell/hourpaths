import { ActivityIndicator, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { getLocales } from 'expo-localization';
import { NativePrimaryButton } from './native-primary-button';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { ScreenHeader, StatusBanner, ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function SignedOutScreen({
  errorText,
  onSignIn,
  onRetry,
  providerBusy,
  providerDiscoveryFailed,
  providerReady,
  sessionExpired,
}: {
  errorText?: string;
  onSignIn: () => void;
  onRetry: () => void;
  providerBusy: boolean;
  providerDiscoveryFailed: boolean;
  providerReady: boolean;
  sessionExpired: boolean;
}) {
  const { fontScale, width } = useWindowDimensions();
  const accessibilityLayout = needsCompactVerticalLayout(width, fontScale);
  const recoveryErrorText = providerDiscoveryFailed
    ? i18n.t(sessionExpired ? 'auth.expiredProviderUnavailable' : 'auth.providerUnavailable')
    : errorText;
  const primaryAction = <View style={styles.primaryAction}>
    {providerDiscoveryFailed
      ? <NativePrimaryButton fullWidth label={i18n.t('common.retry')} onPress={onRetry} />
      : <NativePrimaryButton
        disabled={!providerReady || providerBusy}
        fullWidth
        label={i18n.t('auth.signIn')}
        onPress={onSignIn}
      />}
  </View>;
  return <ScrollView
    alwaysBounceVertical={false}
    contentInsetAdjustmentBehavior="never"
    contentContainerStyle={[styles.content, accessibilityLayout && styles.accessibilityContent]}
  >
    <View style={styles.hero}>
      <ScreenHeader
        compact={accessibilityLayout}
        eyebrow={accessibilityLayout ? undefined : i18n.t('app.title')}
        title={i18n.t(accessibilityLayout ? 'app.title' : 'auth.welcomeHeading')}
      />
      {accessibilityLayout ? null : <Text style={styles.explanation}>{i18n.t('auth.welcomeBody')}</Text>}
    </View>

    <View style={[styles.actionArea, accessibilityLayout && styles.accessibilityActionArea]}>
      {accessibilityLayout ? primaryAction : null}
      {recoveryErrorText ? <StatusBanner text={recoveryErrorText} tone="error" /> : null}
      {!providerDiscoveryFailed && (!providerReady || providerBusy) ? <View
        accessibilityLabel={i18n.t(providerBusy ? 'auth.signingIn' : 'auth.preparingSignIn')}
        accessibilityLiveRegion="polite"
        accessibilityRole="progressbar"
        style={styles.preparing}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} />
        <Text style={styles.preparingText}>{i18n.t(providerBusy ? 'auth.signingIn' : 'auth.preparingSignIn')}</Text>
      </View> : null}
      {accessibilityLayout ? null : primaryAction}
      <Text style={styles.help}>{i18n.t('auth.signInHelp')}</Text>
    </View>
  </ScrollView>;
}

const styles = StyleSheet.create({
  accessibilityActionArea: {
    paddingTop: 0,
  },
  accessibilityContent: {
    gap: mobileTheme.spacing.lg,
    justifyContent: 'flex-start',
  },
  actionArea: {
    gap: mobileTheme.spacing.sm,
    paddingTop: mobileTheme.spacing.xl,
  },
  content: {
    flexGrow: 1,
    justifyContent: 'space-between',
    paddingBottom: mobileTheme.spacing.lg,
    paddingTop: mobileTheme.spacing.sm,
  },
  explanation: {
    color: mobileTheme.colors.textMuted,
    maxWidth: 340,
  },
  help: {
    color: mobileTheme.colors.textMuted,
    fontSize: mobileTheme.typography.caption.fontSize,
    lineHeight: mobileTheme.typography.caption.lineHeight,
    textAlign: 'center',
  },
  hero: {
    gap: mobileTheme.spacing.md,
  },
  preparing: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  preparingText: {
    color: mobileTheme.colors.textMuted,
    flexShrink: 1,
  },
  primaryAction: {
    width: '100%',
  },
});
