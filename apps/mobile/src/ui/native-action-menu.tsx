import { Alert, Button } from 'react-native';
import type { NativeMenuAction } from './native-menu-action';

export function NativeActionMenu({
  accessibilityLabel,
  actions,
}: {
  accessibilityLabel: string;
  actions: readonly NativeMenuAction[];
}) {
  return <Button
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
