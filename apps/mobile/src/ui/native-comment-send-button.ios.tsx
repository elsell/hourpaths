import { NativeHost as Host } from './native-host';
import { Button, } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  buttonStyle,
  clipShape,
  disabled as nativeDisabled,
  foregroundColor,
  frame,
  labelStyle,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet } from 'react-native';
import { mobileTheme } from './tokens';

export function NativeCommentSendButton({ busy, disabled, label, onPress }: { busy: boolean; disabled: boolean; label: string; onPress: () => void }) {
  return <Host style={styles.host}><Button
    label={label}
    modifiers={[
      accessibilityLabel(label),
      buttonStyle('borderedProminent'),
      clipShape('circle'),
      frame({ height: 44, width: 44 }),
      foregroundColor(mobileTheme.colors.accentText),
      labelStyle('iconOnly'),
      nativeDisabled(busy || disabled),
      tint(mobileTheme.colors.accent),
    ]}
    onPress={onPress}
    systemImage="arrow.up"
  /></Host>;
}
const styles = StyleSheet.create({ host: { height: 44, width: 44 } });
