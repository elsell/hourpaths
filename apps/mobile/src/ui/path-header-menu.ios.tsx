import { Stack } from 'expo-router';
import type { NativeRouteAction } from './native-route-presentation';

/** Expo Router converts this composition directly into a native UIBarButtonItem and UIMenu. */
export function PathHeaderMenu({
  accessibilityLabel,
  actions,
}: {
  accessibilityLabel: string;
  actions: readonly NativeRouteAction[];
}) {
  return <Stack.Toolbar placement="right">
    <Stack.Toolbar.Menu accessibilityLabel={accessibilityLabel} icon="ellipsis">
      {actions.map((action) => <Stack.Toolbar.MenuAction
        disabled={action.disabled}
        icon={action.systemImage}
        key={action.label}
        onPress={action.onPress}
        destructive={action.destructive}
      >
        {action.label}
      </Stack.Toolbar.MenuAction>)}
    </Stack.Toolbar.Menu>
  </Stack.Toolbar>;
}
