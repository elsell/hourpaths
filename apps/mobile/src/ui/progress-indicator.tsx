import { StyleSheet, Text, View } from 'react-native';
import { boundedAccessibilityProgress } from './progress-indicator-values';
import { mobileTheme } from './tokens';

type ProgressIndicatorProps = {
  accessibilityLabel: string;
  compact?: boolean;
  targetValue: number;
  text: string;
  visualValue: number;
};

export function ProgressIndicator({
  accessibilityLabel,
  compact = false,
  targetValue,
  text,
  visualValue,
}: ProgressIndicatorProps) {
  const visualPercent = targetValue > 0 ? (visualValue / targetValue) * 100 : 0;
  const clampedVisualPercent = Math.min(100, Math.max(0, visualPercent));
  const accessibilityProgress = boundedAccessibilityProgress(targetValue, visualValue);

  return <View
    accessible
    accessibilityLabel={accessibilityLabel}
    accessibilityRole="progressbar"
    accessibilityValue={{
      min: 0,
      ...accessibilityProgress,
      text,
    }}
    style={[styles.container, compact ? styles.compactContainer : null]}
  >
    <Text style={[styles.label, compact ? styles.compactLabel : null]}>{text}</Text>
    <View
      importantForAccessibility="no-hide-descendants"
      style={[styles.track, compact ? styles.compactTrack : null]}
    >
      <View style={[styles.fill, compact ? styles.compactFill : null, { width: `${clampedVisualPercent}%` as `${number}%` }]} />
    </View>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderRadius: mobileTheme.radii.md,
    gap: mobileTheme.spacing.xs,
    padding: mobileTheme.spacing.sm,
  },
  compactContainer: {
    backgroundColor: 'transparent',
    flex: 1,
    gap: mobileTheme.spacing.xxs,
    minWidth: 0,
    padding: 0,
  },
  compactFill: {
    height: 4,
  },
  compactLabel: {
    ...mobileTheme.typography.caption,
  },
  compactTrack: {
    height: 4,
  },
  fill: {
    backgroundColor: mobileTheme.colors.accent,
    borderRadius: mobileTheme.radii.pill,
    height: mobileTheme.sizes.progressTrack,
  },
  label: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
  },
  track: {
    backgroundColor: mobileTheme.colors.progressTrack,
    borderRadius: mobileTheme.radii.pill,
    height: mobileTheme.sizes.progressTrack,
    overflow: 'hidden',
  },
});
