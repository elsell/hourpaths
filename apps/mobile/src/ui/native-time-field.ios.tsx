import { clockMinuteValue } from './native-time-value';
import { DatePicker } from '@expo/ui/swift-ui';
import { datePickerStyle, disabled, environment, tint } from '@expo/ui/swift-ui/modifiers';
import { getLocales } from 'expo-localization';
import { NativeHost } from './native-host';
import type { NativeTimeFieldProps } from './native-time-field';
import { localTimeFromPicker, manualOccurrencePickerValue } from './manual-occurrence-values';
import { mobileTheme } from './tokens';
const pickerTimeZone = 'utc';
export function NativeTimeField({ i18n, label, minute, disabled: busy = false, onChange }: NativeTimeFieldProps) {
  const value = clockMinuteValue(minute);
  return <NativeHost matchContents={{ vertical: true }} style={{ minHeight: mobileTheme.sizes.minimumTouchTarget, width: '100%' }}>
    <DatePicker title={label} displayedComponents={['hourAndMinute']} selection={manualOccurrencePickerValue('2000-01-15', value) ?? undefined}
      modifiers={[datePickerStyle('compact'), environment('colorScheme', 'dark'), environment('locale', getLocales()[0]?.languageTag ?? i18n.locale), environment('timeZone', pickerTimeZone), tint(mobileTheme.colors.accent), disabled(busy)]}
      onDateChange={date => { if (!busy) { const [hour, minutes] = localTimeFromPicker(date).split(':').map(Number); onChange(hour * 60 + minutes); } }} />
  </NativeHost>;
}
