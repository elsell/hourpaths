import type { ComponentProps } from 'react';
import { Host } from '@expo/ui/swift-ui';
import { tint } from '@expo/ui/swift-ui/modifiers';
import { mobileTheme } from './tokens';

type NativeHostProps = Pick<ComponentProps<typeof Host>, 'children' | 'matchContents' | 'style' | 'modifiers' | 'colorScheme' | 'useViewportSizeMeasurement'>;

// Every embedded SwiftUI control belongs to the same appearance and action palette.
export function NativeHost({ children, matchContents, style, useViewportSizeMeasurement, modifiers = [] }: NativeHostProps) {
  return <Host useViewportSizeMeasurement={useViewportSizeMeasurement} colorScheme="dark" matchContents={matchContents} style={style} modifiers={[tint(mobileTheme.colors.accent), ...modifiers]}>{children}</Host>;
}
