import { NativeHost as Host } from './native-host';
import { useState } from 'react';
import { Button, Text as NativeText } from '@expo/ui/swift-ui';
import { accessibilityLabel as nativeAccessibilityLabel, buttonStyle, controlSize, contentShape, shapes, disabled as nativeDisabled, frame, foregroundColor, tint } from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import type { NativeButtonProps } from './native-button-types';
import { mobileTheme } from './tokens';

export function NativeButton({ accessibilityLabel, busy = false, disabled = false, fullWidth = false, label, onPress, selected, systemImage, tone, variant = 'primary' }: NativeButtonProps) {
  const [width, setWidth] = useState(0);
  const unavailable = disabled || busy;
  const color = tone?.foreground ?? (variant === 'primary' ? mobileTheme.colors.accentText : variant === 'danger' ? mobileTheme.colors.error : mobileTheme.colors.accent);
  return <View
    accessible
    accessibilityLabel={accessibilityLabel ?? label}
    accessibilityRole="button"
    onAccessibilityTap={() => { if (!unavailable) onPress(); }}
    accessibilityState={{ busy, disabled: unavailable, selected }}
    onLayout={fullWidth ? (event) => setWidth(Math.round(event.nativeEvent.layout.width)) : undefined}
    pointerEvents={unavailable ? 'none' : 'auto'}
    style={[styles.container, tone ? { backgroundColor: tone.background, borderRadius: mobileTheme.radii.md } : null, fullWidth && styles.fullWidth]}
  >
    <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={fullWidth ? styles.fullWidth : undefined}>
    <Host useViewportSizeMeasurement colorScheme="dark" matchContents={fullWidth ? { vertical: true } : true} style={fullWidth ? styles.fullWidth : undefined}>
      <Button
        label={fullWidth ? undefined : label}
        modifiers={[
          nativeAccessibilityLabel(accessibilityLabel ?? label),
          buttonStyle(tone ? 'plain' : variant === 'primary' ? 'borderedProminent' : variant === 'quiet' ? 'plain' : 'bordered'),
          controlSize('large'),
          tint(variant === 'danger' ? mobileTheme.colors.error : mobileTheme.colors.accent),
          foregroundColor(color),
          nativeDisabled(unavailable),
        ]}
        onPress={() => { if (!unavailable) onPress(); }}
        role={variant === 'danger' ? 'destructive' : undefined}
        systemImage={fullWidth ? undefined : systemImage as React.ComponentProps<typeof Button>['systemImage']}
      >
        {fullWidth ? <NativeText modifiers={[
          frame({ width: width > 0 ? Math.max(0, width - (tone ? mobileTheme.spacing.sm : mobileTheme.spacing.lg) * 2) : undefined, minHeight: tone ? mobileTheme.sizes.minimumTouchTarget : undefined }),
          contentShape(shapes.rectangle()),
          foregroundColor(color),
        ]}>{systemImage === 'stop.fill' ? '■ ' : systemImage === 'play.fill' ? '▶ ' : ''}{label}</NativeText> : undefined}
      </Button>
    </Host>
    </View>
  </View>;
}
const styles = StyleSheet.create({
  container: { alignItems: 'center', justifyContent: 'center', minHeight: mobileTheme.sizes.minimumTouchTarget },
  fullWidth: { width: '100%' },
});
