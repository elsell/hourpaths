import { StyleSheet, Text, View } from 'react-native';
import { NativeButton } from './native-button';
import type { PathTone } from './path-appearance';
import { mobileTheme } from './tokens';

type TimerControlProps = {
  actionLabel: string;
  busy: boolean;
  elapsedAccessibilityLabel?: string;
  elapsedText?: string;
  errorText?: string;
  onPress: () => void;
  running: boolean;
  tone?: PathTone;
};

export function TimerControl({ actionLabel, busy, elapsedAccessibilityLabel, elapsedText, errorText, onPress, running, tone }: TimerControlProps) {
  return <View style={styles.container}>
    <NativeButton
      accessibilityLabel={elapsedAccessibilityLabel ?? actionLabel}
      busy={busy}
      fullWidth
      label={elapsedText ?? actionLabel}
      onPress={onPress}
      systemImage={running ? 'stop.fill' : 'play.fill'}
      tone={{ background: '#FFFFFFD9', foreground: running ? '#AD1830' : tone?.accent ?? '#235F9E' }}
      variant="secondary"
    />
    {errorText ? <Text accessibilityRole="alert" style={[styles.error, tone ? { color: tone.foreground } : null]}>{errorText}</Text> : null}
  </View>;
}
const styles = StyleSheet.create({
  container: { alignSelf: 'stretch', gap: mobileTheme.spacing.xxs },
  error: { color: mobileTheme.colors.error, ...mobileTheme.typography.caption },
});
