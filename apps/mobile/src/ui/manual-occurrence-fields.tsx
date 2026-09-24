import { getLocales } from 'expo-localization';
import { StyleSheet, View } from 'react-native';
import type { ManualActivityFormState, ManualActivityLocalDateTime } from '@hourpaths/client-core';
import { createDeviceTranslator } from '../i18n';
import { ThemedText, ThemedTextInput } from './primitives';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function ManualOccurrenceFields({
  busy,
  form,
  onChange,
}: {
  busy: boolean;
  form: ManualActivityFormState;
  onChange: (patch: Partial<ManualActivityLocalDateTime>) => void;
}) {
  return <View style={styles.fields}>
    <View style={styles.field}>
      <ThemedText style={styles.label}>{i18n.t('activity.date')}</ThemedText>
      <ThemedTextInput
        accessibilityLabel={i18n.t('activity.date')}
        editable={!busy}
        onChangeText={(localDate) => onChange({ localDate })}
        value={form.localDate}
      />
    </View>
    <View style={styles.field}>
      <ThemedText style={styles.label}>{i18n.t('activity.startTime')}</ThemedText>
      <ThemedTextInput
        accessibilityLabel={i18n.t('activity.startTime')}
        editable={!busy}
        onChangeText={(localTime) => onChange({ localTime })}
        value={form.localTime}
      />
    </View>
  </View>;
}

const styles = StyleSheet.create({
  field: {
    gap: mobileTheme.spacing.xs,
  },
  fields: {
    gap: mobileTheme.spacing.sm,
  },
  label: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
});
