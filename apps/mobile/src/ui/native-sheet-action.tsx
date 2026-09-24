import { Pressable, StyleSheet, Text } from 'react-native';
import { mobileTheme } from './tokens';

export function NativeSheetAction({
  disabled = false,
  label,
  onPress,
}: {
  disabled?: boolean;
  label: string;
  onPress: () => void;
}) {
  return <Pressable
    accessibilityLabel={label}
    accessibilityRole="button"
    accessibilityState={{ disabled }}
    disabled={disabled}
    onPress={onPress}
    style={({ pressed }) => [styles.action, pressed ? styles.pressed : null, disabled ? styles.disabled : null]}
  >
    <Text style={styles.label}>{label}</Text>
  </Pressable>;
}

const styles = StyleSheet.create({
  action: {
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 64,
    paddingHorizontal: mobileTheme.spacing.xs,
  },
  disabled: {
    opacity: 0.4,
  },
  label: {
    color: mobileTheme.colors.accent,
    fontSize: 17,
    fontWeight: '600',
    lineHeight: 22,
  },
  pressed: {
    opacity: 0.65,
  },
});
