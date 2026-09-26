import { NativeHost as Host } from './native-host';
import { Picker, Text as NativeText } from '@expo/ui/swift-ui';
import {
  accessibilityLabel as nativeAccessibilityLabel,
  disabled as nativeDisabled,
  environment,
  pickerStyle,
  tag,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, Text, View } from 'react-native';
import { mobileTheme } from './tokens';

export type NativeChoice<Value extends string> = {
  label: string;
  value: Value;
};

export function NativeChoicePicker<Value extends string>({
  accessibilityLabel,
  choices,
  disabled = false,
  label,
  onChange,
  value,
}: {
  accessibilityLabel: string;
  choices: readonly NativeChoice<Value>[];
  disabled?: boolean;
  label: string;
  onChange: (value: Value) => void;
  value: Value;
}) {
  return <View pointerEvents={disabled ? 'none' : 'auto'} style={styles.container}>
    <Text style={styles.label}>{label}</Text>
    <Host matchContents={{ vertical: true }} style={styles.host}>
      <Picker
        modifiers={[
          nativeAccessibilityLabel(accessibilityLabel),
          environment('colorScheme', 'dark'),
          pickerStyle('menu'),
          tint(mobileTheme.colors.accent),
          nativeDisabled(disabled),
        ]}
        onSelectionChange={onChange}
        selection={value}
      >
        {choices.map((choice) => <NativeText key={choice.value} modifiers={[tag(choice.value)]}>
          {choice.label}
        </NativeText>)}
      </Picker>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: mobileTheme.spacing.xs,
    justifyContent: 'center',
    minHeight: 48,
    width: '100%',
  },
  label: { color: mobileTheme.colors.text, ...mobileTheme.typography.body, flexGrow: 1, flexShrink: 1 },
  host: { minHeight: 48, width: '100%' },
});
