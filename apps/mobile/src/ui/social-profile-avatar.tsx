import { StyleSheet, View } from 'react-native';
import { Image as ExpoImage } from 'expo-image';
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
    /> : <>
      <View style={[styles.head, { borderRadius: size * 0.12, height: size * 0.24, width: size * 0.24 }]} />
      <View style={[
      styles.shoulders,
      {
        borderTopLeftRadius: size * 0.24,
        borderTopRightRadius: size * 0.24,
        height: size * 0.24,
        width: size * 0.48,
      },
      ]} />
    </>}
  </View>;
}

const styles = StyleSheet.create({
  blank: {
    alignItems: 'center',
    backgroundColor: mobileTheme.colors.surfaceRaised,
    justifyContent: 'center',
  },
  head: {
    backgroundColor: mobileTheme.colors.textMuted,
    marginBottom: 2,
  },
  shoulders: {
    backgroundColor: mobileTheme.colors.textMuted,
  },
});
