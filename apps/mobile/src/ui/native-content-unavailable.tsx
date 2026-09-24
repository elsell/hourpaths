import { StyleSheet, View } from 'react-native';
import { SectionHeading, Surface, ThemedText as Text } from './primitives';
import { NativeSystemImage } from './native-system-image';
import { mobileTheme } from './tokens';

export function NativeContentUnavailable({
  description,
  systemImage,
  title,
}: {
  description: string;
  systemImage: string;
  title: string;
}) {
  return <Surface>
    <View style={styles.content}>
      <NativeSystemImage systemName={systemImage} />
      <SectionHeading>{title}</SectionHeading>
      <Text style={styles.description}>{description}</Text>
    </View>
  </Surface>;
}

const styles = StyleSheet.create({
  content: {
    gap: mobileTheme.spacing.xs,
    minHeight: 120,
    paddingVertical: mobileTheme.spacing.lg,
  },
  description: {
    color: mobileTheme.colors.textMuted,
  },
});
