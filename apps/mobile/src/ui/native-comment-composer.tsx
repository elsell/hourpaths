import { StyleSheet, View } from 'react-native';
import { NativeButton } from './native-button';
import type { NativeCommentComposerProps } from './native-comment-composer-types';
import { NativeCommentInput } from './native-comment-input';
import { mobileTheme } from './tokens';

export function NativeCommentComposer({
  accessibilityLabel,
  busy,
  disabled,
  label,
  onChangeText,
  onPress,
  placeholder,
  stacked = false,
  value,
}: NativeCommentComposerProps) {
  return <View style={[styles.container, stacked && styles.stacked]}>
    <View style={[styles.input, stacked && styles.stackedInput]}>
      <NativeCommentInput
        accessibilityLabel={accessibilityLabel}
        onChangeText={onChangeText}
        placeholder={placeholder}
        value={value}
      />
    </View>
    <NativeButton busy={busy} disabled={disabled} label={label} onPress={onPress} />
  </View>;
}

const styles = StyleSheet.create({
  container: { alignItems: 'flex-end', flexDirection: 'row', gap: mobileTheme.spacing.sm, width: '100%' },
  input: { flex: 1, maxHeight: 120, minHeight: 44 },
  stacked: { alignItems: 'stretch', flexDirection: 'column' },
  stackedInput: { flex: 0 },
});
