import type { ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { NativeActionMenu } from './native-action-menu';
import type { NativeRouteAction } from './native-route-presentation';
import { defaultPathAppearance, pathPalette, type PathAppearance } from './path-appearance';
import { mobileTheme } from './tokens';

export type PathCardProps = {
  onFocusTarget?: (node: View | null) => void;
  actions?: readonly NativeRouteAction[];
  actionsAccessibilityLabel?: string;
  accumulatedText?: string;
  appearance?: PathAppearance;
  headline?: string;
  intervalSummary?: string;
  name: string;
  onOpen?: () => void;
  progress?: ReactNode;
  timer?: ReactNode;
};

export function PathCard({ onFocusTarget, actions, actionsAccessibilityLabel, accumulatedText, appearance = defaultPathAppearance(''), headline, intervalSummary, name, onOpen, progress, timer }: PathCardProps) {
  const tone = pathPalette[appearance.color];
  return <View style={[styles.card, { backgroundColor: tone.background }]}>
    <View style={styles.identity}>
      <Pressable ref={onFocusTarget} accessibilityLabel={name} accessibilityRole={onOpen ? "button" : undefined} disabled={!onOpen} onPress={onOpen}
        style={({ pressed }) => [styles.nameLink, pressed ? styles.pressed : null]}>
        <Text accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={styles.emoji}>{appearance.emoji}</Text>
        <Text style={[styles.name, { color: tone.foreground }]}>{name}</Text>
      </Pressable>
      {actions?.length && actionsAccessibilityLabel ? <NativeActionMenu color={tone.accent} accessibilityLabel={actionsAccessibilityLabel} actions={actions} /> : null}
    </View>
    {headline ? <Text style={[styles.headline, { color: tone.foreground }]}>{headline}</Text> : null}
    {intervalSummary ? <Text style={[styles.summary, { color: tone.foreground }]}>{intervalSummary}</Text> : null}
    {progress ? <View style={styles.progressStack}>{progress}</View> : null}
    {accumulatedText ? <Text style={[styles.summary, { color: tone.foreground }]}>{accumulatedText}</Text> : null}
    {timer ? <View style={styles.timer}>{timer}</View> : null}
  </View>;
}
const styles = StyleSheet.create({
  card: { borderRadius: mobileTheme.radii.lg, borderCurve: 'continuous', padding: mobileTheme.spacing.sm, gap: mobileTheme.spacing.xs, flex: 1 },
  identity: { flexDirection: 'row', alignItems: 'center', minWidth: 0 },
  nameLink: { flex: 1, flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: mobileTheme.spacing.xs, minHeight: mobileTheme.sizes.minimumTouchTarget },
  name: { ...mobileTheme.typography.body, fontWeight: '600', flexShrink: 1 },
  emoji: { fontSize: 26 },
  headline: { fontSize: 30, fontWeight: '700', fontVariant: ['tabular-nums'] },
  summary: { ...mobileTheme.typography.caption },
  progressStack: { gap: mobileTheme.spacing.xs },
  timer: { marginTop: 'auto', paddingTop: mobileTheme.spacing.xxs },
  pressed: { opacity: 0.65 },
});
