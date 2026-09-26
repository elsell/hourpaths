import { Pressable, StyleSheet } from 'react-native';
import { NativeButton } from './native-button';
import { PlatformSymbol } from './platform-symbol';
import { mobileTheme } from './tokens';
export function NativeHeaderButton({ accessibilityLabel, disabled = false, label, onPress, systemImage }: {
  accessibilityLabel: string;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  systemImage?: string;
}) {
  if (!systemImage) return <NativeButton accessibilityLabel={accessibilityLabel} disabled={disabled} label={label} onPress={onPress} variant="quiet" />;
  return <Pressable accessibilityLabel={accessibilityLabel} accessibilityRole="button" accessibilityState={{ disabled }}
    disabled={disabled} onPress={onPress} android_ripple={{ color: mobileTheme.colors.surfacePressed, borderless: true }}
    style={[styles.action, disabled && styles.disabled]}>
    <PlatformSymbol systemName={systemImage} color={mobileTheme.colors.accent} />
  </Pressable>;
}
const styles = StyleSheet.create({
  action: { alignItems: 'center', justifyContent: 'center', minHeight: 48, minWidth: 48 },
  disabled: { opacity: 0.4 },
});
