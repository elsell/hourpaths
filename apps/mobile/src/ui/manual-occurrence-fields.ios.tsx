import { NativeHost as Host } from './native-host';
import { getLocales } from 'expo-localization';
import { DatePicker, } from '@expo/ui/swift-ui';
import {
  datePickerStyle,
  disabled,
  environment,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import type { ManualActivityFormState, ManualActivityLocalDateTime } from '@hourpaths/client-core';
import { createDeviceTranslator } from '../i18n';
import {
  localDateFromPicker,
  localTimeFromPicker,
  manualOccurrencePickerValue,
} from './manual-occurrence-values';
import { mobileTheme } from './tokens';

const locales = getLocales();
const i18n = createDeviceTranslator(() => locales);
const nativeLocale = locales[0]?.languageTag ?? i18n.locale;

export function ManualOccurrenceFields({
  busy,
  form,
  onChange,
}: {
  busy: boolean;
  form: ManualActivityFormState;
  onChange: (patch: Partial<ManualActivityLocalDateTime>) => void;
}) {
  const selection = manualOccurrencePickerValue(form.localDate, form.localTime);
  const modifiers = [
    datePickerStyle('compact'),
    environment('colorScheme', 'dark'),
    environment('locale', nativeLocale),
    environment('timeZone', 'UTC'),
    tint(mobileTheme.colors.accent),
    disabled(busy),
  ];

  return <View style={styles.fields}>
    <Host matchContents={{ vertical: true }} style={styles.host}>
      <DatePicker
        displayedComponents={['date']}
        modifiers={modifiers}
        onDateChange={(value) => onChange({ localDate: localDateFromPicker(value) })}
        selection={selection ?? undefined}
        title={i18n.t('activity.date')}
      />
    </Host>
    <Host matchContents={{ vertical: true }} style={styles.host}>
      <DatePicker
        displayedComponents={['hourAndMinute']}
        modifiers={modifiers}
        onDateChange={(value) => onChange({ localTime: localTimeFromPicker(value) })}
        selection={selection ?? undefined}
        title={i18n.t('activity.startTime')}
      />
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  fields: {
    gap: mobileTheme.spacing.sm,
  },
  host: {
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    width: '100%',
  },
});
