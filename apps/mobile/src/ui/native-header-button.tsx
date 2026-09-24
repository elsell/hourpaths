import { Button } from 'react-native';

export function NativeHeaderButton({
  accessibilityLabel,
  disabled = false,
  label,
  onPress,
  systemImage: _systemImage,
}: {
  accessibilityLabel: string;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  systemImage?: string;
}) {
  return <Button accessibilityLabel={accessibilityLabel} disabled={disabled} onPress={onPress} title={label} />;
}
