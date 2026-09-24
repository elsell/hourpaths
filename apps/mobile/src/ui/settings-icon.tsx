import { StyleSheet, Text, View } from 'react-native';
import { mobileTheme } from './tokens';

export function SettingsIcon({
  systemName,
  variant = 'settings',
}: {
  systemName: string;
  variant?: 'disclosure' | 'settings';
}) {
  return <View
    accessibilityElementsHidden
    importantForAccessibility="no-hide-descendants"
    style={[styles.frame, variant === 'disclosure' ? styles.disclosureFrame : null]}
  >
    {variant === 'disclosure'
      ? <Text allowFontScaling={false} style={styles.disclosureGlyph}>›</Text>
      : systemName === 'plus'
        ? <Text allowFontScaling={false} style={styles.plusGlyph}>+</Text>
        : systemName === 'bell'
          ? <View style={styles.bellFrame}>
            <View style={styles.bell} />
            <View style={styles.bellClapper} />
          </View>
          : <>
            <View style={styles.head} />
            <View style={styles.shoulders} />
          </>}
  </View>;
}

const styles = StyleSheet.create({
  frame: {
    alignItems: 'center',
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderRadius: mobileTheme.radii.sm,
    height: 34,
    justifyContent: 'center',
    width: 34,
  },
  disclosureFrame: {
    backgroundColor: 'transparent',
    borderRadius: 0,
    width: 20,
  },
  disclosureGlyph: {
    color: mobileTheme.colors.textMuted,
    fontSize: 24,
    lineHeight: 28,
  },
  plusGlyph: {
    color: mobileTheme.colors.accent,
    fontSize: 27,
    fontWeight: '500',
    lineHeight: 29,
  },
  bellFrame: {
    alignItems: 'center',
    height: 20,
    justifyContent: 'flex-end',
    width: 20,
  },
  bell: {
    borderColor: mobileTheme.colors.accent,
    borderTopLeftRadius: 8,
    borderTopRightRadius: 8,
    borderWidth: 2,
    height: 14,
    width: 16,
  },
  bellClapper: {
    backgroundColor: mobileTheme.colors.accent,
    borderBottomLeftRadius: 2,
    borderBottomRightRadius: 2,
    height: 3,
    width: 5,
  },
  head: {
    backgroundColor: mobileTheme.colors.accent,
    borderRadius: 5,
    height: 10,
    marginBottom: 2,
    width: 10,
  },
  shoulders: {
    backgroundColor: mobileTheme.colors.accent,
    borderTopLeftRadius: 8,
    borderTopRightRadius: 8,
    height: 8,
    width: 18,
  },
});
