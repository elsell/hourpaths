import { getLocales } from 'expo-localization';
import { useEffect, useState } from 'react';
import { AccessibilityInfo, StyleSheet, View } from 'react-native';
import type { ManualActivityFormState, ManualActivityLocalDateTime } from '@hourpaths/client-core';
import { createDeviceTranslator } from '../i18n';
import { HumanDurationEditor } from './human-duration-editor';
import { ManualOccurrenceFields } from './manual-occurrence-fields';
import {
  NativeSheet,
  StatusBanner,
  ThemedText,
  ThemedTextInput,
} from './primitives';
import { NativePrimaryButton } from './native-primary-button';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function ManualActivityForm({
  activityVersion,
  busy,
  editing,
  errorText,
  form,
  note,
  onCancel,
  onChangeDuration,
  onChangeNote,
  onChangeOccurrence,
  onSave,
  timeZone,
}: {
  activityVersion?: number;
  busy: boolean;
  editing: boolean;
  errorText?: string;
  form: ManualActivityFormState;
  note: string;
  onCancel: () => void;
  onChangeDuration: (durationSeconds: string) => void;
  onChangeNote: (note: string) => void;
  onChangeOccurrence: (patch: Partial<ManualActivityLocalDateTime>) => void;
  onSave: () => void;
  timeZone: string;
}) {
  const [noteExpanded, setNoteExpanded] = useState(Boolean(note));
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => subscription.remove();
  }, []);

  return <NativeSheet
    animationType={reduceMotion === false ? 'slide' : 'none'}
    compact
    dismissible={!busy}
    leadingAction={{ disabled: busy, label: i18n.t('common.cancel'), onPress: onCancel }}
    onRequestClose={onCancel}
    title={i18n.t(editing ? 'activity.editHeading' : 'activity.addHeading')}
    trailingAction={{
      disabled: busy,
      label: i18n.t(errorText ? 'common.retry' : editing ? 'activity.saveEdit' : 'activity.save'),
      onPress: onSave,
    }}
    visible
  >
    <View style={styles.section}>
      <ManualOccurrenceFields busy={busy} form={form} onChange={onChangeOccurrence} />
      <ThemedText style={styles.supporting}>
        {i18n.t('activity.timeZone', { timeZone })}
      </ThemedText>
    </View>

    <View style={styles.section}>
      <HumanDurationEditor
        busy={busy}
        compact
        label={i18n.t('activity.durationValue')}
        onChange={onChangeDuration}
        seconds={form.durationSeconds}
        unitLabels={{
          hours: i18n.t('activity.duration.hours'),
          minutes: i18n.t('activity.duration.minutes'),
          seconds: i18n.t('activity.duration.seconds'),
        }}
      />
    </View>

    <NativePrimaryButton
      label={i18n.t(noteExpanded ? 'activity.hideNote' : 'activity.addNote')}
      onPress={() => setNoteExpanded((expanded) => !expanded)}
      variant="plain"
    />
    {noteExpanded ? <View style={styles.section}>
      <View style={styles.field}>
        <ThemedText style={styles.label}>{i18n.t('activity.note')}</ThemedText>
        <ThemedTextInput
          accessibilityLabel={i18n.t('activity.note')}
          editable={!busy}
          multiline
          onChangeText={onChangeNote}
          style={styles.note}
          value={note}
        />
        <ThemedText style={styles.supporting}>{i18n.t('activity.notePrivacy')}</ThemedText>
      </View>
    </View> : null}

    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
    {activityVersion !== undefined ? <ThemedText accessibilityLiveRegion="polite" style={styles.saved}>
      {i18n.t('activity.savedVersion', { version: i18n.number(activityVersion) })}
    </ThemedText> : null}
    {busy ? <StatusBanner text={i18n.t('activity.saving')} /> : null}

  </NativeSheet>;
}

const styles = StyleSheet.create({
  field: {
    gap: mobileTheme.spacing.xs,
  },
  label: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  note: {
    minHeight: 112,
    textAlignVertical: 'top',
  },
  saved: {
    color: mobileTheme.colors.accent,
  },
  section: {
    gap: mobileTheme.spacing.sm,
  },
  supporting: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
});
