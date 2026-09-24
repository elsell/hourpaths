import { Pressable, StyleSheet, View } from 'react-native';
import { homeFilters, type HomeFilter, type HomeOrder } from './home-organization';
import { NativeActionMenu } from './native-action-menu';
import type { NativeRouteAction } from './native-route-presentation';
import { SettingsIcon } from './settings-icon';
import { mobileTheme } from './tokens';

export type HomePathMode = 'active' | 'archived';

export function HomeHeaderActions({
  activeLabel,
  archivedLabel,
  createAccessibilityLabel,
  filter,
  filterLabels,
  mode,
  onArrange,
  onCreate,
  onFilterChange,
  onModeChange,
  onOrderChange,
  onOpenNotifications,
  onOpenSettings,
  notificationsAccessibilityLabel,
  order,
  orderLabels,
  pathViewAccessibilityLabel,
  settingsAccessibilityLabel,
}: {
  activeLabel: string;
  archivedLabel: string;
  createAccessibilityLabel: string;
  filter?: HomeFilter;
  filterLabels?: Record<HomeFilter, string>;
  mode: HomePathMode;
  onArrange: () => void;
  onCreate: () => void;
  onFilterChange?: (filter: HomeFilter) => void;
  onModeChange: (mode: HomePathMode) => void;
  onOrderChange: (order: HomeOrder) => void;
  onOpenNotifications: () => void;
  onOpenSettings: () => void;
  notificationsAccessibilityLabel: string;
  order: HomeOrder;
  orderAccessibilityLabel: string;
  orderLabels: Record<HomeOrder, string> & { arrange: string };
  pathViewAccessibilityLabel: string;
  settingsAccessibilityLabel: string;
}) {
  const viewActions: NativeRouteAction[] = [
    { label: `${mode === 'active' ? '✓ ' : ''}${activeLabel}`, onPress: () => onModeChange('active'), systemImage: 'slider.horizontal.3' },
    { label: `${mode === 'archived' ? '✓ ' : ''}${archivedLabel}`, onPress: () => onModeChange('archived'), systemImage: 'archivebox' },
    ...(mode === 'active' ? ([
      ...(filter && filterLabels && onFilterChange ? homeFilters.map((value) => ({
        label: `${filter === value ? '✓ ' : ''}${filterLabels[value]}`,
        onPress: () => onFilterChange(value), systemImage: 'slider.horizontal.3' as const,
      })) : []),
      ...(['recent', 'alphabetical', 'manual'] as const).map((value) => ({
        label: `${order === value ? '✓ ' : ''}${orderLabels[value]}`,
        onPress: () => onOrderChange(value),
        systemImage: value === 'recent' ? 'clock.arrow.circlepath' as const : 'slider.horizontal.3' as const,
      })),
      { label: orderLabels.arrange, onPress: onArrange, systemImage: 'slider.horizontal.3' as const },
    ]) : []),
  ];
  const currentView = mode === 'active' ? activeLabel : archivedLabel;
  const menuAccessibilityLabel = mode === 'active'
    ? `${pathViewAccessibilityLabel}: ${currentView}, ${orderLabels[order]}`
    : `${pathViewAccessibilityLabel}: ${currentView}`;
  return <View style={styles.actions}>
    <Pressable accessibilityLabel={createAccessibilityLabel} accessibilityRole="button" onPress={onCreate} style={styles.action}>
      <SettingsIcon systemName="plus" />
    </Pressable>
    <NativeActionMenu
      accessibilityLabel={menuAccessibilityLabel}
      actions={viewActions}
    />
    <Pressable
      accessibilityLabel={notificationsAccessibilityLabel}
      accessibilityRole="button"
      onPress={onOpenNotifications}
      style={styles.action}
    ><SettingsIcon systemName="bell" /></Pressable>
    <Pressable accessibilityLabel={settingsAccessibilityLabel} accessibilityRole="button" onPress={onOpenSettings} style={styles.action}>
      <SettingsIcon systemName="person.crop.circle" />
    </Pressable>
  </View>;
}

const styles = StyleSheet.create({
  actions: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xxs,
    justifyContent: 'flex-end',
  },
  action: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    minWidth: mobileTheme.sizes.minimumTouchTarget,
  },
});
