import { StyleSheet, Text } from 'react-native';
import { mobileTheme } from './tokens';

export function CommentHeartIcon({ selected }: { selected: boolean }) {
  return <Text
    accessibilityElementsHidden
    allowFontScaling={false}
    importantForAccessibility="no-hide-descendants"
    style={[styles.icon, selected ? styles.selected : null]}
  >{selected ? '♥' : '♡'}</Text>;
}

const styles = StyleSheet.create({
  icon: { color: mobileTheme.colors.textMuted, fontSize: 20, lineHeight: 22 },
  selected: { color: mobileTheme.colors.accent },
});
