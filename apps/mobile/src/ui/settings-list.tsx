import { type ReactNode } from 'react';
import { Pressable, ScrollView, StyleSheet, Switch, Text, View, useWindowDimensions } from 'react-native';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { SettingsIcon } from './settings-icon';
import { mobileTheme } from './tokens';

export function SettingsShell({ children }: { children: ReactNode }) {
  return <ScrollView
    automaticallyAdjustContentInsets
    contentContainerStyle={styles.content}
    contentInsetAdjustmentBehavior="automatic"
    style={styles.shell}
  >
    {children}
  </ScrollView>;
}

export function SettingsSection({
  children,
  footer,
  title,
}: {
  children: ReactNode;
  footer?: string;
  title?: string;
}) {
  return <View style={styles.section}>
    {title ? <Text accessibilityRole="header" style={styles.sectionTitle}>{title}</Text> : null}
    <View style={styles.group}>{children}</View>
    {footer ? <Text style={styles.footer}>{footer}</Text> : null}
  </View>;
}

export function SettingsSeparator() {
  return <View style={styles.separator} />;
}

export function SettingsNavigationRow({
  accessibilityLabel,
  context,
  icon,
  label,
  onPress,
  value,
}: {
  accessibilityLabel: string;
  context?: string;
  icon?: ReactNode;
  label: string;
  onPress: () => void;
  value?: string;
}) {
  const { fontScale, width } = useWindowDimensions();
  const stacked = needsCompactVerticalLayout(width, fontScale);
  return <Pressable
    accessibilityLabel={accessibilityLabel}
    accessibilityRole="button"
    accessibilityValue={value ? { text: value } : undefined}
    onPress={onPress}
    style={({ pressed }) => [styles.row, pressed ? styles.rowPressed : null]}
  >
    <View style={styles.navigationRowContent}>
      {icon ? <View style={styles.rowIconFrame}>{icon}</View> : null}
      <View style={[styles.navigationRowMain, stacked ? styles.navigationRowMainStacked : null]}>
        <View style={styles.rowText}>
          <Text style={styles.rowLabel}>{label}</Text>
          {context ? <Text style={styles.rowContext}>{context}</Text> : null}
        </View>
        <View style={[styles.trailing, stacked ? styles.trailingStacked : null]}>
          {value ? <Text style={[styles.rowValue, stacked ? styles.valueStacked : null]}>{value}</Text> : null}
          <SettingsIcon systemName="chevron.right" variant="disclosure" />
        </View>
      </View>
    </View>
  </Pressable>;
}

export function SettingsValueRow({ label, value }: { label: string; value: string }) {
  const { fontScale, width } = useWindowDimensions();
  const stacked = needsCompactVerticalLayout(width, fontScale);
  return <View style={[styles.row, styles.valueRow]}>
    <View style={[styles.rowContent, stacked ? styles.rowContentStacked : null]}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Text selectable style={[styles.rowValue, stacked ? styles.valueStacked : null]}>{value}</Text>
    </View>
  </View>;
}

export function SettingsSwitchRow({
  accessibilityLabel,
  accessibilityLiveRegion,
  disabled = false,
  label,
  onValueChange,
  value,
  valueLabel,
}: {
  accessibilityLabel: string;
  accessibilityLiveRegion?: 'none' | 'polite' | 'assertive';
  disabled?: boolean;
  label: string;
  onValueChange: (value: boolean) => void;
  value: boolean;
  valueLabel: string;
}) {
  const { fontScale, width } = useWindowDimensions();
  const stacked = needsCompactVerticalLayout(width, fontScale);
  return <View accessibilityLiveRegion={accessibilityLiveRegion} style={[styles.row, styles.valueRow, disabled ? styles.disabled : null]}>
    <View style={[styles.rowContent, stacked ? styles.switchRowStacked : null]}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Switch
        accessibilityLabel={accessibilityLabel}
        accessibilityRole="switch"
        accessibilityState={{ checked: value, disabled }}
        accessibilityValue={{ text: valueLabel }}
        disabled={disabled}
        ios_backgroundColor={mobileTheme.colors.border}
        onValueChange={onValueChange}
        style={stacked ? styles.switchStacked : null}
        trackColor={{ false: mobileTheme.colors.border, true: mobileTheme.colors.accent }}
        value={value}
      />
    </View>
  </View>;
}

