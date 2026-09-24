import { Button, Host } from '@expo/ui/swift-ui';
import {
  accessibilityLabel as nativeAccessibilityLabel,
  buttonStyle,
  controlSize,
  disabled as nativeDisabled,
  foregroundColor,
  frame,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

export function NativeTimerButton({
  busy,
  label,
  onPress,
  running,
}: {
  busy: boolean;
  label: string;
  onPress: () => void;
  running: boolean;
}) {
  return <View style={styles.container}>
    <Host style={styles.host}>
      <Button
        label={label}
        modifiers={[
          nativeAccessibilityLabel(label),
          buttonStyle('borderedProminent'),
          controlSize('large'),
          frame({ maxWidth: Number.POSITIVE_INFINITY, minHeight: mobileTheme.sizes.minimumTouchTarget }),
          tint(running ? mobileTheme.colors.errorSurface : mobileTheme.colors.accent),
          foregroundColor(running ? mobileTheme.colors.error : mobileTheme.colors.accentText),
          nativeDisabled(busy),
        ]}
        onPress={onPress}
      />
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    width: '100%',
  },
  host: {
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    width: '100%',
  },
});
