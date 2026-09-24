import { Stack } from 'expo-router';
import type { HomeOrder } from './home-organization';
import { homeFilters, type HomeFilter } from './home-organization';

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
  orderAccessibilityLabel,
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
  return <>
  <Stack.Toolbar placement="left">
    <Stack.Toolbar.Menu accessibilityLabel={pathViewAccessibilityLabel} icon="line.3.horizontal.decrease">
      <Stack.Toolbar.MenuAction
        icon="line.3.horizontal"
        isOn={mode === 'active'}
        onPress={() => onModeChange('active')}
      >
        {activeLabel}
      </Stack.Toolbar.MenuAction>
      <Stack.Toolbar.MenuAction
        icon="archivebox"
        isOn={mode === 'archived'}
        onPress={() => onModeChange('archived')}
      >
        {archivedLabel}
      </Stack.Toolbar.MenuAction>
      {mode === 'active' ? <>
        {filter && filterLabels && onFilterChange ? homeFilters.map((value) => <Stack.Toolbar.MenuAction
          isOn={filter === value} key={value} onPress={() => onFilterChange(value)}>
          {filterLabels[value]}
        </Stack.Toolbar.MenuAction>) : null}
        {(['recent', 'alphabetical', 'manual'] as const).map((value) => <Stack.Toolbar.MenuAction
          isOn={order === value}
          key={value}
          onPress={() => onOrderChange(value)}
        >
          {orderLabels[value]}
        </Stack.Toolbar.MenuAction>)}
        <Stack.Toolbar.MenuAction icon="line.3.horizontal" onPress={onArrange}>
          {orderLabels.arrange}
        </Stack.Toolbar.MenuAction>
      </> : null}
    </Stack.Toolbar.Menu>
  </Stack.Toolbar>
  <Stack.Toolbar placement="right">
    <Stack.Toolbar.Button accessibilityLabel={createAccessibilityLabel} icon="plus" onPress={onCreate} separateBackground />
    <Stack.Toolbar.Button
      accessibilityLabel={notificationsAccessibilityLabel}
      icon="bell"
      onPress={onOpenNotifications}
    />
    <Stack.Toolbar.Button accessibilityLabel={settingsAccessibilityLabel} icon="person.crop.circle" onPress={onOpenSettings} />
  </Stack.Toolbar>
  </>;
}
