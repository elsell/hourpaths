import { Button } from 'react-native';
import { mobileTheme } from './tokens';
export function NativeCommentSendButton({ busy, disabled, label, onPress }: { busy: boolean; disabled: boolean; label: string; onPress: () => void }) {
  return <Button accessibilityLabel={label} accessibilityState={{ busy }} color={mobileTheme.colors.accent} disabled={busy || disabled} onPress={onPress} title={label} />;
}
