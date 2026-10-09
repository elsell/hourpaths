import { clockMinuteValue } from './native-time-value';
import { useEffect, useState } from 'react';
import type { Translator } from '@hourpaths/i18n';
import { TextInput, View } from 'react-native';
import { ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';
export interface NativeTimeFieldProps { i18n: Translator; label: string; minute: number; disabled?: boolean; onChange(minute: number): void; }
export function NativeTimeField({ label, minute, disabled, onChange }: NativeTimeFieldProps) {
  const formatted = clockMinuteValue(minute);
  const [draft, setDraft] = useState(formatted);
  useEffect(() => setDraft(formatted), [formatted]);
  return <View><Text>{label}</Text><TextInput accessibilityLabel={label} editable={!disabled} value={draft} style={{ color: mobileTheme.colors.text, minHeight: mobileTheme.sizes.minimumTouchTarget }} onChangeText={value => {
    setDraft(value);
    if (/^(?:[01]\d|2[0-3]):[0-5]\d$/.test(value)) { const [hour, minutes] = value.split(':').map(Number); onChange(hour * 60 + minutes); }
  }} onBlur={() => setDraft(formatted)} /></View>;
}
