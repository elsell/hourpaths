import { getLocales } from 'expo-localization';
import { useEffect, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { durationParts, secondsFromDurationParts, type DurationParts } from './duration-parts';
import type { DurationUnit } from './duration-input';
import { NativeButton } from './native-button';
import { ThemedText as Text, ThemedTextInput as TextInput } from './primitives';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function HumanDurationEditor({ busy, compact = false, inputAccessoryViewID, label, onChange, onSubmitEditing, seconds, unitLabels }: {
  busy: boolean;
  compact?: boolean;
  inputAccessoryViewID?: string;
  label: string;
  onChange: (seconds: string) => void;
  onSubmitEditing?: () => void;
  seconds: string;
  unitLabels: Readonly<Record<DurationUnit, string>>;
}) {
  const [parts, setParts] = useState(() => durationParts(seconds));
  const [showSeconds, setShowSeconds] = useState(() => Boolean(durationParts(seconds).seconds));
  const lastEmitted = useRef(seconds);
  useEffect(() => {
    if (seconds === lastEmitted.current) return;
    const next = durationParts(seconds);
    lastEmitted.current = seconds;
    setParts(next);
    if (next.seconds) setShowSeconds(true);
  }, [seconds]);

  function change(unit: keyof DurationParts, value: string) {
    const next = { ...parts, [unit]: value };
    setParts(next);
    lastEmitted.current = secondsFromDurationParts(next);
    onChange(lastEmitted.current);
  }

  return <View style={styles.field}>
    <Text accessibilityRole="header" style={styles.label}>{label}</Text>
    <View style={styles.units}>
      {(['hours', 'minutes', ...(showSeconds ? ['seconds'] : [])] as DurationUnit[]).map((unit) => <View key={unit} style={styles.unit}>
        <Text style={styles.label}>{unitLabels[unit]}</Text>
        <TextInput
          accessibilityLabel={i18n.t('duration.fieldLabel', { label, unit: unitLabels[unit] })}
          editable={!busy}
          inputAccessoryViewID={inputAccessoryViewID}
          keyboardType="number-pad"
          onChangeText={(value) => change(unit, value)}
          onSubmitEditing={onSubmitEditing}
          placeholder={i18n.number(0)}
          returnKeyType="done"
          style={[styles.input, compact && styles.compactInput]}
          value={parts[unit]}
        />
      </View>)}
    </View>
    {!showSeconds ? <NativeButton disabled={busy} label={i18n.t('duration.showSeconds')} onPress={() => setShowSeconds(true)} variant="quiet" /> : null}
  </View>;
}
const styles = StyleSheet.create({
  compactInput: { borderWidth: 0 },
  field: { gap: mobileTheme.spacing.xs },
  units: { flexDirection: 'row', flexWrap: 'wrap', gap: mobileTheme.spacing.sm },
  unit: { flexBasis: 100, flexGrow: 1, gap: mobileTheme.spacing.xs },
  input: { fontVariant: ['tabular-nums'] },
  label: { color: mobileTheme.colors.textMuted, ...mobileTheme.typography.caption },
});