export function SettingsActionRow({
  accessibilityLabel,
  disabled = false,
  label,
  onPress,
  tone = 'destructive',
}: {
  accessibilityLabel: string;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  tone?: 'default' | 'destructive';
}) {
  return <Pressable
    accessibilityLabel={accessibilityLabel}
    accessibilityRole="button"
    accessibilityState={{ disabled }}
    disabled={disabled}
    onPress={onPress}
    style={({ pressed }) => [styles.row, pressed ? styles.rowPressed : null, disabled ? styles.disabled : null]}
  >
    <Text
      style={tone === 'destructive' ? styles.destructiveLabel : styles.actionLabel}
    >
      {label}
    </Text>
  </Pressable>;
}

const styles = StyleSheet.create({
  actionLabel: {
    color: mobileTheme.colors.accent,
    fontSize: 17,
    fontWeight: '500',
    lineHeight: 22,
  },
  chevron: {
    color: mobileTheme.colors.textMuted,
    fontSize: 28,
    fontWeight: '300',
    lineHeight: 28,
  },
  content: {
    gap: mobileTheme.spacing.lg,
    padding: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  destructiveLabel: {
    color: mobileTheme.colors.error,
    fontSize: 17,
    fontWeight: '500',
    lineHeight: 22,
  },
  disabled: {
    opacity: 0.48,
  },
  footer: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingTop: mobileTheme.spacing.xs,
  },
  group: {
    backgroundColor: mobileTheme.colors.surface,
    borderRadius: mobileTheme.radii.md,
    overflow: 'hidden',
  },
  navigationRowContent: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
  },
  navigationRowMain: {
    alignItems: 'center',
    flex: 1,
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'space-between',
    minWidth: 0,
  },
  navigationRowMainStacked: {
    alignItems: 'stretch',
    flexDirection: 'column',
    gap: mobileTheme.spacing.xs,
  },
  row: {
    justifyContent: 'center',
    minHeight: 52,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  rowContent: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'space-between',
  },
  rowContentStacked: {
    alignItems: 'stretch',
    flexDirection: 'column',
    gap: mobileTheme.spacing.xs,
  },
  rowLabel: {
    color: mobileTheme.colors.text,
    flexShrink: 1,
    fontSize: 17,
    fontWeight: '500',
    lineHeight: 22,
  },
  rowContext: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
  },
  rowIconFrame: {
    alignItems: 'center',
    flexShrink: 0,
    justifyContent: 'center',
  },
  rowPressed: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  rowText: {
    flex: 1,
    minWidth: 0,
  },
  rowValue: {
    color: mobileTheme.colors.textMuted,
    flexShrink: 1,
    fontSize: 16,
    lineHeight: 21,
    textAlign: 'right',
  },
  section: {
    gap: mobileTheme.spacing.xxs,
  },
  sectionTitle: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    fontWeight: '600',
    letterSpacing: 0.4,
    lineHeight: 18,
    paddingHorizontal: mobileTheme.spacing.md,
    textTransform: 'uppercase',
  },
  separator: {
    backgroundColor: mobileTheme.colors.separator,
    height: StyleSheet.hairlineWidth,
    marginLeft: mobileTheme.spacing.md,
  },
  shell: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
  },
  switchRowStacked: {
    alignItems: 'stretch',
    flexDirection: 'column',
  },
  switchStacked: {
    alignSelf: 'flex-start',
  },
  trailing: {
    alignItems: 'center',
    flexDirection: 'row',
    flexShrink: 1,
    gap: mobileTheme.spacing.xs,
  },
  trailingStacked: {
    alignItems: 'center',
    justifyContent: 'space-between',
    width: '100%',
  },
  valueRow: {
    backgroundColor: mobileTheme.colors.surface,
  },
  valueStacked: {
    textAlign: 'left',
  },
});
