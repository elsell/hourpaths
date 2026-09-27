import { NativeButton } from './native-button';
import { NativeHeaderButton } from './native-header-button';
export function NativeSheetAction({ disabled = false, label, onPress, systemImage }: { disabled?: boolean; label: string; onPress: () => void; systemImage?: string }) {
  if (systemImage) return <NativeHeaderButton accessibilityLabel={label} disabled={disabled} label={label} onPress={onPress} systemImage={systemImage} />;
  return <NativeButton disabled={disabled} label={label} onPress={onPress} variant="quiet" />;
}
