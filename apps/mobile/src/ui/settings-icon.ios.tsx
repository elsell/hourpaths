import { NativeHost as Host } from './native-host';
import { type ComponentProps } from 'react';
import { Image as SwiftUIImage } from '@expo/ui/swift-ui';
import { StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

type SystemName = ComponentProps<typeof SwiftUIImage>['systemName'];

export function SettingsIcon({
  systemName,
  variant = 'settings',
}: {
  systemName: SystemName;
  variant?: 'disclosure' | 'settings';
}) {
  return <View
    accessibilityElementsHidden
    importantForAccessibility="no-hide-descendants"
    style={[styles.frame, variant === 'disclosure' ? styles.disclosureFrame : null]}
  >
    <Host style={variant === 'disclosure' ? styles.disclosureHost : styles.host}>
      <SwiftUIImage
        color={variant === 'disclosure' ? mobileTheme.colors.textMuted : mobileTheme.colors.accent}
        size={variant === 'disclosure' ? 14 : 22}
        systemName={systemName}
      />
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  frame: {
    alignItems: 'center',
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderRadius: mobileTheme.radii.sm,
    height: 34,
    justifyContent: 'center',
    width: 34,
  },
  disclosureFrame: {
    backgroundColor: 'transparent',
    borderRadius: 0,
    width: 20,
  },
  disclosureHost: {
    height: 20,
    width: 14,
  },
  host: {
    height: 24,
    width: 24,
  },
});
