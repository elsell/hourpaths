import { useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { Button, HStack, Image, Spacer, Text } from '@expo/ui/swift-ui';
import { buttonStyle, contentShape, disabled as nativeDisabled, fixedSize, foregroundColor, frame, padding, shapes } from '@expo/ui/swift-ui/modifiers';
import { NativeHost } from './native-host';
import type { SettingsChoiceRowProps } from './settings-choice-row';
import { mobileTheme } from './tokens';

export function SettingsChoiceRow({ label, selected, disabled = false, onPress }: SettingsChoiceRowProps) {
  const [width, setWidth] = useState(0);
  const activate = () => { if (!disabled) onPress(); };
  return <View accessible accessibilityLabel={label} accessibilityRole="radio" accessibilityState={{ disabled, checked: selected }}
    onAccessibilityTap={activate} onLayout={event => setWidth(Math.round(event.nativeEvent.layout.width))} style={styles.row}>
    <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      <NativeHost matchContents={{ vertical: true }} style={styles.host}>
        <Button modifiers={[buttonStyle('plain'), nativeDisabled(disabled)]} onPress={activate}>
          <HStack modifiers={[
            padding({ horizontal: mobileTheme.spacing.md, vertical: mobileTheme.spacing.sm }),
            frame({ width: width || undefined, minHeight: 52, alignment: 'leading' }),
            contentShape(shapes.rectangle()),
          ]}>
            <Text modifiers={[foregroundColor(mobileTheme.colors.text), fixedSize({ horizontal: false, vertical: true })]}>{label}</Text>
            <Spacer />
            <Image systemName={selected ? 'checkmark' : 'circle'} size={17} color={selected ? mobileTheme.colors.accent : mobileTheme.colors.textMuted} />
          </HStack>
        </Button>
      </NativeHost>
    </View>
  </View>;
}
const styles = StyleSheet.create({ row: { width: '100%', minHeight: 52 }, host: { width: '100%' } });
