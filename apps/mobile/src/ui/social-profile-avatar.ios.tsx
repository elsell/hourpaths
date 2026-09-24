import { Host, Image as SwiftUIImage } from '@expo/ui/swift-ui';
import { Image as ExpoImage } from 'expo-image';
import { StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

export function SocialProfileAvatar({
  accessibilityLabel,
  profilePictureURL,
  size = 48,
}: {
  accessibilityLabel: string;
  profilePictureURL?: string;
  size?: number;
}) {
  const frame = { borderRadius: size / 2, height: size, width: size };
  return <View
    accessibilityLabel={accessibilityLabel}
    accessibilityRole="image"
    style={[styles.blank, frame]}
  >
    {profilePictureURL ? <ExpoImage
      accessibilityElementsHidden
      contentFit="cover"
      source={{ uri: profilePictureURL }}
      style={frame}
    /> : <Host matchContents style={frame}>
      <SwiftUIImage
        color={mobileTheme.colors.textMuted}
        size={size}
        systemName="person.crop.circle.fill"
      />
    </Host>}
  </View>;
}

const styles = StyleSheet.create({
  blank: {
    alignItems: 'center',
    backgroundColor: mobileTheme.colors.surfaceRaised,
    justifyContent: 'center',
    overflow: 'hidden',
  },
});
