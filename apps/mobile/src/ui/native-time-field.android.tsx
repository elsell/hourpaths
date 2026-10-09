import { clockMinuteValue } from './native-time-value';
import { Host, TimePickerDialog } from '@expo/ui/jetpack-compose';
import { getCalendars } from 'expo-localization';
import { useState } from 'react';
import { View } from 'react-native';
import type { NativeTimeFieldProps } from './native-time-field';
import { androidClockPickerValue, localTimeFromAndroidClock, manualOccurrencePickerValue } from './manual-occurrence-values';
import { SettingsNavigationRow } from './settings-list';
import { mobileTheme } from './tokens';
const pickerTimeZone = 'utc';
export function NativeTimeField({ i18n, label, minute, disabled = false, onChange }: NativeTimeFieldProps) {
  const [open, setOpen] = useState(false);
  const time = clockMinuteValue(minute);
  const uses24Hour = getCalendars()[0]?.uses24hourClock ?? true;
  const date = manualOccurrencePickerValue('2000-01-15', time)!;
  return <View><SettingsNavigationRow accessibilityLabel={label} label={label} value={i18n.time(date, { timeZone: pickerTimeZone, hour: 'numeric', minute: '2-digit', hour12: !uses24Hour })} disabled={disabled} onPress={() => setOpen(true)} />
    {open && !disabled && <Host colorScheme="dark" style={{ height: 1, position: 'absolute', width: 1 }}><TimePickerDialog initialDate={androidClockPickerValue(time)?.toISOString()} is24Hour={uses24Hour} confirmButtonLabel={i18n.t('common.done')} dismissButtonLabel={i18n.t('common.cancel')} color={mobileTheme.colors.accent}
      elementColors={{ selectorColor: mobileTheme.colors.accent, clockDialSelectedContentColor: mobileTheme.colors.accentText, timeSelectorSelectedContainerColor: mobileTheme.colors.accent, timeSelectorSelectedContentColor: mobileTheme.colors.accentText }}
      onDismissRequest={() => setOpen(false)} onDateSelected={date => { setOpen(false); if (!disabled) { const [hour, minutes] = localTimeFromAndroidClock(date).split(':').map(Number); onChange(hour * 60 + minutes); } }} /></Host>}
  </View>;
}
