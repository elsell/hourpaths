import { createElement, forwardRef, type ReactNode } from 'react';
import {
  ActivityIndicator,
  Button,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
  useWindowDimensions,
  type AccessibilityState,
  type TextInputProps,
  type TextProps,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { NativeSheetAction } from './native-sheet-action';
import { mobileTheme } from './tokens';

type ActionButtonProps = {
  accessibilityLabel?: string;
  busy?: boolean;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  selected?: AccessibilityState['selected'];
  variant?: 'primary' | 'secondary' | 'quiet' | 'danger';
};

export function ActionButton({
  accessibilityLabel,
  busy = false,
  disabled = false,
  label,
  onPress,
  selected,
  variant = 'primary',
}: ActionButtonProps) {
  const color = variant === 'primary'
    ? mobileTheme.colors.accentText
    : variant === 'danger'
      ? mobileTheme.colors.error
      : mobileTheme.colors.accent;

  return <View style={[styles.action, styles[`${variant}Action`], disabled ? styles.actionDisabled : null]}>
    <Button
      accessibilityLabel={accessibilityLabel}
      accessibilityState={{ busy, disabled, selected }}
      color={color}
      disabled={disabled}
      onPress={onPress}
      title={label}
    />
  </View>;
}

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
    {onAction ? <Button color={mobileTheme.colors.accent} onPress={onAction} title={actionLabel ?? ''} /> : null}
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
  const { fontScale, width } = useWindowDimensions();
  const stackSheetHeader = needsCompactVerticalLayout(width, fontScale);
  const leadingHeaderAction = <View style={mobileShellStyles.sheetHeaderAction}>
    {leadingAction ? <NativeSheetAction
      disabled={leadingAction.disabled}
      label={leadingAction.label}
      onPress={leadingAction.onPress}
    /> : null}
  </View>;
  const trailingHeaderAction = <View style={[mobileShellStyles.sheetHeaderAction, mobileShellStyles.sheetHeaderTrailing]}>
    {trailingAction ? <NativeSheetAction
      disabled={trailingAction.disabled}
      label={trailingAction.label}
      onPress={trailingAction.onPress}
    /> : null}
  </View>;

  return <Modal
    allowSwipeDismissal={dismissible}
    animationType={animationType}
    onRequestClose={dismissible ? closeSheet : undefined}
    presentationStyle="pageSheet"
    visible={visible}
  >
    <SafeAreaView style={mobileShellStyles.sheet}>
      {title ? <View style={[mobileShellStyles.sheetHeader, stackSheetHeader ? mobileShellStyles.sheetHeaderStacked : null]}>
        {stackSheetHeader ? <>
          <Text accessibilityRole="header" style={[mobileShellStyles.sheetTitle, mobileShellStyles.sheetTitleStacked]}>{title}</Text>
          <View style={mobileShellStyles.sheetHeaderStackedActions}>
            {leadingHeaderAction}
            {trailingHeaderAction}
          </View>
        </> : <>
          {leadingHeaderAction}
          <Text accessibilityRole="header" style={mobileShellStyles.sheetTitle}>{title}</Text>
          {trailingHeaderAction}
        </>}
      </View> : null}
      {scrollable ? <ScrollView
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
  </Modal>;
}

export const mobileShellStyles = StyleSheet.create({
  sheetBody: {
    flex: 1,
  },
  emptyState: {
    gap: mobileTheme.spacing.sm,
    paddingVertical: mobileTheme.spacing.lg,
  },
  screen: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingTop: mobileTheme.spacing.sm,
  },
  sheet: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
  },
  sheetCompactContent: {
    gap: mobileTheme.spacing.lg,
    padding: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  sheetContent: {
    gap: mobileTheme.spacing.md,
    padding: mobileTheme.spacing.lg,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  sheetHeader: {
    alignItems: 'center',
    borderBottomColor: mobileTheme.colors.surfaceRaised,
    borderBottomWidth: StyleSheet.hairlineWidth,
    flexDirection: 'row',
    minHeight: 52,
    paddingHorizontal: mobileTheme.spacing.xs,
  },
  sheetHeaderAction: {
    alignItems: 'flex-start',
    flexShrink: 1,
    maxWidth: '45%',
    minWidth: 72,
  },
  sheetHeaderStacked: {
    alignItems: 'stretch',
    flexDirection: 'column',
    gap: mobileTheme.spacing.xxs,
    paddingTop: mobileTheme.spacing.sm,
  },
  sheetHeaderStackedActions: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
    width: '100%',
  },
  sheetHeaderTrailing: {
    alignItems: 'flex-end',
  },
  sheetTitle: {
    color: mobileTheme.colors.text,
    flex: 1,
    fontSize: 17,
    fontWeight: '600',
    lineHeight: 22,
    paddingHorizontal: mobileTheme.spacing.xs,
    textAlign: 'center',
  },
  sheetTitleStacked: {
    flex: 0,
  },
  scrollContent: {
    gap: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  groupedScrollContent: {
    gap: mobileTheme.spacing.lg,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingTop: mobileTheme.spacing.sm,
  },
  stack: {
    gap: mobileTheme.spacing.md,
  },
  text: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
  },
  textMuted: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.body,
  },
});

const styles = StyleSheet.create({
  action: {
    borderRadius: mobileTheme.radii.md,
    borderWidth: mobileTheme.sizes.border,
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    overflow: 'hidden',
    paddingHorizontal: mobileTheme.spacing.xs,
  },
  actionDisabled: {
    opacity: 0.48,
  },
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
  dangerAction: {
    backgroundColor: mobileTheme.colors.errorSurface,
    borderColor: mobileTheme.colors.error,
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
    ...mobileTheme.typography.heading,
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
  primaryAction: {
    backgroundColor: mobileTheme.colors.accent,
    borderColor: mobileTheme.colors.accent,
  },
  quietAction: {
    backgroundColor: 'transparent',
    borderColor: 'transparent',
  },
  secondaryAction: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderColor: mobileTheme.colors.border,
  },
  surface: {
    backgroundColor: mobileTheme.colors.surface,
    borderColor: mobileTheme.colors.border,
    borderRadius: mobileTheme.radii.lg,
    borderWidth: mobileTheme.sizes.border,
    gap: mobileTheme.spacing.sm,
    padding: mobileTheme.spacing.md,
  },
  title: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.title,
  },
});
