import SegmentedControl from '@expo/ui/community/segmented-control';
import { StyleSheet, useWindowDimensions } from 'react-native';
import { NativeChoicePicker } from './native-choice-picker';

export type NativeSegment<Value extends string> = {
  label: string;
  value: Value;
};

export function NativeSegmentedControl<Value extends string>({
  label,
  disabled = false,
  onChange,
  segments,
  value,
}: {
  label: string;
  disabled?: boolean;
  onChange: (value: Value) => void;
  segments: readonly NativeSegment<Value>[];
  value: Value;
}) {
  const { fontScale } = useWindowDimensions();
  if (fontScale > 1.2) return <NativeChoicePicker accessibilityLabel={label} label={label} choices={segments} disabled={disabled} value={value} onChange={onChange} />;
  const selectedIndex = Math.max(0, segments.findIndex((segment) => segment.value === value));
  return <SegmentedControl
    appearance="dark"
    enabled={!disabled}
    onValueChange={(label) => {
      const segment = segments.find((candidate) => candidate.label === label);
      if (segment) onChange(segment.value);
    }}
    selectedIndex={selectedIndex}
    style={styles.control}
    values={segments.map((segment) => segment.label)}
  />;
}

const styles = StyleSheet.create({
  control: {
    minHeight: 44,
  },
});
