import { NUDGE_PRESETS, type NudgePreset } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Pressable, StyleSheet, View } from 'react-native';
import { NativeSheet, ThemedText as Text } from './primitives';
import { SettingsIcon } from './settings-icon';
import { SettingsSection, SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

export function NudgeComposerSheet({
  busy,
  errorText,
  i18n,
  onCancel,
  onSelect,
  onSend,
  pathName,
  recipientDisplayName,
  selectedPreset,
  visible,
}: {
  busy: boolean;
  errorText?: string;
  i18n: Translator;
  onCancel: () => void;
  onSelect: (preset: NudgePreset) => void;
  onSend: () => void;
  pathName: string;
  recipientDisplayName: string;
  selectedPreset: NudgePreset | null;
  visible: boolean;
}) {
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  const [admittedActionKey, setAdmittedActionKey] = useState<'common.retry' | 'nudge.compose.send' | null>(null);
  const wasBusy = useRef(false);
  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => subscription.remove();
  }, []);
  useEffect(() => {
    if (wasBusy.current && !busy) setAdmittedActionKey(null);
    wasBusy.current = busy;
  }, [busy]);

  const currentActionKey = errorText ? 'common.retry' : 'nudge.compose.send';
  const submit = () => {
    setAdmittedActionKey(currentActionKey);
    onSend();
  };

  return <NativeSheet
    animationType={reduceMotion === false ? 'slide' : 'none'}
    compact
    dismissible={!busy}
    leadingAction={{ disabled: busy, label: i18n.t('nudge.compose.cancel'), onPress: onCancel }}
    onRequestClose={onCancel}
    title={i18n.t('nudge.compose.title', { displayName: recipientDisplayName })}
    trailingAction={{
      disabled: busy || selectedPreset === null,
      label: i18n.t(busy && admittedActionKey ? admittedActionKey : currentActionKey),
      onPress: submit,
    }}
    visible={visible}
  >
    <Text style={styles.context}>{i18n.t('nudge.compose.pathContext', { pathName })}</Text>
    <View accessibilityRole="radiogroup">
    <SettingsSection title={i18n.t('nudge.compose.choosePreset')}>
      {NUDGE_PRESETS.map((preset, index) => {
        const selected = selectedPreset === preset;
        return <View key={preset}>
          {index > 0 ? <SettingsSeparator /> : null}
          <Pressable
            accessibilityLabel={i18n.t(`nudge.preset.${preset}`)}
            accessibilityRole="radio"
            accessibilityState={{ disabled: busy, selected }}
            disabled={busy}
            onPress={() => { if (!selected) onSelect(preset); }}
            style={({ pressed }) => [styles.row, pressed ? styles.pressed : null]}
          >
            <Text style={styles.label}>{i18n.t(`nudge.preset.${preset}`)}</Text>
            <View style={styles.check}>{selected ? <SettingsIcon systemName="checkmark" variant="disclosure" /> : null}</View>
          </Pressable>
        </View>;
      })}
    </SettingsSection>
    </View>
    {busy ? <Text accessibilityLiveRegion="polite" style={styles.status}>
      {i18n.t('nudge.compose.sending')}
    </Text> : null}
    {errorText ? <Text accessibilityRole="alert" style={styles.error}>{errorText}</Text> : null}
  </NativeSheet>;
}

const styles = StyleSheet.create({
  check: { alignItems: 'center', minWidth: 24 },
  context: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20, textAlign: 'center' },
  error: { color: mobileTheme.colors.error, fontSize: 15, lineHeight: 20 },
  label: { flex: 1, fontSize: 17, lineHeight: 22 },
  pressed: { backgroundColor: mobileTheme.colors.surfaceRaised },
  row: { alignItems: 'center', flexDirection: 'row', minHeight: 52, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  status: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20, textAlign: 'center' },
});
