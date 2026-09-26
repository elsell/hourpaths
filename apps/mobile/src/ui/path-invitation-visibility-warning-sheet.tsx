import type { PathInvitationAcceptanceReview } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { StyleSheet, View } from 'react-native';
import {
  NativeSheet,
  Surface,
  ThemedText,
} from './primitives';
import { NativePrimaryButton } from './native-primary-button';
import { mobileTheme } from './tokens';

type ConfirmationReview = Extract<
  PathInvitationAcceptanceReview,
  { kind: 'confirmation-required' }
>;

export function PathInvitationVisibilityWarningSheet({
  busy,
  onCancel,
  onConfirm,
  review,
  translator,
}: {
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  review: ConfirmationReview;
  translator: Translator;
}) {
  return <NativeSheet
    dismissible={!busy}
    leadingAction={{ disabled: busy, label: translator.t('pathInvitation.visibilityWarning.cancel'), onPress: onCancel }}
    onRequestClose={onCancel}
    title={translator.t('pathInvitation.visibilityWarning.heading')}
    visible>
    <View accessibilityRole="alert">
      <Surface>
        <ThemedText style={styles.audience}>
          {translator.t(review.warning.pathVisibility === 'public'
            ? 'pathInvitation.visibilityWarning.audience.public'
            : 'pathInvitation.visibilityWarning.audience.followers')}
        </ThemedText>
        <ThemedText>
          {translator.t('pathInvitation.visibilityWarning.exposure')}
        </ThemedText>
        <ThemedText>
          {translator.t('pathInvitation.visibilityWarning.privacyScope')}
        </ThemedText>
        {review.warning.hasRetainedActivity ? <ThemedText>
          {translator.t('pathInvitation.visibilityWarning.retainedActivity')}
        </ThemedText> : null}
      </Surface>
    </View>
    <NativePrimaryButton
      disabled={busy}
      label={translator.t('pathInvitation.visibilityWarning.confirm')}
      onPress={onConfirm}
    />
  </NativeSheet>;
}

const styles = StyleSheet.create({
  audience: {
    color: mobileTheme.colors.accent,
    ...mobileTheme.typography.subheading,
  },
});
