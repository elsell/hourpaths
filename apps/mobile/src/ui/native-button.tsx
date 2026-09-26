import { ActivityIndicator, Pressable, StyleSheet, Text } from 'react-native';
import type { NativeButtonProps } from './native-button.types';
import { mobileTheme } from './tokens';

export function NativeButton({ accessibilityLabel, busy = false, disabled = false, fullWidth = false, label, onPress, selected, variant = 'primary' }: NativeButtonProps) {
  const unavailable = disabled || busy;
  return <Pressable
    accessibilityLabel={accessibilityLabel ?? label}
    accessibilityRole="button"
    accessibilityState={{ busy, disabled: unavailable, selected }}
    android_ripple={{ color: mobileTheme.colors.surfacePressed }}
    disabled={unavailable}
    onPress={onPress}
    style={({ pressed }) => [styles.button, styles[variant], fullWidth && styles.fullWidth, (pressed || unavailable) && styles.dimmed]}
  >
    {busy ? <ActivityIndicator color={variant === 'primary' ? mobileTheme.colors.accentText : mobileTheme.colors.accent} /> : null}
    <Text style={[styles.label, variant === 'primary' && styles.primaryLabel, variant === 'danger' && styles.dangerLabel]}>{label}</Text>
  </Pressable>;
}
const styles = StyleSheet.create({
  button: { alignItems: 'center', alignSelf: 'center', borderRadius: mobileTheme.radii.pill, flexDirection: 'row', gap: mobileTheme.spacing.xs, justifyContent: 'center', minHeight: mobileTheme.sizes.minimumTouchTarget, paddingHorizontal: mobileTheme.spacing.lg, paddingVertical: mobileTheme.spacing.sm },
  fullWidth: { alignSelf: 'stretch' },
  primary: { backgroundColor: mobileTheme.colors.accent },
  secondary: { backgroundColor: mobileTheme.colors.surfaceRaised },
  quiet: { backgroundColor: 'transparent' },
  danger: { backgroundColor: mobileTheme.colors.errorSurface },
  label: { ...mobileTheme.typography.button, color: mobileTheme.colors.accent, flexShrink: 1, textAlign: 'center' },
  primaryLabel: { color: mobileTheme.colors.accentText },
  dangerLabel: { color: mobileTheme.colors.error },
  dimmed: { opacity: 0.5 },
});
