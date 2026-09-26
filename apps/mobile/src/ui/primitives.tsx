import { mobileShellStyles } from './shell-styles';
export { mobileShellStyles } from './shell-styles';
import { createElement, forwardRef, type ReactNode } from 'react';
import {
  ActivityIndicator,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
  type TextInputProps,
  type TextProps,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { NativeButton } from './native-button';
import { NativeSheetFrame } from './native-sheet-frame';
import { mobileTheme } from './tokens';

// Compatibility export for existing forms; all actions share one platform control.
export { NativeButton as ActionButton } from './native-button';

export function ScreenHeader({
  compact = false,
  eyebrow,
  title,
}: {
  compact?: boolean;
  eyebrow?: string;
  title: string;
}) {
  return <View style={styles.header}>
    {eyebrow ? <Text style={styles.eyebrow}>{eyebrow}</Text> : null}
    <Text accessibilityRole="header" style={[styles.title, compact ? styles.compactTitle : undefined]}>{title}</Text>
  </View>;
}

export function SectionHeading({ children }: { children: ReactNode }) {
  return <Text accessibilityRole="header" style={styles.heading}>{children}</Text>;
}

export function StatusBanner({
  actionLabel,
  onAction,
  text,
  tone = 'loading',
}: {
  actionLabel?: string;
  onAction?: () => void;
  text: string;
  tone?: 'loading' | 'offline' | 'error';
}) {
  return <View style={[styles.banner, tone === 'offline' ? styles.offlineBanner : null, tone === 'error' ? styles.errorBanner : null]}>
    <View
      accessible
      accessibilityLabel={text}
      accessibilityLiveRegion={tone === 'loading' ? 'polite' : 'assertive'}
      accessibilityRole={tone === 'loading' ? 'progressbar' : 'alert'}
      style={styles.bannerContent}
    >
      {tone === 'loading' ? <ActivityIndicator
        accessibilityElementsHidden
        color={mobileTheme.colors.accent}
        importantForAccessibility="no-hide-descendants"
      /> : null}
      <Text style={styles.bannerText}>{text}</Text>
    </View>
    {onAction && actionLabel ? <NativeButton label={actionLabel} onPress={onAction} variant="quiet" /> : null}
  </View>;
}

export function Surface({ children }: { children: ReactNode }) {
  return <View style={styles.surface}>{children}</View>;
}

export function ThemedText({ style, ...props }: TextProps) {
  return createElement(Text, {
    ...props,
    style: [styles.baseText, style],
  });
}

export const ThemedTextInput = forwardRef<TextInput, TextInputProps>(function ThemedTextInput(
  { style, ...props },
  ref,
) {
  return createElement(TextInput, {
    ...props,
    ref,
    placeholderTextColor: mobileTheme.colors.textMuted,
    selectionColor: mobileTheme.colors.accent,
    style: [styles.input, style],
  });
});

export function NativeSheet({
  animationType = 'slide',
  children,
  compact = false,
  dismissible = true,
  leadingAction,
  onRequestClose: closeSheet,
  scrollable = true,
  title,
  trailingAction,
  visible,
}: {
  animationType?: 'none' | 'slide';
  children: ReactNode;
  compact?: boolean;
  dismissible?: boolean;
  leadingAction?: { disabled?: boolean; label: string; onPress: () => void };
  onRequestClose: () => void;
  scrollable?: boolean;
  title?: string;
  trailingAction?: { disabled?: boolean; label: string; onPress: () => void };
  visible: boolean;
}) {

  return <Modal
    allowSwipeDismissal={dismissible}
    animationType={animationType}
    onRequestClose={dismissible ? closeSheet : undefined}
    presentationStyle="pageSheet"
    visible={visible}
  >
    <NativeSheetFrame title={title} leadingAction={leadingAction} trailingAction={trailingAction}>
    <SafeAreaView edges={['left', 'right', 'bottom']} style={mobileShellStyles.sheet}>
      {scrollable ? <ScrollView
        contentInsetAdjustmentBehavior="automatic"
        automaticallyAdjustKeyboardInsets
        contentContainerStyle={[
          mobileShellStyles.sheetContent,
          compact ? mobileShellStyles.sheetCompactContent : null,
        ]}
        keyboardDismissMode="interactive"
        keyboardShouldPersistTaps="handled"
      >
        {children}
      </ScrollView> : <View style={mobileShellStyles.sheetBody}>{children}</View>}
    </SafeAreaView>
    </NativeSheetFrame>
  </Modal>;
}


const styles = StyleSheet.create({
  banner: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderColor: mobileTheme.colors.border,
    borderRadius: mobileTheme.radii.md,
    borderWidth: mobileTheme.sizes.border,
    padding: mobileTheme.spacing.sm,
  },
  bannerContent: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
  },
  bannerText: {
    color: mobileTheme.colors.text,
    flexShrink: 1,
    ...mobileTheme.typography.body,
  },
  baseText: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
  },
  compactTitle: {
    ...mobileTheme.typography.subheading,
  },
  errorBanner: {
    backgroundColor: mobileTheme.colors.errorSurface,
    borderColor: mobileTheme.colors.error,
  },
  eyebrow: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  header: {
    gap: mobileTheme.spacing.xxs,
  },
  heading: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.subheading,
  },
  input: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderColor: mobileTheme.colors.border,
    borderRadius: mobileTheme.radii.md,
    borderWidth: mobileTheme.sizes.border,
    color: mobileTheme.colors.text,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.sm,
    paddingVertical: mobileTheme.spacing.xs,
    ...mobileTheme.typography.body,
  },
  offlineBanner: {
    backgroundColor: mobileTheme.colors.offlineSurface,
    borderColor: mobileTheme.colors.accent,
  },
  surface: {
    backgroundColor: mobileTheme.colors.surface,
    borderRadius: mobileTheme.radii.md,
    gap: mobileTheme.spacing.sm,
    padding: mobileTheme.spacing.md,
  },
  title: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.title,
  },
});
