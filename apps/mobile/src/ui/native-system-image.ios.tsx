import { NativeHost as Host } from './native-host';
import { type ComponentProps } from 'react';
import { Image as SwiftUIImage } from '@expo/ui/swift-ui';
import { StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

type SystemName = ComponentProps<typeof SwiftUIImage>['systemName'];

export function NativeSystemImage({ systemName }: { systemName: SystemName }) {
  return <View
    accessibilityElementsHidden
    importantForAccessibility="no-hide-descendants"
    style={styles.container}
  >
    <Host style={styles.host}>
      <SwiftUIImage color={mobileTheme.colors.textMuted} size={32} systemName={systemName} />
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    height: 40,
    width: 40,
  },
  host: {
    height: 40,
    width: 40,
  },
});
