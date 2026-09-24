import { StyleSheet, Text, View } from 'react-native';
import { mobileTheme } from './tokens';

export function NativeSystemImage({ systemName: _systemName }: { systemName: string }) {
  return <View
    accessibilityElementsHidden
    importantForAccessibility="no-hide-descendants"
    style={styles.container}
  >
    <Text allowFontScaling={false} style={styles.glyph}>!</Text>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    borderColor: mobileTheme.colors.textMuted,
    borderRadius: 18,
    borderWidth: 2,
    height: 36,
    justifyContent: 'center',
    width: 36,
  },
  glyph: {
    color: mobileTheme.colors.textMuted,
    fontSize: 23,
    fontWeight: '600',
    lineHeight: 25,
  },
});
