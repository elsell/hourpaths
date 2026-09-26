import { Pressable, StyleSheet, Text, useWindowDimensions, View } from 'react-native';
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
  stop: {
    backgroundColor: mobileTheme.colors.error,
  },
  stopIcon: {
    color: mobileTheme.colors.background,
  },
});
