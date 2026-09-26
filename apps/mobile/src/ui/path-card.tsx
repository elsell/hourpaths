import type { ReactNode } from 'react';
import { Pressable, StyleSheet, Text, useWindowDimensions, View } from 'react-native';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { NativeActionMenu } from './native-action-menu';
import type { NativeRouteAction } from './native-route-presentation';
import { SettingsIcon } from './settings-icon';
import { mobileTheme } from './tokens';

type PathCardProps = {
  actions?: readonly NativeRouteAction[];
  actionsAccessibilityLabel?: string;
  accumulatedText?: string;
  intervalSummary?: string;
  name: string;
  onOpen: () => void;
  progress?: ReactNode;
  timer?: ReactNode;
};

export function PathCard({
  actions,
  actionsAccessibilityLabel,
  accumulatedText,
  intervalSummary,
  name,
  onOpen,
  progress,
  timer,
}: PathCardProps) {
  const { fontScale, width } = useWindowDimensions();
  const accessibilityLayout = needsCompactVerticalLayout(width, fontScale);
  const tracking = Boolean(timer);

  return <View style={[
    styles.card,
    tracking ? styles.trackingRow : null,
    tracking && accessibilityLayout ? styles.accessibilityRow : null,
  ]}>
    <Pressable accessible={false} onPress={onOpen}
      style={({ pressed }) => [styles.identity, tracking ? styles.trackingIdentity : null, pressed && styles.identityPressed]}>
      <View style={styles.identityCopy}>
          <Pressable accessibilityLabel={name} accessibilityRole="button" onPress={onOpen} style={styles.nameRow}>
          <Text style={styles.name}>{name}</Text>
          <SettingsIcon systemName="chevron.right" variant="disclosure" />
          </Pressable>
        {accumulatedText ? <Text style={styles.accumulated}>{accumulatedText}</Text> : null}
        {intervalSummary ? <Text style={styles.summary}>{intervalSummary}</Text> : null}
        {progress ? <View style={styles.progressStack}>{progress}</View> : null}
      </View>
    </Pressable>
    {timer ? <View style={styles.timer}>{timer}</View> : null}
    {actions?.length && actionsAccessibilityLabel ? <View style={styles.actions}>
      <NativeActionMenu accessibilityLabel={actionsAccessibilityLabel} actions={actions} />
    </View> : null}
  </View>;
}

const styles = StyleSheet.create({
  actions: {
    alignItems: 'center',
    justifyContent: 'center',
    paddingRight: mobileTheme.spacing.xxs,
  },
  accumulated: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  card: {
    alignItems: 'stretch',
    backgroundColor: 'transparent',
    borderBottomColor: mobileTheme.colors.separator,
    borderBottomWidth: StyleSheet.hairlineWidth,
    flexDirection: 'row',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    overflow: 'hidden',
  },
  accessibilityRow: {
    flexDirection: 'column',
    height: undefined,
    minHeight: 80,
  },
  identity: {
    alignItems: 'center',
    flex: 1,
    flexDirection: 'row',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    minWidth: 0,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.md,
  },
  identityCopy: {
    flex: 1,
    gap: mobileTheme.spacing.xxs,
    minWidth: 0,
  },
  nameRow: {
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xxs,
  },
  nameLink: {
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  identityPressed: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  name: {
    color: mobileTheme.colors.text,
    flexShrink: 1,
    ...mobileTheme.typography.subheading,
  },
  progressStack: {
    flexDirection: 'column',
    gap: mobileTheme.spacing.xs,
    minWidth: 0,
  },
  summary: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  timer: {
    alignItems: 'center',
    alignSelf: 'stretch',
    justifyContent: 'center',
    paddingRight: mobileTheme.spacing.xs,
  },
  trackingIdentity: {
    flex: 1,
    minWidth: 0,
  },
  trackingRow: {
    minHeight: 60,
  },
});
