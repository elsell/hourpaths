import { DelayedStatus } from './delayed-status';
import { PathEmoji } from './path-emoji';
import type { ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View, type GestureResponderEvent } from 'react-native';
import { NativeActionMenu } from './native-action-menu';
import type { NativeRouteAction } from './native-route-presentation';
import { defaultPathAppearance, pathPalette, type PathAppearance } from './path-appearance';
import { mobileTheme } from './tokens';

export type PathCardProps = {
  onLongPress?: (event: GestureResponderEvent) => void;
  reorderHint?: string;
  onFocusTarget?: (node: View | null) => void;
  actions?: readonly NativeRouteAction[];
  actionsAccessibilityLabel?: string;
  accumulatedText?: string;
  statusText?: string;
  appearance?: PathAppearance;
  headline?: string;
  intervalSummary?: string;
  name: string;
  onOpen?: () => void;
  progress?: ReactNode;
  timer?: ReactNode;
};

export function PathCard({ onLongPress, reorderHint, onFocusTarget, actions, actionsAccessibilityLabel, accumulatedText, statusText, appearance = defaultPathAppearance(''), headline, intervalSummary, name, onOpen, progress, timer }: PathCardProps) {
  const tone = pathPalette[appearance.color];
  return <View style={[styles.card, { backgroundColor: tone.background }]}>
    {onOpen ? <Pressable ref={onFocusTarget} accessibilityLabel={name} accessibilityRole="button" accessibilityHint={onLongPress ? reorderHint : undefined} onPress={onOpen} onLongPress={onLongPress} delayLongPress={350}
      style={({ pressed }) => [StyleSheet.absoluteFill, styles.hitSurface, pressed ? styles.pressed : null]} /> : null}
    <View pointerEvents="none" style={[styles.identity, actions?.length ? styles.withActions : null]}>
      <PathEmoji emoji={appearance.emoji} />
      <Text style={[styles.name, { color: tone.foreground }]}>{name}</Text>
    </View>
    {actions?.length && actionsAccessibilityLabel ? <View style={styles.actions}><NativeActionMenu color={tone.accent} accessibilityLabel={actionsAccessibilityLabel} actions={actions} /></View> : null}
    <View pointerEvents="none" style={styles.summaryStack}>
      {headline ? <Text adjustsFontSizeToFit minimumFontScale={0.7} numberOfLines={1} style={[styles.headline, { color: tone.foreground }]}>{headline}</Text> : null}
      {intervalSummary ? <Text style={[styles.summary, { color: tone.foreground }]}>{intervalSummary}</Text> : null}
      {progress ? <View style={styles.progressStack}>{progress}</View> : null}
      {statusText ? <DelayedStatus><Text style={[styles.summary, { color: tone.foreground }]}>{statusText}</Text></DelayedStatus> : null}
      {accumulatedText ? <Text style={[styles.summary, { color: tone.foreground }]}>{accumulatedText}</Text> : null}
    </View>
    {timer ? <View style={styles.timer}>{timer}</View> : null}
  </View>;
}

const styles = StyleSheet.create({
  card: { borderRadius: mobileTheme.radii.lg, borderCurve: 'continuous', padding: mobileTheme.spacing.sm, gap: mobileTheme.spacing.xs, flex: 1 },
  identity: { flexDirection: 'row', alignItems: 'flex-start', minWidth: 0, minHeight: mobileTheme.sizes.minimumTouchTarget, gap: mobileTheme.spacing.xs },
  withActions: { paddingRight: mobileTheme.sizes.minimumTouchTarget },
  actions: { position: 'absolute', top: mobileTheme.spacing.xxs, right: mobileTheme.spacing.xxs },
  hitSurface: { borderRadius: mobileTheme.radii.lg },
  summaryStack: { gap: mobileTheme.spacing.xs },
  nameLink: { flex: 1, flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: mobileTheme.spacing.xs, minHeight: mobileTheme.sizes.minimumTouchTarget },
  name: { ...mobileTheme.typography.body, fontWeight: '600', flexShrink: 1 },
  emoji: { fontSize: 26 },
  headline: { fontSize: 30, fontWeight: '700', fontVariant: ['tabular-nums'] },
  summary: { ...mobileTheme.typography.caption },
  progressStack: { gap: mobileTheme.spacing.xs },
  timer: { marginTop: 'auto', paddingTop: mobileTheme.spacing.xxs },
  pressed: { opacity: 0.65 },
});
