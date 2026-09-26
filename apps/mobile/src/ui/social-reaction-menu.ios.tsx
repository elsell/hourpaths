import { NativeHost as Host } from './native-host';
import { Button, Image as SwiftUIImage, Menu } from '@expo/ui/swift-ui';
import {
  accessibilityLabel as nativeAccessibilityLabel,
  disabled as nativeDisabled,
  frame,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import type { SocialReactionMenuChoice } from './social-reaction-presentation';
import { mobileTheme } from './tokens';

export function SocialReactionMenu({
  accessibilityLabel,
  busy,
  choices,
  onRemove,
  removeLabel,
  selectedEmoji,
}: {
  accessibilityLabel: string;
  busy: boolean;
  cancelLabel: string;
  choices: readonly SocialReactionMenuChoice[];
  onRemove?: () => void;
  removeLabel: string;
  selectedEmoji?: string;
}) {
  return <View
    accessibilityState={{ busy, disabled: busy, selected: selectedEmoji !== undefined }}
    pointerEvents={busy ? 'none' : 'auto'}
    style={styles.container}
  >
    <Host style={styles.host}>
      <Menu
        label={<SwiftUIImage systemName={selectedEmoji ? 'heart.fill' : 'heart'} />}
        modifiers={[
          nativeAccessibilityLabel(accessibilityLabel),
          frame({ height: 44, minWidth: 44 }),
          tint(mobileTheme.colors.accent),
          nativeDisabled(busy),
        ]}
      >
        {choices.map((choice) => <Button
          key={choice.type}
          label={choice.menuLabel}
          onPress={choice.onPress}
          systemImage={choice.selected ? 'checkmark' : undefined}
        />)}
        {onRemove ? <Button
          label={removeLabel}
          onPress={onRemove}
          systemImage="xmark.circle"
        /> : null}
      </Menu>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    height: 44,
    minWidth: 44,
  },
  host: { height: 44, minWidth: 44 },
});
