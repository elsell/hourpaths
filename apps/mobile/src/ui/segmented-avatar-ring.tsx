import { Image as ExpoImage } from 'expo-image';
import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { mobileTheme } from './tokens';

/** The enclosing button supplies the person and active-Path accessibility label. */
export function SegmentedAvatarRing({ count, avatarSize = 56, color = mobileTheme.colors.accent, children }: {
  count: number;
  avatarSize?: number;
  color?: string;
  children: ReactNode;
}) {
  const stroke = 2;
  const size = avatarSize + 2 * (stroke + mobileTheme.spacing.xxs);
  const radius = (size - stroke) / 2;
  const segments = Math.max(0, Math.floor(count));
  const circumference = 2 * Math.PI * radius;
  const segmentLength = circumference / Math.max(1, segments);
  const gap = segments > 1 ? Math.min(4, segmentLength / 3) : 0;
  // The image is a local SVG using the existing Expo image adapter. Its dashed
  // circumference draws equal segments without depending on the page background.
  const svg = (`<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}"><circle cx="${size / 2}" cy="${size / 2}" r="${radius}" fill="none" stroke="${color.replace(/["<>&]/g, '')}" stroke-width="${stroke}" stroke-dasharray="${segmentLength - gap} ${gap}" transform="rotate(-90 ${size / 2} ${size / 2})"/></svg>`);
  return <View style={[styles.frame, { height: size, width: size }]}>
    {segments > 0 ? <ExpoImage accessibilityElementsHidden importantForAccessibility="no-hide-descendants" pointerEvents="none"
      source={{ uri: `data:image/svg+xml;utf8,${encodeURIComponent(svg)}` }} contentFit="contain" style={StyleSheet.absoluteFill} /> : null}
    {children}
  </View>;
}

const styles = StyleSheet.create({
  frame: { alignItems: 'center', justifyContent: 'center' },
});
