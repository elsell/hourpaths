import { Alert, Button } from 'react-native';
import type { CommentAction } from './comment-action';

export function CommentActionMenu({ accessibilityLabel, actions }: { accessibilityLabel: string; actions: readonly CommentAction[] }) {
  return <Button accessibilityLabel={accessibilityLabel} onPress={() => Alert.alert(
    accessibilityLabel,
    undefined,
    actions.filter(({ disabled }) => !disabled).map((action) => ({
      onPress: action.onPress,
      style: action.destructive ? 'destructive' : 'default',
      text: action.label,
    })),
  )} title="•••" />;
}
