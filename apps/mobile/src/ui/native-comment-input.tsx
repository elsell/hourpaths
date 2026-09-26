import { StyleSheet } from 'react-native';
import type { NativeCommentInputProps } from './native-comment-input-types';
import { ThemedTextInput } from './primitives';

export function NativeCommentInput({
  accessibilityLabel,
  autoFocus = false,
  editable = true,
  maximumLines = 5,
  minimumLines = 1,
  onChangeText,
  placeholder,
  value,
}: NativeCommentInputProps) {
  return <ThemedTextInput
    accessibilityLabel={accessibilityLabel}
    allowFontScaling
    autoFocus={autoFocus}
    editable={editable}
    multiline
    numberOfLines={minimumLines}
    onChangeText={onChangeText}
    placeholder={placeholder}
    style={[styles.input, {
      maxHeight: maximumLines * 24 + 20,
      minHeight: Math.max(44, minimumLines * 24 + 20),
    }]}
    value={value}
  />;
}

const styles = StyleSheet.create({
  input: { textAlignVertical: 'top', width: '100%' },
});
