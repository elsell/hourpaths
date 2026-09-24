import SegmentedControl from '@expo/ui/community/segmented-control';
import { StyleSheet } from 'react-native';
import { mobileTheme } from './tokens';

export type NativeSegment<Value extends string> = {
  label: string;
  value: Value;
};

export function NativeSegmentedControl<Value extends string>({
  disabled = false,
  onChange,
  segments,
  value,
}: {
  disabled?: boolean;
  onChange: (value: Value) => void;
  segments: readonly NativeSegment<Value>[];
  value: Value;
}) {
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
    tintColor={mobileTheme.colors.accent}
    values={segments.map((segment) => segment.label)}
  />;
}

const styles = StyleSheet.create({
  control: {
    minHeight: 44,
  },
});
