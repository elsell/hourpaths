import type { ComponentProps } from 'react';
import { Host } from '@expo/ui/swift-ui';
import { tint } from '@expo/ui/swift-ui/modifiers';
import { mobileTheme } from './tokens';

// Every embedded SwiftUI control belongs to the same appearance and action palette.
export function NativeHost({ modifiers = [], ...props }: ComponentProps<typeof Host>) {
  return <Host colorScheme="dark" {...props} modifiers={[tint(mobileTheme.colors.accent), ...modifiers]} />;
}
