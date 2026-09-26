import { NativeButton } from './native-button';
export function NativeCommentSendButton({ busy, disabled, label, onPress }: { busy: boolean; disabled: boolean; label: string; onPress: () => void }) {
  return <NativeButton accessibilityLabel={label} busy={busy} disabled={disabled} label={label} onPress={onPress} />;
}
