import { useEffect, useRef } from 'react';
import { TextField } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  disabled,
  lineLimit,
  textFieldStyle,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet } from 'react-native';
import type { NativeCommentInputProps } from './native-comment-input-types';
import { NativeHost as Host } from './native-host';

type NativeTextFieldHandle = {
  blur: () => Promise<void>;
  focus: () => Promise<void>;
  setSelection: (start: number, end: number) => Promise<void>;
  setText: (value: string) => Promise<void>;
};

export function NativeCommentInput({
  accessibilityLabel: label,
  autoFocus = false,
  editable = true,
  maximumLines = 5,
  minimumLines = 1,
  onChangeText,
  placeholder,
  value,
}: NativeCommentInputProps) {
  const input = useRef<NativeTextFieldHandle>(null);
  const nativeValue = useRef(value);

  useEffect(() => {
    if (nativeValue.current === value) return;
    nativeValue.current = value;
    void input.current?.setText(value);
  }, [value]);

  return <Host matchContents={{ vertical: true }} style={styles.host}>
    <TextField
      autoFocus={autoFocus}
      axis="vertical"
      defaultValue={value}
      modifiers={[
        accessibilityLabel(label),
        disabled(!editable),
        lineLimit({ min: minimumLines, max: maximumLines }),
        textFieldStyle('roundedBorder'),
      ]}
      onValueChange={(nextValue) => {
        nativeValue.current = nextValue;
        onChangeText(nextValue);
      }}
      placeholder={placeholder}
      ref={input}
    />
  </Host>;
}

const styles = StyleSheet.create({
  host: { minHeight: 44, width: '100%' },
});
