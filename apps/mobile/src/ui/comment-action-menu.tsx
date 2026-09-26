import type { CommentAction } from './comment-action';
import { NativeActionMenu } from './native-action-menu';
export function CommentActionMenu({ accessibilityLabel, actions }: { accessibilityLabel: string; actions: readonly CommentAction[] }) {
  return <NativeActionMenu accessibilityLabel={accessibilityLabel} actions={actions} />;
}
