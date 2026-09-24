import { useState } from 'react';
import { StyleSheet, View } from 'react-native';
import {
  durationInputForSeconds,
  durationSecondsForInput,
  durationUnits,
  type DurationUnit,
} from './duration-input';
import { NativeSegmentedControl, type NativeSegment } from './native-segmented-control';
import { ThemedText as Text, ThemedTextInput as TextInput } from './primitives';
import { mobileTheme } from './tokens';

export function HumanDurationEditor({
  busy,
  compact = false,
  fallbackUnit,
  inputAccessoryViewID,
  label,
  onChange,
  onSubmitEditing,
  seconds,
  unitLabel,
  unitLabels,
}: {
  busy: boolean;
  compact?: boolean;
  fallbackUnit: DurationUnit;
  inputAccessoryViewID?: string;
  label: string;
  onChange: (seconds: string) => void;
  onSubmitEditing?: () => void;
  seconds: string;
  unitLabel: string;
  unitLabels: Readonly<Record<DurationUnit, string>>;
}) {
  const initial = durationInputForSeconds(seconds, fallbackUnit);
  const [unit, setUnit] = useState<DurationUnit>(initial.unit);
  const [value, setValue] = useState(initial.value);
  const segments: readonly NativeSegment<DurationUnit>[] = durationUnits.map(
    (candidate) => ({ label: unitLabels[candidate], value: candidate }),
  );

  function changeValue(nextValue: string) {
    setValue(nextValue);
    onChange(durationSecondsForInput(nextValue, unit));
  }

  function changeUnit(nextUnit: DurationUnit) {
    setUnit(nextUnit);
    onChange(durationSecondsForInput(value, nextUnit));
  }

  return <View style={styles.field}>
    <Text accessibilityRole="header" style={styles.label}>{label}</Text>
    <TextInput
      accessibilityLabel={label}
      editable={!busy}
      inputAccessoryViewID={inputAccessoryViewID}
      keyboardType="number-pad"
      onChangeText={changeValue}
      onSubmitEditing={onSubmitEditing}
      returnKeyType="done"
      style={compact ? styles.compactInput : undefined}
      value={value}
    />
    <Text style={styles.label}>{unitLabel}</Text>
    <NativeSegmentedControl
      disabled={busy}
      onChange={changeUnit}
      segments={segments}
      value={unit}
    />
  </View>;
}

const styles = StyleSheet.create({
  compactInput: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderWidth: 0,
  },
  field: {
    gap: mobileTheme.spacing.xs,
  },
  label: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
});
