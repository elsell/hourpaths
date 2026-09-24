import { useEffect, useState } from 'react';
import { AccessibilityInfo, Modal, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { ThemedText as Text } from './primitives';
import type { SocialReactionMenuChoice } from './social-reaction-presentation';
import { mobileTheme } from './tokens';

export function SocialReactionMenu({
  accessibilityLabel,
  busy,
  cancelLabel,
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
  const [visible, setVisible] = useState(false);
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => subscription.remove();
  }, []);
  const closeAndRun = (action: () => void) => {
    setVisible(false);
    action();
  };

  return <>
    <Pressable
      accessibilityLabel={accessibilityLabel}
      accessibilityRole="button"
      accessibilityState={{ busy, disabled: busy, selected: selectedEmoji !== undefined }}
      disabled={busy}
      onPress={() => setVisible(true)}
      style={({ pressed }) => [
        styles.control,
        selectedEmoji ? styles.selected : null,
        pressed ? styles.pressed : null,
        busy ? styles.disabled : null,
      ]}
    >
      <Text style={[styles.label, selectedEmoji ? styles.selectedLabel : null]}>
        {selectedEmoji ?? '♡'}
      </Text>
    </Pressable>
    <Modal
      animationType={reduceMotion === false ? 'fade' : 'none'}
      onRequestClose={() => setVisible(false)}
      transparent
      visible={visible}
    >
      <View accessibilityViewIsModal style={styles.modal}>
        <Pressable
          accessibilityLabel={cancelLabel}
          accessibilityRole="button"
          onPress={() => setVisible(false)}
          style={StyleSheet.absoluteFill}
        />
        <View style={styles.sheet}>
          <Text accessibilityRole="header" style={styles.heading}>{accessibilityLabel}</Text>
          <ScrollView contentContainerStyle={styles.choices}>
            {choices.map((choice) => <Pressable
              accessibilityLabel={choice.label}
              accessibilityRole="button"
              accessibilityState={{ selected: choice.selected }}
              key={choice.type}
              onPress={() => closeAndRun(choice.onPress)}
              style={({ pressed }) => [styles.choice, pressed ? styles.pressed : null]}
            >
              <Text style={styles.choiceEmoji}>{choice.emoji}</Text>
              <Text style={[styles.choiceLabel, choice.selected ? styles.selectedLabel : null]}>
                {choice.label}
              </Text>
              {choice.selected ? <Text style={styles.selectedLabel}>✓</Text> : null}
            </Pressable>)}
            {onRemove ? <Pressable
              accessibilityRole="button"
              onPress={() => closeAndRun(onRemove)}
              style={({ pressed }) => [styles.choice, pressed ? styles.pressed : null]}
            >
              <Text style={styles.removeLabel}>{removeLabel}</Text>
            </Pressable> : null}
            <Pressable
              accessibilityRole="button"
              onPress={() => setVisible(false)}
              style={({ pressed }) => [styles.choice, pressed ? styles.pressed : null]}
            >
              <Text style={styles.choiceLabel}>{cancelLabel}</Text>
            </Pressable>
          </ScrollView>
        </View>
      </View>
    </Modal>
  </>;
}

const styles = StyleSheet.create({
  control: {
    alignItems: 'center',
    borderColor: mobileTheme.colors.border,
    borderRadius: mobileTheme.radii.pill,
    borderWidth: StyleSheet.hairlineWidth,
    height: 44,
    justifyContent: 'center',
    minWidth: 44,
    paddingHorizontal: mobileTheme.spacing.sm,
  },
  choice: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: 48,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  choiceEmoji: { fontSize: 24 },
  choiceLabel: {
    color: mobileTheme.colors.text,
    flex: 1,
    ...mobileTheme.typography.body,
  },
  choices: { paddingBottom: mobileTheme.spacing.sm },
  disabled: { opacity: 0.55 },
  heading: {
    color: mobileTheme.colors.text,
    padding: mobileTheme.spacing.md,
    ...mobileTheme.typography.subheading,
  },
  label: { color: mobileTheme.colors.textMuted, fontSize: 20 },
  pressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  modal: {
    backgroundColor: 'rgba(0, 0, 0, 0.45)',
    flex: 1,
    justifyContent: 'flex-end',
  },
  removeLabel: {
    color: mobileTheme.colors.error,
    ...mobileTheme.typography.body,
  },
  selected: { borderColor: mobileTheme.colors.accent },
  selectedLabel: { color: mobileTheme.colors.accent },
  sheet: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderTopLeftRadius: mobileTheme.radii.lg,
    borderTopRightRadius: mobileTheme.radii.lg,
    maxHeight: '80%',
    overflow: 'hidden',
    paddingBottom: mobileTheme.spacing.lg,
  },
});
