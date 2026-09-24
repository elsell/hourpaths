import { type ComponentProps } from 'react';
import { Button, Host, Image as SwiftUIImage, Menu } from '@expo/ui/swift-ui';
import { accessibilityLabel as nativeAccessibilityLabel, disabled as nativeDisabled } from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import type { NativeRouteAction } from './native-route-presentation';

type SwiftButtonImage = ComponentProps<typeof Button>['systemImage'];

export function NativeActionMenu({
  accessibilityLabel,
  actions,
}: {
  accessibilityLabel: string;
  actions: readonly NativeRouteAction[];
}) {
  return <View style={styles.container}>
    <Host style={styles.host}>
      <Menu
        label={<SwiftUIImage size={20} systemName="ellipsis" />}
        modifiers={[nativeAccessibilityLabel(accessibilityLabel)]}
      >
        {actions.map((action) => <Button
          key={action.label}
          label={action.label}
          modifiers={[nativeDisabled(action.disabled ?? false)]}
          onPress={action.onPress}
          role={action.destructive ? 'destructive' : undefined}
          systemImage={action.systemImage as SwiftButtonImage}
        />)}
      </Menu>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    height: 44,
    width: 44,
  },
  host: {
    height: 44,
    width: 44,
  },
});
