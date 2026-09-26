import { NativeHost as Host } from './native-host';
import { type ComponentProps } from 'react';
import { Button, } from '@expo/ui/swift-ui';
import {
  accessibilityLabel as nativeAccessibilityLabel,
  buttonStyle,
  disabled as nativeDisabled,
  labelStyle,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';

export function NativeHeaderButton({
  accessibilityLabel,
  disabled = false,
  onPress,
  systemImage = 'person.crop.circle',
}: {
  accessibilityLabel: string;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  systemImage?: ComponentProps<typeof Button>['systemImage'];
}) {
  return <View style={styles.container}>
    <Host style={styles.host}>
      <Button
        label={accessibilityLabel}
        modifiers={[
          nativeAccessibilityLabel(accessibilityLabel),
          buttonStyle('plain'),
          nativeDisabled(disabled),
          labelStyle('iconOnly'),
        ]}
        onPress={onPress}
        systemImage={systemImage}
      />
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    height: 44,
    width: 44,
  },
  host: {
    height: 44,
    width: 44,
  },
});
