import type { ComponentProps } from 'react';
import { Host } from '@expo/ui/swift-ui';
import { tint } from '@expo/ui/swift-ui/modifiers';
import { mobileTheme } from './tokens';

type NativeHostProps = Pick<ComponentProps<typeof Host>, 'children' | 'matchContents' | 'style' | 'modifiers' | 'colorScheme'>;

// Every embedded SwiftUI control belongs to the same appearance and action palette.
export function NativeHost({ children, matchContents, style, modifiers = [] }: NativeHostProps) {
  return <Host colorScheme="dark" matchContents={matchContents} style={style} modifiers={[tint(mobileTheme.colors.accent), ...modifiers]}>{children}</Host>;
}
