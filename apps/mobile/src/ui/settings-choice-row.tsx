import { Pressable, StyleSheet, Text, View } from 'react-native';
import { SettingsIcon } from './settings-icon';
import { mobileTheme } from './tokens';

export type SettingsChoiceRowProps = { label: string; selected: boolean; disabled?: boolean; onPress: () => void };

export function SettingsChoiceRow({ label, selected, disabled = false, onPress }: SettingsChoiceRowProps) {
  return <Pressable accessibilityLabel={label} accessibilityRole="radio" accessibilityState={{ disabled, checked: selected }}
    disabled={disabled} onPress={onPress} style={({ pressed }) => [styles.row, pressed && styles.pressed]}>
    <Text style={styles.label}>{label}</Text>
    <View style={styles.check}>{selected ? <SettingsIcon systemName="checkmark" variant="disclosure" /> : null}</View>
  </Pressable>;
}
const styles = StyleSheet.create({
  row: { alignItems: 'center', flexDirection: 'row', minHeight: 52, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  label: { color: mobileTheme.colors.text, flex: 1, fontSize: 17, lineHeight: 22 },
  check: { alignItems: 'center', minWidth: 24 },
  pressed: { backgroundColor: mobileTheme.colors.surfaceRaised },
});
