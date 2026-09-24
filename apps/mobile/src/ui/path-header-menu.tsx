import { Stack } from 'expo-router';
import { NativeActionMenu } from './native-action-menu';
import type { NativeRouteAction } from './native-route-presentation';

export function PathHeaderMenu({
  accessibilityLabel,
  actions,
}: {
  accessibilityLabel: string;
  actions: readonly NativeRouteAction[];
}) {
  return <Stack.Toolbar asChild placement="right">
    <NativeActionMenu accessibilityLabel={accessibilityLabel} actions={actions} />
  </Stack.Toolbar>;
}
