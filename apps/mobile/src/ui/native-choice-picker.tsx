import { Pressable, StyleSheet, View } from 'react-native';
import { ThemedText as Text } from './primitives';
import { PlatformSymbol } from './platform-symbol';
import { mobileTheme } from './tokens';

export type NativeChoice<Value extends string> = {
  label: string;
  value: Value;
};

export function NativeChoicePicker<Value extends string>({
  accessibilityLabel,
  choices,
  disabled = false,
  label,
  onChange,
  value,
}: {
  accessibilityLabel: string;
  choices: readonly NativeChoice<Value>[];
  disabled?: boolean;
  label: string;
  onChange: (value: Value) => void;
  value: Value;
}) {
  return <View
    accessibilityLabel={accessibilityLabel}
    accessibilityRole="radiogroup"
    style={styles.group}
  >
    <Text style={styles.groupLabel}>{label}</Text>
    {choices.map((choice) => {
      const selected = choice.value === value;
      return <Pressable
        accessibilityRole="radio"
        accessibilityState={{ disabled, checked: selected }}
        disabled={disabled}
        key={choice.value}
        onPress={() => {
          if (!selected) onChange(choice.value);
        }}
        style={({ pressed }) => [
          styles.choice,
          selected ? styles.selected : null,
          pressed ? styles.pressed : null,
        ]}
      >
        {selected ? <PlatformSymbol systemName="checkmark" color={mobileTheme.colors.accent} /> : <View style={{ width: 24 }} />}
        <Text style={styles.choiceLabel}>{choice.label}</Text>
      </Pressable>;
    })}
  </View>;
}

const styles = StyleSheet.create({
  choice: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  choiceLabel: {
    flexShrink: 1,
    color: mobileTheme.colors.accent,
    ...mobileTheme.typography.body,
  },
  group: {
    gap: mobileTheme.spacing.xs,
  },
  groupLabel: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  pressed: {
    opacity: 0.72,
  },
  selected: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
  },
});
