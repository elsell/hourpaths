import { Button, StyleSheet, View } from 'react-native';

export function NativePrimaryButton({
  disabled = false,
  fullWidth: _fullWidth = false,
  label,
  onPress,
  systemImage: _systemImage,
  variant: _variant = 'prominent',
}: {
  disabled?: boolean;
  fullWidth?: boolean;
  label: string;
  onPress: () => void;
  systemImage?: string;
  variant?: 'plain' | 'prominent';
}) {
  return <View style={styles.container}>
    <Button
      accessibilityLabel={label}
      accessibilityState={{ disabled }}
      disabled={disabled}
      onPress={onPress}
      title={label}
    />
  </View>;
}

const styles = StyleSheet.create({
  container: {
    justifyContent: 'center',
    minHeight: 48,
  },
});
