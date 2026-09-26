import { useEffect, useRef } from 'react';
import { Button, HStack, TextField } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  background,
  buttonStyle,
  clipShape,
  disabled as nativeDisabled,
  foregroundColor,
  frame,
  glassEffect,
  labelStyle,
  layoutPriority,
  lineLimit,
  padding,
  shapes,
  textFieldStyle,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet } from 'react-native';
import type { NativeCommentComposerProps } from './native-comment-composer-types';
import { NativeHost as Host } from './native-host';
import { mobileTheme } from './tokens';

type NativeTextFieldHandle = {
  blur: () => Promise<void>;
  focus: () => Promise<void>;
  setSelection: (start: number, end: number) => Promise<void>;
  setText: (value: string) => Promise<void>;
};

export function NativeCommentComposer({
  accessibilityLabel: inputLabel,
  busy,
  disabled,
  label,
  onChangeText,
  onPress,
  placeholder,
  value,
}: NativeCommentComposerProps) {
  const input = useRef<NativeTextFieldHandle>(null);
  const nativeValue = useRef(value);

  useEffect(() => {
    if (nativeValue.current === value) return;
    nativeValue.current = value;
    void input.current?.setText(value);
  }, [value]);

  return <Host matchContents={{ vertical: true }} style={styles.host}>
    <HStack
      alignment="center"
      modifiers={[
        padding({ bottom: 5, leading: 16, top: 5, trailing: 5 }),
        background(mobileTheme.colors.surfaceRaised, shapes.capsule({ roundedCornerStyle: 'continuous' })),
        glassEffect({
          glass: { tint: mobileTheme.colors.surfaceRaised, variant: 'regular' },
          shape: 'capsule',
        }),
      ]}
      spacing={8}
    >
      <TextField
        axis="vertical"
        defaultValue={value}
        modifiers={[
          accessibilityLabel(inputLabel),
          layoutPriority(1),
          lineLimit({ min: 1, max: 5 }),
          textFieldStyle('plain'),
        ]}
        onValueChange={(nextValue) => {
          nativeValue.current = nextValue;
          onChangeText(nextValue);
        }}
        placeholder={placeholder}
        ref={input}
      />
      <Button
        label={label}
        modifiers={[
          accessibilityLabel(label),
          buttonStyle('borderedProminent'),
          clipShape('circle'),
          frame({ height: 44, width: 44 }),
          foregroundColor(mobileTheme.colors.systemActionText),
          labelStyle('iconOnly'),
          nativeDisabled(busy || disabled),
          tint(mobileTheme.colors.systemAction),
        ]}
        onPress={() => { if (!busy && !disabled) onPress(); }}
        systemImage="arrow.up"
      />
    </HStack>
  </Host>;
}

const styles = StyleSheet.create({
  host: { minHeight: 54, width: '100%' },
});
