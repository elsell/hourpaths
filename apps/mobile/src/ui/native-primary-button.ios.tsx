import { type ComponentProps, useState } from 'react';
import { Button, Host, Text as NativeText } from '@expo/ui/swift-ui';
import {
  accessibilityLabel as nativeAccessibilityLabel,
  buttonStyle,
  controlSize,
  disabled as nativeDisabled,
  frame,
  foregroundColor,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { type LayoutChangeEvent, StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

export function NativePrimaryButton({
  disabled = false,
  fullWidth = false,
  label,
  onPress,
  systemImage,
  variant = 'prominent',
}: {
  disabled?: boolean;
  fullWidth?: boolean;
  label: string;
  onPress: () => void;
  systemImage?: ComponentProps<typeof Button>['systemImage'];
  variant?: 'plain' | 'prominent';
}) {
  const [measuredWidth, setMeasuredWidth] = useState(0);
  const styleModifier = variant === 'plain'
    ? buttonStyle('plain')
    : buttonStyle('borderedProminent');
  const foregroundModifier = variant === 'plain'
    ? foregroundColor(mobileTheme.colors.accent)
    : foregroundColor(mobileTheme.colors.accentText);
  const captureFullWidth = (event: LayoutChangeEvent) => {
    const nextWidth = Math.round(event.nativeEvent.layout.width);
    if (nextWidth > 0 && nextWidth !== measuredWidth) setMeasuredWidth(nextWidth);
  };
  const measuredLabelWidth = Math.max(0, measuredWidth - (mobileTheme.spacing.md * 2));
  return <View
    onLayout={fullWidth ? captureFullWidth : undefined}
    pointerEvents={disabled ? 'none' : 'auto'}
    style={[styles.container, fullWidth && styles.fullWidth]}
  >
    <Host
      matchContents={fullWidth ? { vertical: true } : true}
      style={[styles.host, fullWidth && styles.fullWidth]}
    >
      <Button
        label={fullWidth ? undefined : label}
        modifiers={[
          nativeAccessibilityLabel(label),
          controlSize('large'),
          styleModifier,
          tint(mobileTheme.colors.accent),
          foregroundModifier,
          nativeDisabled(disabled),
        ]}
        onPress={onPress}
        systemImage={systemImage}
      >
        {fullWidth ? <NativeText modifiers={[
          frame({ width: measuredLabelWidth || undefined }),
          foregroundColor(mobileTheme.colors.accentText),
        ]}>{label}</NativeText> : undefined}
      </Button>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: 50,
  },
  fullWidth: {
    width: '100%',
  },
  host: {
    minHeight: 50,
  },
});
