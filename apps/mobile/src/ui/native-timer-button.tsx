import { NativeButton } from './native-button';
export function NativeTimerButton({ busy, label, onPress, running }: {
  busy: boolean;
  label: string;
  onPress: () => void;
  running: boolean;
}) {
  return <NativeButton accessibilityLabel={label} busy={busy} fullWidth label={label} onPress={onPress} variant={running ? 'danger' : 'primary'} />;
}
