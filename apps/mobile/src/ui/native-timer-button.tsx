import { Button, StyleSheet, View } from 'react-native';
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
    <Button
      accessibilityLabel={label}
      accessibilityState={{ busy, disabled: busy }}
      color={running ? mobileTheme.colors.error : mobileTheme.colors.accent}
      disabled={busy}
      onPress={onPress}
      title={label}
    />
  </View>;
}

const styles = StyleSheet.create({
  container: {
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    width: '100%',
  },
});
