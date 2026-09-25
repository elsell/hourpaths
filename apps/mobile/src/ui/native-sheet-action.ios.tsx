import { Button, Host, HStack, Spacer, Text } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  buttonStyle,
  controlSize,
  disabled as nativeDisabled,
  fixedSize,
  foregroundColor,
  frame,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

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
    <Host matchContents={{ vertical: true }} style={styles.host}>
      <Button
        modifiers={[
          accessibilityLabel(label),
          buttonStyle('plain'),
          controlSize('large'),
          nativeDisabled(disabled),
          foregroundColor(mobileTheme.colors.accent),
          tint(mobileTheme.colors.accent),
        ]}
        onPress={() => {
          if (!disabled) onPress();
        }}
      >
        <HStack>
          <Spacer />
          <Text modifiers={[
            fixedSize({ horizontal: false, vertical: true }),
            frame({ minHeight: 24 }),
            foregroundColor(mobileTheme.colors.accent),
          ]}>{label}</Text>
          <Spacer />
        </HStack>
      </Button>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 64,
  },
  host: {
    minHeight: 44,
    width: '100%',
  },
});
