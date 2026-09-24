import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Animated, Easing, Pressable, StyleSheet, Text, useWindowDimensions, View } from 'react-native';
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
    <Pressable
      accessibilityLabel={label}
      accessibilityRole="button"
      accessibilityState={{ busy, disabled: busy }}
      disabled={busy}
      onPress={onPress}
      style={({ pressed }) => [
        styles.button,
        { borderRadius: buttonSize / 2, height: buttonSize, width: buttonSize },
        running ? styles.stop : styles.start,
        pressed ? styles.pressed : null,
      ]}
    >
      <Text allowFontScaling={false} style={[styles.icon, running ? styles.stopIcon : null]}>
        {running ? '■' : '▶'}
      </Text>
    </Pressable>
  </View>;
}

const styles = StyleSheet.create({
  button: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  frame: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  icon: {
    color: mobileTheme.colors.accentText,
    fontSize: 18,
  },
  pressed: {
    opacity: 0.72,
  },
  start: {
    backgroundColor: mobileTheme.colors.accent,
  },
  ring: {
    borderColor: mobileTheme.colors.error,
    borderTopColor: 'transparent',
    borderWidth: 3,
    position: 'absolute',
  },
  stop: {
    backgroundColor: mobileTheme.colors.error,
  },
  stopIcon: {
    color: mobileTheme.colors.background,
  },
});
