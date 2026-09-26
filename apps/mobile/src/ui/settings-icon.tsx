import { StyleSheet, View } from 'react-native';
import { PlatformSymbol } from './platform-symbol';
import { mobileTheme } from './tokens';

export function SettingsIcon({ systemName, variant = 'settings' }: {
  systemName: string;
  variant?: 'disclosure' | 'settings' | 'inline';
}) {
  return <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants"
    style={[styles.frame, variant !== 'settings' && styles.plain]}>
    <PlatformSymbol systemName={systemName} size={variant === 'disclosure' ? 20 : 24}
      color={variant === 'disclosure' ? mobileTheme.colors.textMuted : mobileTheme.colors.accent} />
  </View>;
}
const styles = StyleSheet.create({
  frame: { alignItems: 'center', backgroundColor: mobileTheme.colors.surfaceRaised, borderRadius: mobileTheme.radii.sm, height: 34, justifyContent: 'center', width: 34 },
  plain: { backgroundColor: 'transparent', width: 24 },
});
