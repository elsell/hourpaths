import { StyleSheet, Text, View } from 'react-native';
import { NativeTrackingButton } from './native-tracking-button';
import { mobileTheme } from './tokens';

type TimerControlProps = {
  actionLabel: string;
  busy: boolean;
  elapsedAccessibilityLabel?: string;
  elapsedText?: string;
  errorText?: string;
  onPress: () => void;
  running: boolean;
};

export function TimerControl({
  actionLabel,
  busy,
  elapsedAccessibilityLabel,
  elapsedText,
  errorText,
  onPress,
  running,
}: TimerControlProps) {
  return <View style={[styles.container, running ? styles.runningContainer : null]}>
    {running && elapsedText ? <Text
      accessibilityLabel={elapsedAccessibilityLabel}
      style={styles.elapsed}
    >{elapsedText}</Text> : null}
    <NativeTrackingButton
      busy={busy}
      label={actionLabel}
      onPress={onPress}
      running={running}
    />
    {errorText ? <Text accessibilityRole="alert" style={styles.error}>{errorText}</Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: 88,
  },
  elapsed: {
    color: mobileTheme.colors.text,
    fontVariant: ['tabular-nums'],
    paddingBottom: mobileTheme.spacing.xxs,
    textAlign: 'center',
    ...mobileTheme.typography.caption,
  },
  error: {
    color: mobileTheme.colors.error,
    maxWidth: 120,
    textAlign: 'center',
    ...mobileTheme.typography.caption,
  },
  runningContainer: {
    minWidth: 88,
  },
});
