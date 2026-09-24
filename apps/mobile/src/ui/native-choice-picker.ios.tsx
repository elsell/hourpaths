import { Host, Picker, Text } from '@expo/ui/swift-ui';
import {
  accessibilityLabel as nativeAccessibilityLabel,
  disabled as nativeDisabled,
  environment,
  pickerStyle,
  tag,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
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
    <Host style={styles.host}>
      <Picker
        label={label}
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
        {choices.map((choice) => <Text key={choice.value} modifiers={[tag(choice.value)]}>
          {choice.label}
        </Text>)}
      </Picker>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    alignSelf: 'stretch',
    justifyContent: 'center',
    minHeight: 48,
    width: '100%',
  },
  host: {
    minHeight: 48,
    width: '100%',
  },
});
