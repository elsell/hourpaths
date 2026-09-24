import { StyleSheet, View } from 'react-native';
import { NativePrimaryButton } from './native-primary-button';

export function NativeSheetAction({
  disabled = false,
  label,
  onPress,
}: {
  disabled?: boolean;
  label: string;
  onPress: () => void;
}) {
  return <View pointerEvents={disabled ? 'none' : 'auto'} style={styles.container}>
    <NativePrimaryButton
      disabled={disabled}
      label={label}
      onPress={onPress}
      variant="plain"
    />
  </View>;
}

const styles = StyleSheet.create({
  container: {
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 64,
  },
});
