import { NativeHost as Host } from './native-host';
import { Button, } from '@expo/ui/swift-ui';
import { accessibilityLabel, buttonStyle, disabled as nativeDisabled, tint } from '@expo/ui/swift-ui/modifiers';
import { StyleSheet } from 'react-native';
import { mobileTheme } from './tokens';

export function NativeCommentSendButton({ busy, disabled, label, onPress }: { busy: boolean; disabled: boolean; label: string; onPress: () => void }) {
  return <Host style={styles.host}><Button
    label={label}
    modifiers={[accessibilityLabel(label), buttonStyle('borderedProminent'), nativeDisabled(busy || disabled), tint(mobileTheme.colors.accent)]}
    onPress={onPress}
    systemImage="arrow.up"
  /></Host>;
}
const styles = StyleSheet.create({ host: { height: 44, minWidth: 48 } });
