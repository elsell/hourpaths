import { DropdownMenu, DropdownMenuItem, Host, RNHostView, Text } from '@expo/ui/jetpack-compose';
import { useState } from 'react';
import { Pressable, StyleSheet } from 'react-native';
import type { NativeMenuAction } from './native-menu-action';
import { PlatformSymbol } from './platform-symbol';
import { mobileTheme } from './tokens';

export function NativeActionMenu({ accessibilityLabel, actions }: {
  accessibilityLabel: string;
  actions: readonly NativeMenuAction[];
}) {
  const [expanded, setExpanded] = useState(false);
  return <Host colorScheme="dark" style={styles.anchor}>
    <DropdownMenu expanded={expanded} onDismissRequest={() => setExpanded(false)} color={mobileTheme.colors.surfaceRaised}>
      <DropdownMenu.Trigger>
        <RNHostView matchContents>
          <Pressable accessibilityLabel={accessibilityLabel} accessibilityRole="button"
            accessibilityState={{ expanded }} onPress={() => setExpanded(true)}
            android_ripple={{ color: mobileTheme.colors.surfacePressed, borderless: true }} style={styles.anchor}>
            <PlatformSymbol systemName="ellipsis" color={mobileTheme.colors.accent} />
          </Pressable>
        </RNHostView>
      </DropdownMenu.Trigger>
      <DropdownMenu.Items>
        {actions.map((action) => <DropdownMenuItem key={action.label} enabled={!action.disabled}
          elementColors={{ textColor: action.destructive ? mobileTheme.colors.error : mobileTheme.colors.text, disabledTextColor: mobileTheme.colors.textMuted }}
          onClick={() => {
            if (action.disabled) return;
            setExpanded(false);
            action.onPress();
          }}>
          <DropdownMenuItem.Text><Text>{action.label}</Text></DropdownMenuItem.Text>
        </DropdownMenuItem>)}
      </DropdownMenu.Items>
    </DropdownMenu>
  </Host>;
}
const styles = StyleSheet.create({
  anchor: { alignItems: 'center', justifyContent: 'center', height: 48, width: 48 },
});
