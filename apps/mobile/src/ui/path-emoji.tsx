import { StyleSheet, Text, View } from 'react-native';

/** Decorative artwork; the adjacent Path name provides its accessible identity. */
export function PathEmoji({ emoji, size = 40 }: { emoji: string; size?: number }) {
  return <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={[styles.circle, { width: size, height: size, borderRadius: size / 2 }]}>
    <Text allowFontScaling={false} style={{ fontSize: size * 0.6, lineHeight: size * 0.8, color: '#FFFFFF' }}>{emoji}</Text>
  </View>;
}
const styles = StyleSheet.create({ circle: { backgroundColor: '#292734', alignItems: 'center', justifyContent: 'center', flexShrink: 0 } });
