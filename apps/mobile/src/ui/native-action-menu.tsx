import { Alert, Button } from 'react-native';
import type { NativeMenuAction } from './native-menu-action';

export function NativeActionMenu({
  accessibilityLabel,
  actions,
  color,
}: {
  accessibilityLabel: string;
  color?: string;
  actions: readonly NativeMenuAction[];
}) {
  return <Button
    color={color}
    accessibilityLabel={accessibilityLabel}
    onPress={() => Alert.alert(
      accessibilityLabel,
      undefined,
      actions.filter((action) => !action.disabled).map((action) => ({
        onPress: action.onPress,
        style: action.destructive ? 'destructive' as const : 'default' as const,
        text: action.label,
      })),
    )}
    title="•••"
  />;
}
