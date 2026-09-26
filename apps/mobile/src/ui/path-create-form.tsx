import { useEffect, useState, type ReactNode } from 'react';
import type { PathRecurrence } from '@hourpaths/api-client';
import type { PathVisibility } from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { AccessibilityInfo, InputAccessoryView, Keyboard, Pressable, StyleSheet, Switch, View } from 'react-native';
import type { PathGoalForm } from '../path-goals';
import { pathRecurrences } from '../path-goals';
import type { DurationUnit } from './duration-input';
import { HumanDurationEditor } from './human-duration-editor';
import { NativeChoicePicker } from './native-choice-picker';
import {
  pathAlignmentIsCustomized,
  resetPathAlignment,
} from './path-create-presentation';
import {
  NativeSheet,
  StatusBanner,
  Surface,
  ThemedText as Text,
  ThemedTextInput as TextInput,
} from './primitives';
import { mobileTheme } from './tokens';

const recurrenceKeys: Record<PathRecurrence, MessageKey> = {
  hourly: 'pathCreate.recurrence.hourly',
  daily: 'pathCreate.recurrence.daily',
  weekly: 'pathCreate.recurrence.weekly',
  monthly: 'pathCreate.recurrence.monthly',
  yearly: 'pathCreate.recurrence.yearly',
};

const durationUnitKeys: Record<DurationUnit, MessageKey> = {
  seconds: 'pathCreate.duration.seconds',
  minutes: 'pathCreate.duration.minutes',
  hours: 'pathCreate.duration.hours',
};

const weekdayKeys = [
  'pathCreate.weekday.1',
  'pathCreate.weekday.2',
  'pathCreate.weekday.3',
  'pathCreate.weekday.4',
  'pathCreate.weekday.5',
  'pathCreate.weekday.6',
  'pathCreate.weekday.7',
] as const satisfies readonly MessageKey[];

const keyboardAccessoryID = 'path-create-keyboard-actions';

type PathCreateFormProps = {
  busy: boolean;
  errorText?: string;
  form: PathGoalForm;
  i18n: Translator;
  name: string;
  onCancel: () => void;
  onCreate: () => void;
  onNameChange: (name: string) => void;
  onUpdate: (update: Partial<PathGoalForm>) => void;
  onVisibilityChange: (visibility: PathVisibility) => void;
  visibility: PathVisibility;
  visibilityOptions: readonly PathVisibility[];
  visible: boolean;
};

export function PathCreateForm({
  busy,
  errorText,
  form,
  i18n,
  name,
  onCancel,
  onCreate,
  onNameChange,
  onUpdate,
  onVisibilityChange,
  visibility,
  visibilityOptions,
  visible,
}: PathCreateFormProps) {
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => subscription.remove();
  }, []);
  const canCreate = name.trim().length > 0;
  const requestClose = () => {
    if (!busy) {
      onCancel();
    }
  };

  return <NativeSheet
    animationType={reduceMotion === false ? 'slide' : 'none'}
    compact
    dismissible={!busy}
    leadingAction={{ disabled: busy, label: i18n.t('common.cancel'), onPress: onCancel }}
    onRequestClose={requestClose}
    title={i18n.t('pathCreate.heading')}
    trailingAction={{
      disabled: busy || !canCreate,
      label: i18n.t(errorText ? 'pathCreate.retry' : 'pathCreate.submit'),
      onPress: onCreate,
    }}
    visible={visible}
  >
    <View style={styles.field}>
      <Text style={styles.label}>{i18n.t('pathCreate.nameLabel')}</Text>
      <TextInput
        accessibilityLabel={i18n.t('pathCreate.nameLabel')}
        autoCapitalize="words"
        autoCorrect
        autoFocus
        editable={!busy}
        maxLength={100}
        onChangeText={onNameChange}
        onSubmitEditing={Keyboard.dismiss}
        returnKeyType="done"
        value={name}
      />
    </View>

    <NativeChoicePicker
      accessibilityLabel={i18n.t('pathVisibility.choiceLabel')}
      choices={visibilityOptions.map((option) => ({
        label: i18n.t(`pathVisibility.option.${option}`),
        value: option,
      }))}
      disabled={busy}
      label={i18n.t('pathVisibility.choiceLabel')}
      onChange={onVisibilityChange}
      value={visibility}
    />

    <PathGoalFields
      busy={busy}
      compact
      form={form}
      i18n={i18n}
      inputAccessoryViewID={keyboardAccessoryID}
      onUpdate={onUpdate}
    />

    {busy ? <StatusBanner text={i18n.t('pathCreate.submitting')} tone="loading" /> : null}
    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
    <InputAccessoryView nativeID={keyboardAccessoryID}>
      <View style={styles.keyboardToolbar}>
        <Pressable
          accessibilityRole="button"
          onPress={Keyboard.dismiss}
          style={({ pressed }) => [styles.keyboardDone, pressed ? styles.pressed : null]}
        >
          <Text style={styles.keyboardDoneLabel}>{i18n.t('common.done')}</Text>
        </Pressable>
      </View>
    </InputAccessoryView>
  </NativeSheet>;
}

