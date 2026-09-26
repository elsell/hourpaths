import { DatePickerDialog, Host, TimePickerDialog } from '@expo/ui/jetpack-compose';
import { getCalendars, getLocales } from 'expo-localization';
import { useState } from 'react';
import { StyleSheet, View } from 'react-native';
import type { ManualActivityFormState, ManualActivityLocalDateTime } from '@hourpaths/client-core';
import { createDeviceTranslator } from '../i18n';
import { androidClockPickerValue, localDateFromPicker, localTimeFromAndroidClock, manualOccurrencePickerValue } from './manual-occurrence-values';
import { SettingsNavigationRow, SettingsSection, SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function ManualOccurrenceFields({ busy, form, onChange }: {
  busy: boolean;
  form: ManualActivityFormState;
  onChange: (patch: Partial<ManualActivityLocalDateTime>) => void;
}) {
  const [picker, setPicker] = useState<'date' | 'time' | null>(null);
  const date = manualOccurrencePickerValue(form.localDate, '00:00');
  const wallTime = manualOccurrencePickerValue('2000-01-15', form.localTime);
  const deviceTime = androidClockPickerValue(form.localTime);
  const uses24HourClock = getCalendars()[0]?.uses24hourClock ?? true;
  const dateLabel = date ? i18n.date(date, { timeZone: 'UTC', year: 'numeric', month: 'short', day: 'numeric' }) : form.localDate;
  const timeLabel = wallTime ? i18n.time(wallTime, { timeZone: 'UTC', hour: 'numeric', minute: '2-digit', hour12: !uses24HourClock }) : form.localTime;
  const dismiss = () => setPicker(null);
  return <View>
    <SettingsSection>
      <SettingsNavigationRow accessibilityLabel={i18n.t('activity.date')} disabled={busy}
        label={i18n.t('activity.date')} onPress={() => setPicker('date')}
        value={dateLabel} />
      <SettingsSeparator />
      <SettingsNavigationRow accessibilityLabel={i18n.t('activity.startTime')} disabled={busy}
        label={i18n.t('activity.startTime')} onPress={() => setPicker('time')}
        value={timeLabel} />
    </SettingsSection>
    {!busy && picker ? <Host colorScheme="dark" style={styles.dialogHost}>
      {picker === 'date' ? <DatePickerDialog
        initialDate={date?.toISOString()} showVariantToggle={false}
        confirmButtonLabel={i18n.t('common.done')} dismissButtonLabel={i18n.t('common.cancel')}
        color={mobileTheme.colors.accent}
        elementColors={{ selectedDayContainerColor: mobileTheme.colors.accent, selectedDayContentColor: mobileTheme.colors.accentText, selectedYearContainerColor: mobileTheme.colors.accent, selectedYearContentColor: mobileTheme.colors.accentText }}
        onDismissRequest={dismiss}
        onDateSelected={(value) => {
          dismiss();
          if (!busy) onChange({ localDate: localDateFromPicker(value) });
        }}
      /> : <TimePickerDialog
        initialDate={deviceTime?.toISOString()} is24Hour={uses24HourClock}
        confirmButtonLabel={i18n.t('common.done')} dismissButtonLabel={i18n.t('common.cancel')}
        color={mobileTheme.colors.accent}
        elementColors={{ selectorColor: mobileTheme.colors.accent, clockDialSelectedContentColor: mobileTheme.colors.accentText, timeSelectorSelectedContainerColor: mobileTheme.colors.accent, timeSelectorSelectedContentColor: mobileTheme.colors.accentText }}
        onDismissRequest={dismiss}
        onDateSelected={(value) => {
          dismiss();
          if (!busy) onChange({ localTime: localTimeFromAndroidClock(value) });
        }}
      />}
    </Host> : null}
  </View>;
}
const styles = StyleSheet.create({
  dialogHost: { height: 1, position: 'absolute', width: 1 },
});
