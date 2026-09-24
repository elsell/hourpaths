import { getLocales } from 'expo-localization';
import { ScrollView, StyleSheet, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { NativePrimaryButton } from './native-primary-button';
import {
  ActionButton,
  ScreenHeader,
  StatusBanner,
  Surface,
  ThemedText as Text,
} from './primitives';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function DuplicateEmailRecoveryScreen({
  declining,
  errorText,
  onContinue,
  onReturnToSignIn,
}: {
  declining: boolean;
  errorText?: string;
  onContinue: () => void;
  onReturnToSignIn: () => void;
}) {
  return <ScrollView
    automaticallyAdjustContentInsets
    contentContainerStyle={styles.content}
    contentInsetAdjustmentBehavior="automatic"
    style={styles.screen}
  >
    <ScreenHeader title={i18n.t('duplicateEmailRecovery.heading')} />
    <Surface>
      <Text style={styles.notice}>{i18n.t('duplicateEmailRecovery.notice')}</Text>
    </Surface>

    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
    {declining ? <StatusBanner text={i18n.t('duplicateEmailRecovery.declining')} /> : null}

    <NativePrimaryButton
      disabled={declining}
      label={i18n.t('duplicateEmailRecovery.returnToSignIn')}
      onPress={onReturnToSignIn}
      systemImage="rectangle.portrait.and.arrow.right"
    />

    <View style={styles.separateChoice}>
      <Text style={styles.secondary}>{i18n.t('duplicateEmailRecovery.separateExplanation')}</Text>
      <ActionButton
        busy={declining}
        disabled={declining}
        label={i18n.t('duplicateEmailRecovery.decline')}
        onPress={onContinue}
        variant="secondary"
      />
    </View>
  </ScrollView>;
}

const styles = StyleSheet.create({
  content: {
    gap: mobileTheme.spacing.lg,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  notice: {
    ...mobileTheme.typography.body,
  },
  screen: {
    flex: 1,
  },
  secondary: {
    color: mobileTheme.colors.textMuted,
  },
  separateChoice: {
    gap: mobileTheme.spacing.sm,
  },
});
