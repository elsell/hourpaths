import { NativeHost as Host } from './native-host';
import { Button, Image as SwiftUIImage, VStack } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  buttonStyle,
  clipShape,
  controlSize,
  disabled as nativeDisabled,
  frame,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, useWindowDimensions, View } from 'react-native';
import { mobileTheme } from './tokens';

export function NativeTrackingButton({
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
  const { fontScale } = useWindowDimensions();
  const buttonSize = Math.max(48, 40 + 8 * fontScale);
  const frameSize = buttonSize + 4;

  return <View style={[styles.frame, { height: frameSize, width: frameSize }]}>

    <Host style={[styles.host, { height: buttonSize, width: buttonSize }]}>
      <Button
        modifiers={[
          accessibilityLabel(label),
          buttonStyle('borderedProminent'),
          controlSize('large'),
          clipShape('circle'),
          frame({ height: buttonSize, width: buttonSize }),
          tint(running ? mobileTheme.colors.error : mobileTheme.colors.accent),
          nativeDisabled(busy),
        ]}
        onPress={onPress}
      >
        <VStack alignment="center" spacing={1}>
          <SwiftUIImage
            color={running ? mobileTheme.colors.background : mobileTheme.colors.accentText}
            size={running ? 14 : 20}
            systemName={running ? 'stop.fill' : 'play.fill'}
          />
        </VStack>
      </Button>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  frame: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  host: {
  },
});
