import { NativeHost as Host } from './native-host';
import { Button, } from '@expo/ui/swift-ui';
import { accessibilityLabel, buttonStyle, controlSize, frame, disabled as nativeDisabled, tint } from '@expo/ui/swift-ui/modifiers';
import { mobileTheme } from './tokens';

export function NativeSheetAction({ disabled = false, label, onPress }: { disabled?: boolean; label: string; onPress: () => void }) {
  return <Host colorScheme="dark" matchContents>
    <Button label={label} modifiers={[accessibilityLabel(label), buttonStyle('plain'), controlSize('large'), frame({ minHeight: 44, minWidth: 44 }), nativeDisabled(disabled), tint(mobileTheme.colors.accent)]} onPress={() => { if (!disabled) onPress(); }} />
  </Host>;
}