export function PathGoalFields({
  busy,
  compact = false,
  form,
  i18n,
  inputAccessoryViewID,
  onUpdate,
}: {
  busy: boolean;
  compact?: boolean;
  form: PathGoalForm;
  i18n: Translator;
  inputAccessoryViewID?: string;
  onUpdate: (update: Partial<PathGoalForm>) => void;
}) {
  const [customizeAlignment, setCustomizeAlignment] = useState(
    () => pathAlignmentIsCustomized(form),
  );
  const recurrenceChoices = pathRecurrences.map((recurrence) => ({
    label: i18n.t(recurrenceKeys[recurrence]),
    value: recurrence,
  }));

  function changeRecurrence(recurrence: PathRecurrence) {
    setCustomizeAlignment(false);
    onUpdate({
      recurrence,
      ...resetPathAlignment({ ...form, recurrence }),
    });
  }

  function changeAlignmentCustomization(customized: boolean) {
    setCustomizeAlignment(customized);
    if (!customized) {
      onUpdate(resetPathAlignment(form));
    } else if (form.recurrence === 'weekly' && form.weeklyISOWeekday === '') {
      onUpdate({ weeklyISOWeekday: '1' });
    }
  }

  return <>
    <GoalFieldSection compact={compact}>
      <GoalSwitchRow
        busy={busy}
        label={i18n.t('pathCreate.intervalEnabled')}
        onChange={(intervalEnabled) => onUpdate({ intervalEnabled })}
        value={form.intervalEnabled}
      />
      {form.intervalEnabled ? <View style={[styles.revealed, compact ? styles.compactRevealed : null]}>
        <HumanDurationEditor
          busy={busy}
          compact={compact}
          inputAccessoryViewID={inputAccessoryViewID}
          label={i18n.t('pathCreate.durationValue')}
          onChange={(intervalSeconds) => onUpdate({ intervalSeconds })}
          onSubmitEditing={Keyboard.dismiss}
          seconds={form.intervalSeconds}
          unitLabels={{
            hours: i18n.t(durationUnitKeys.hours),
            minutes: i18n.t(durationUnitKeys.minutes),
            seconds: i18n.t(durationUnitKeys.seconds),
          }}
        />
        <NativeChoicePicker
          accessibilityLabel={i18n.t('pathCreate.recurrence')}
          choices={recurrenceChoices}
          disabled={busy}
          label={i18n.t('pathCreate.recurrence')}
          onChange={changeRecurrence}
          value={form.recurrence}
        />
        <GoalSwitchRow
          busy={busy}
          label={i18n.t('pathCreate.alignment.customize')}
          onChange={changeAlignmentCustomization}
          value={customizeAlignment}
        />
        {!customizeAlignment
          ? <Text style={styles.secondary}>{i18n.t('pathCreate.alignment.default')}</Text>
          : <AlignmentFields
            busy={busy}
            compact={compact}
            form={form}
            i18n={i18n}
            inputAccessoryViewID={inputAccessoryViewID}
            onUpdate={onUpdate}
          />}
      </View> : null}
    </GoalFieldSection>

    <GoalFieldSection compact={compact}>
      <GoalSwitchRow
        busy={busy}
        label={i18n.t('pathCreate.overallEnabled')}
        onChange={(overallEnabled) => onUpdate({ overallEnabled })}
        value={form.overallEnabled}
      />
      {form.overallEnabled ? <View style={[styles.revealed, compact ? styles.compactRevealed : null]}>
        <HumanDurationEditor
          busy={busy}
          compact={compact}
          inputAccessoryViewID={inputAccessoryViewID}
          label={i18n.t('pathCreate.durationValue')}
          onChange={(overallSeconds) => onUpdate({ overallSeconds })}
          onSubmitEditing={Keyboard.dismiss}
          seconds={form.overallSeconds}
          unitLabels={{
            hours: i18n.t(durationUnitKeys.hours),
            minutes: i18n.t(durationUnitKeys.minutes),
            seconds: i18n.t(durationUnitKeys.seconds),
          }}
        />
      </View> : null}
    </GoalFieldSection>
  </>;
}

function GoalFieldSection({
  children,
  compact,
}: {
  children: ReactNode;
  compact: boolean;
}) {
  return compact
    ? <View style={styles.compactSection}>{children}</View>
    : <Surface>{children}</Surface>;
}

function GoalSwitchRow({
  busy,
  label,
  onChange,
  value,
}: {
  busy: boolean;
  label: string;
  onChange: (value: boolean) => void;
  value: boolean;
}) {
  return <Pressable
    accessibilityLabel={label}
    accessibilityRole="switch"
    accessibilityState={{ checked: value, disabled: busy }}
    disabled={busy}
    onPress={() => onChange(!value)}
    style={({ pressed }) => [styles.switchRow, pressed ? styles.pressed : null]}
  >
    <Text style={styles.switchLabel}>{label}</Text>
    <Switch
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      pointerEvents="none"
      trackColor={{ false: mobileTheme.colors.border, true: mobileTheme.colors.accentPressed }}
      thumbColor={value ? mobileTheme.colors.accent : mobileTheme.colors.textMuted}
      value={value}
    />
  </Pressable>;
}

