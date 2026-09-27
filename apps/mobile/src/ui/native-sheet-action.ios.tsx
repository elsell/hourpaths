import { NativeHost as Host } from './native-host';
import { Button, } from '@expo/ui/swift-ui';
import { accessibilityLabel, buttonStyle, controlSize, frame, disabled as nativeDisabled, tint } from '@expo/ui/swift-ui/modifiers';
import { NativeHeaderButton } from './native-header-button';
import { mobileTheme } from './tokens';

export function NativeSheetAction({ disabled = false, label, onPress, systemImage }: { disabled?: boolean; label: string; onPress: () => void; systemImage?: string }) {
  if (systemImage) return <NativeHeaderButton
    accessibilityLabel={label}
    disabled={disabled}
    label={label}
    onPress={onPress}
    systemImage={systemImage}
  />;
  return <Host colorScheme="dark" matchContents>
    <Button label={label} modifiers={[accessibilityLabel(label), buttonStyle('plain'), controlSize('large'), frame({ minHeight: 44, minWidth: 44 }), nativeDisabled(disabled), tint(mobileTheme.colors.accent)]} onPress={() => { if (!disabled) onPress(); }} />
  </Host>;
}
