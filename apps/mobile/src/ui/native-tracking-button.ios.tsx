import { Button, Host, Image as SwiftUIImage, VStack } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  buttonStyle,
  clipShape,
  controlSize,
  disabled as nativeDisabled,
  frame,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Animated, Easing, StyleSheet, useWindowDimensions, View } from 'react-native';
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
  const rotation = useRef(new Animated.Value(0)).current;
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  const { fontScale } = useWindowDimensions();
  const buttonSize = Math.max(48, 40 + 8 * fontScale);
  const frameSize = buttonSize + 4;

  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => subscription.remove();
  }, []);

  useEffect(() => {
    if (!running || reduceMotion !== false) {
      rotation.stopAnimation();
      rotation.setValue(0);
      return;
    }
    const animation = Animated.loop(Animated.timing(rotation, {
      duration: 1400,
      easing: Easing.linear,
      toValue: 1,
      useNativeDriver: true,
    }));
    animation.start();
    return () => animation.stop();
  }, [reduceMotion, rotation, running]);

  return <View style={[styles.frame, { height: frameSize, width: frameSize }]}>
    {running ? <Animated.View
      importantForAccessibility="no"
      style={[styles.ring, {
        borderRadius: frameSize / 2,
        height: frameSize,
        transform: [{ rotate: rotation.interpolate({ inputRange: [0, 1], outputRange: ['0deg', '360deg'] }) }],
        width: frameSize,
      }]}
    /> : null}
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
  ring: {
    borderColor: mobileTheme.colors.error,
    borderTopColor: 'transparent',
    borderWidth: 3,
    position: 'absolute',
  },
});