function AlignmentFields({
  busy,
  compact,
  form,
  i18n,
  inputAccessoryViewID,
  onUpdate,
}: {
  busy: boolean;
  compact: boolean;
  form: PathGoalForm;
  i18n: Translator;
  inputAccessoryViewID?: string;
  onUpdate: (update: Partial<PathGoalForm>) => void;
}) {
  if (form.recurrence === 'weekly') {
    const choices = weekdayKeys.map((key, index) => ({
      label: i18n.t(key),
      value: String(index + 1),
    }));
    return <NativeChoicePicker
      accessibilityLabel={i18n.t('pathCreate.alignment.weekday')}
      choices={choices}
      disabled={busy}
      label={i18n.t('pathCreate.alignment.weekday')}
      onChange={(weeklyISOWeekday) => onUpdate({ weeklyISOWeekday })}
      value={form.weeklyISOWeekday || '1'}
    />;
  }

  return <View style={styles.alignmentFields}>
    {form.recurrence === 'hourly' ? <NumberField
      busy={busy}
      compact={compact}
      inputAccessoryViewID={inputAccessoryViewID}
      label={i18n.t('pathCreate.alignment.minute')}
      onChange={(hourlyMinute) => onUpdate({ hourlyMinute })}
      value={form.hourlyMinute}
    /> : null}
    {form.recurrence === 'daily' ? <NumberField
      busy={busy}
      compact={compact}
      inputAccessoryViewID={inputAccessoryViewID}
      label={i18n.t('pathCreate.alignment.hour')}
      onChange={(dailyHour) => onUpdate({ dailyHour })}
      value={form.dailyHour}
    /> : null}
    {form.recurrence === 'monthly' ? <NumberField
      busy={busy}
      compact={compact}
      inputAccessoryViewID={inputAccessoryViewID}
      label={i18n.t('pathCreate.alignment.day')}
      onChange={(monthlyDay) => onUpdate({ monthlyDay })}
      value={form.monthlyDay}
    /> : null}
    {form.recurrence === 'yearly' ? <>
      <NumberField
        busy={busy}
        compact={compact}
        inputAccessoryViewID={inputAccessoryViewID}
        label={i18n.t('pathCreate.alignment.month')}
        onChange={(yearlyMonth) => onUpdate({ yearlyMonth })}
        value={form.yearlyMonth}
      />
      <NumberField
        busy={busy}
        compact={compact}
        inputAccessoryViewID={inputAccessoryViewID}
        label={i18n.t('pathCreate.alignment.day')}
        onChange={(yearlyDay) => onUpdate({ yearlyDay })}
        value={form.yearlyDay}
      />
    </> : null}
  </View>;
}

function NumberField({
  busy,
  compact,
  inputAccessoryViewID,
  label,
  onChange,
  value,
}: {
  busy: boolean;
  compact: boolean;
  inputAccessoryViewID?: string;
  label: string;
  onChange: (value: string) => void;
  value: string;
}) {
  return <View style={styles.field}>
    <Text style={styles.label}>{label}</Text>
    <TextInput
      accessibilityLabel={label}
      editable={!busy}
      inputAccessoryViewID={inputAccessoryViewID}
      keyboardType="number-pad"
      onChangeText={onChange}
      onSubmitEditing={Keyboard.dismiss}
      returnKeyType="done"
      style={compact ? styles.compactInput : undefined}
      value={value}
    />
  </View>;
}

const styles = StyleSheet.create({
  alignmentFields: {
    gap: mobileTheme.spacing.sm,
  },
  compactInput: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderWidth: 0,
  },
  compactRevealed: {
    borderTopColor: mobileTheme.colors.surfaceRaised,
    gap: mobileTheme.spacing.sm,
    marginTop: mobileTheme.spacing.xs,
    paddingTop: mobileTheme.spacing.sm,
  },
  compactSection: {
    backgroundColor: mobileTheme.colors.surface,
    borderRadius: mobileTheme.radii.md,
    overflow: 'hidden',
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.xs,
  },
  field: {
    gap: mobileTheme.spacing.xs,
  },
  keyboardDone: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  keyboardDoneLabel: {
    color: mobileTheme.colors.accent,
    ...mobileTheme.typography.body,
  },
  keyboardToolbar: {
    alignItems: 'flex-end',
    backgroundColor: mobileTheme.colors.surface,
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: mobileTheme.sizes.border,
  },
  label: {
    ...mobileTheme.typography.caption,
    color: mobileTheme.colors.textMuted,
  },
  pressed: {
    opacity: 0.72,
  },
  revealed: {
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: mobileTheme.sizes.border,
    gap: mobileTheme.spacing.md,
    marginTop: mobileTheme.spacing.sm,
    paddingTop: mobileTheme.spacing.md,
  },
  secondary: {
    color: mobileTheme.colors.textMuted,
  },
  switchLabel: {
    flex: 1,
    ...mobileTheme.typography.body,
  },
  switchRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.md,
    justifyContent: 'space-between',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
});
