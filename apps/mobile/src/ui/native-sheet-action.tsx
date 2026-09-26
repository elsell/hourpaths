import { NativeButton } from './native-button';
export function NativeSheetAction({ disabled = false, label, onPress }: { disabled?: boolean; label: string; onPress: () => void }) {
  return <NativeButton disabled={disabled} label={label} onPress={onPress} variant="quiet" />;
}
