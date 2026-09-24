import { useEffect, useState } from 'react';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import type { GoalConfiguration, GoalConfigurationComparison, PathVisibility, PathVisibilityChangeReview } from '@hourpaths/client-core';
import { StyleSheet, View } from 'react-native';
import {
  derivePathRenamePresentation,
  pathManagementIsBusy,
  reconcilePathManagementScreen,
  type PathManagementScreen,
} from '../path-management-presentation';
import type { PathGoalForm } from '../path-goals';
import { formatCompactDuration } from './compact-duration';
import { PathGoalFields } from './path-create-form';
import {
  ActionButton,
  NativeSheet,
  ThemedText as Text,
  ThemedTextInput,
} from './primitives';
import {
  SettingsNavigationRow,
  SettingsActionRow,
  SettingsSection,
  SettingsSeparator,
} from './settings-list';
import { mobileTheme } from './tokens';

const recurrenceSummaryKeys: Record<
  NonNullable<GoalConfiguration['intervalGoal']>['recurrence'],
  MessageKey
> = {
  hourly: 'path.goal.recurrence.hourly',
  daily: 'path.goal.recurrence.daily',
  weekly: 'path.goal.recurrence.weekly',
  monthly: 'path.goal.recurrence.monthly',
  yearly: 'path.goal.recurrence.yearly',
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

export type PathGoalManagementFormProps = {
  archived?: boolean;
  busy: boolean;
  canDelete: boolean;
  canManageGoals: boolean;
  canManageVisibility: boolean;
  canRename: boolean;
  deleteBusy: boolean;
  deleteErrorText?: string;
  errorText?: string;
  form: PathGoalForm;
  goalCanReview: boolean;
  i18n: Translator;
  onCancel: (dirty: boolean) => void;
  onCancelReview: () => void;
  onConfirm: () => void;
  onDeleteCancel: () => void;
  onDeleteConfirm: () => void;
  onDeleteReview: () => void;
  onReview: () => void;
  onOpenArchive?: (dirty: boolean) => void;
  onOpenOwnershipTransfer?: (dirty: boolean) => void;
  onVisibilityCancelReview: () => void;
  onVisibilityChange: (visibility: PathVisibility) => void;
  onVisibilityConfirm: () => void;
  onVisibilityReview: () => void;
  onRenameChange: (name: string) => void;
  onRenameSave: () => void;
  onUpdate: (update: Partial<PathGoalForm>) => void;
  renameBusy: boolean;
  renameCurrentName: string;
  renameErrorText?: string;
  renameName: string;
  renameSavedName?: string;
  pathName: string;
  review?: Pick<GoalConfigurationComparison, 'changed' | 'current' | 'proposed'>;
  saved: boolean;
  visible: boolean;
  visibilityBusy: boolean;
  visibilityCurrent: PathVisibility;
  visibilityDraft: PathVisibility;
  visibilityErrorText?: string;
  visibilityOptions: readonly PathVisibility[];
  visibilityReview?: Extract<PathVisibilityChangeReview, { kind: 'ready' }>;
  visibilitySaved: boolean;
};

export function PathGoalManagementForm({
  archived = false,
  busy,
  canDelete,
  canManageGoals,
  canManageVisibility,
  canRename,
  deleteBusy,
  deleteErrorText,
  errorText,
  form,
  goalCanReview,
  i18n,
  onCancel,
  onCancelReview,
  onConfirm,
  onDeleteCancel,
  onDeleteConfirm,
  onDeleteReview,
  onReview,
  onOpenArchive,
  onOpenOwnershipTransfer,
  onVisibilityCancelReview,
  onVisibilityChange,
  onVisibilityConfirm,
  onVisibilityReview,
  onRenameChange,
  onRenameSave,
  onUpdate,
  renameBusy,
  renameCurrentName,
  renameErrorText,
  renameName,
  renameSavedName,
  pathName,
  review,
  saved,
  visible,
  visibilityBusy,
  visibilityCurrent,
  visibilityDraft,
  visibilityErrorText,
  visibilityOptions,
  visibilityReview,
  visibilitySaved,
}: PathGoalManagementFormProps) {
  const [screen, setScreen] = useState<PathManagementScreen>('overview');
  const managementBusy = pathManagementIsBusy({ deleteBusy, goalBusy: busy, renameBusy });
  const renamePresentation = derivePathRenamePresentation({
    busy: renameBusy,
    currentName: renameCurrentName,
    draftName: renameName,
    errorText: renameErrorText,
    savedName: renameSavedName,
  });
  const dirty = goalCanReview || renamePresentation.canSave || Boolean(review);

  useEffect(() => {
    setScreen((current) => reconcilePathManagementScreen({
      goalSaved: saved,
      hasReview: Boolean(review),
      screen: current,
    }));
  }, [review, saved]);

  function leaveFocusedScreen() {
    if (managementBusy) return;
    if (screen === 'review') onCancelReview();
    if (screen === 'delete') onDeleteCancel();
    setScreen(screen === 'review' ? 'goals' : 'overview');
  }

  const title = screen === 'overview'
    ? i18n.t('pathManage.mobileHeading')
    : screen === 'name'
      ? i18n.t('pathRename.heading')
      : screen === 'goals'
        ? i18n.t('pathManage.goalsHeading')
        : screen === 'delete'
          ? i18n.t('pathDelete.heading', { pathName })
          : i18n.t('pathManage.reviewHeading');
  const leadingAction = screen === 'overview'
    ? undefined
    : {
        disabled: managementBusy,
        label: i18n.t('common.back'),
        onPress: leaveFocusedScreen,
      };
  const trailingAction = screen === 'overview'
    ? { label: i18n.t('common.done'), onPress: () => onCancel(dirty) }
    : screen === 'name'
      ? {
          disabled: managementBusy || !renamePresentation.canSave,
          label: i18n.t(renameBusy ? 'pathRename.saving' : 'common.save'),
          onPress: onRenameSave,
        }
      : screen === 'goals'
        ? {
            disabled: managementBusy || !goalCanReview,
            label: i18n.t('pathManage.reviewAction'),
            onPress: onReview,
          }
        : screen === 'review' ? {
            disabled: busy || !review?.changed,
            label: i18n.t(busy ? 'pathManage.applyingAction' : 'pathManage.confirmAction'),
            onPress: onConfirm,
          } : undefined;

  return <NativeSheet
    compact
    dismissible={!managementBusy && !dirty}
    leadingAction={leadingAction}
    onRequestClose={() => {
      if (!managementBusy) onCancel(dirty);
    }}
    title={title}
    trailingAction={trailingAction}
    visible={visible}
  >
    {screen === 'overview' && (canRename || canManageGoals || onOpenArchive || onOpenOwnershipTransfer) ? <SettingsSection footer={i18n.t('pathManage.overviewExplanation')}>
      {canRename ? <>
        <SettingsNavigationRow
          accessibilityLabel={i18n.t('pathRename.action')}
          label={i18n.t('pathRename.nameLabel')}
          onPress={() => setScreen('name')}
          value={renameCurrentName}
        />
        {(canManageGoals || onOpenArchive || onOpenOwnershipTransfer) ? <SettingsSeparator /> : null}
      </> : null}
      {canManageGoals ? <SettingsNavigationRow
        accessibilityLabel={i18n.t('pathManage.goalsHeading')}
        context={i18n.t('pathManage.explanation')}
        label={i18n.t('pathManage.goalsHeading')}
        onPress={() => setScreen('goals')}
      /> : null}
      {canManageGoals && (onOpenArchive || onOpenOwnershipTransfer) ? <SettingsSeparator /> : null}
      {onOpenArchive ? <SettingsActionRow accessibilityLabel={i18n.t(archived ? 'pathArchive.unarchiveAction' : 'pathArchive.action')} label={i18n.t(archived ? 'pathArchive.unarchiveAction' : 'pathArchive.action')} onPress={() => onOpenArchive(dirty)} /> : null}
      {onOpenArchive && onOpenOwnershipTransfer ? <SettingsSeparator /> : null}
      {onOpenOwnershipTransfer ? <SettingsNavigationRow accessibilityLabel={i18n.t('pathOwnership.heading')} label={i18n.t('pathOwnership.heading')} onPress={() => onOpenOwnershipTransfer(dirty)} /> : null}
    </SettingsSection> : null}

    {screen === 'overview' && canDelete ? <SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t('pathDelete.action')}
        label={i18n.t('pathDelete.action')}
        onPress={() => {
          onDeleteReview();
          setScreen('delete');
        }}
      />
    </SettingsSection> : null}

    {screen === 'name' && canRename ? <>
      <View style={styles.compactGroup}>
        <ThemedTextInput
          accessibilityLabel={i18n.t('pathRename.nameLabel')}
          autoCapitalize="sentences"
          autoCorrect
          autoFocus
          editable={!renameBusy}
          enterKeyHint="done"
          maxLength={100}
          onChangeText={onRenameChange}
          onSubmitEditing={() => {
            if (renamePresentation.canSave) onRenameSave();
          }}
          returnKeyType="done"
          style={styles.nameInput}
          value={renameName}
        />
      </View>
      <Text style={styles.footer}>{i18n.t('pathRename.explanation')}</Text>
      {renamePresentation.kind === 'invalid'
        ? <InlineStatus text={i18n.t('pathRename.invalid')} tone="error" />
        : null}
      {renamePresentation.kind === 'saved'
        ? <InlineStatus text={i18n.t('pathRename.saved', { name: renamePresentation.name })} tone="success" />
        : null}
      {renamePresentation.kind === 'error'
        ? <InlineStatus text={renamePresentation.text} tone="error" />
        : null}
    </> : null}

    {screen === 'goals' ? <>
      <PathGoalFields
        busy={managementBusy}
        compact
        form={form}
        i18n={i18n}
        onUpdate={onUpdate}
      />
      <Text style={styles.footer}>{i18n.t('pathManage.explanation')}</Text>
      {saved ? <InlineStatus text={i18n.t('pathManage.saved')} tone="success" /> : null}
      {errorText ? <InlineStatus text={errorText} tone="error" /> : null}
    </> : null}

    {screen === 'review' && review ? <GoalChangeReview
      errorText={errorText}
      i18n={i18n}
      review={review}
    /> : null}

    {screen === 'delete' && canDelete ? <>
      <View style={styles.compactGroup}>
        <Text accessibilityRole="alert" style={styles.warning}>{i18n.t('pathDelete.warning')}</Text>
        <Text style={styles.warning}>{i18n.t('pathDelete.timerWarning')}</Text>
        <Text style={styles.secondary}>{i18n.t('pathDelete.archiveAlternative')}</Text>
      </View>
      {deleteErrorText ? <InlineStatus text={deleteErrorText} tone="error" /> : null}
      <ActionButton
        disabled={deleteBusy}
        label={i18n.t(deleteBusy ? 'pathDelete.deleting' : 'pathDelete.confirm')}
        onPress={onDeleteConfirm}
        variant="danger"
      />
    </> : null}
  </NativeSheet>;
}

function GoalChangeReview({
  errorText,
  i18n,
  review,
}: {
  errorText?: string;
  i18n: Translator;
  review: Pick<GoalConfigurationComparison, 'changed' | 'current' | 'proposed'>;
}) {
  return <>
    <View style={styles.compactGroup}>
      <Text accessibilityRole="header" style={styles.summaryHeading}>
        {i18n.t('pathManage.current')}
      </Text>
      <GoalConfigurationSummary configuration={review.current} i18n={i18n} />
    </View>
    <View style={styles.compactGroup}>
      <View style={styles.proposedHeading}>
        <Text accessibilityRole="header" style={styles.summaryHeading}>
          {i18n.t('pathManage.proposed')}
        </Text>
        {review.changed ? <Text style={styles.changed}>{i18n.t('pathManage.changed')}</Text> : null}
      </View>
      <GoalConfigurationSummary configuration={review.proposed} i18n={i18n} />
    </View>

    <Text accessibilityRole="alert" style={styles.warning}>
      {i18n.t('pathManage.goalWarning')}
    </Text>
    {!review.changed ? <InlineStatus text={i18n.t('pathManage.noChanges')} /> : null}
    {errorText ? <InlineStatus text={errorText} tone="error" /> : null}
  </>;
}

function InlineStatus({
  text,
  tone = 'neutral',
}: {
  text: string;
  tone?: 'neutral' | 'success' | 'error';
}) {
  return <Text
    accessibilityLiveRegion={tone === 'neutral' ? 'none' : tone === 'error' ? 'assertive' : 'polite'}
    accessibilityRole={tone === 'error' ? 'alert' : undefined}
    style={[
      styles.inlineStatus,
      tone === 'success' ? styles.success : null,
      tone === 'error' ? styles.error : null,
    ]}
  >
    {text}
  </Text>;
}

function GoalConfigurationSummary({
  configuration,
  i18n,
}: {
  configuration: GoalConfiguration;
  i18n: Translator;
}) {
  const interval = configuration.intervalGoal;
  return <View style={styles.summary}>
    <View style={styles.summaryGroup}>
      <Text style={styles.label}>{i18n.t('pathCreate.intervalHeading')}</Text>
      {interval ? <>
        <Text style={styles.value}>{i18n.t('pathManage.intervalSummary', {
          duration: formatCompactDuration(interval.targetSeconds, i18n),
          recurrence: i18n.t(recurrenceSummaryKeys[interval.recurrence]),
        })}</Text>
        <GoalAlignmentSummary configuration={configuration} i18n={i18n} />
      </> : <Text style={styles.secondary}>{i18n.t('pathManage.noInterval')}</Text>}
    </View>
    <View style={styles.summaryGroup}>
      <Text style={styles.label}>{i18n.t('pathCreate.overallHeading')}</Text>
      <Text style={configuration.overallTarget ? styles.value : styles.secondary}>
        {configuration.overallTarget
          ? i18n.t('pathManage.overallSummary', {
            duration: formatCompactDuration(configuration.overallTarget.targetSeconds, i18n),
          })
          : i18n.t('pathManage.noOverall')}
      </Text>
    </View>
  </View>;
}

function GoalAlignmentSummary({
  configuration,
  i18n,
}: {
  configuration: GoalConfiguration;
  i18n: Translator;
}) {
  const interval = configuration.intervalGoal;
  if (!interval) return null;
  const alignment = interval.alignment;
  if (interval.recurrence === 'hourly') {
    return <Text style={styles.secondary}>{i18n.t('pathManage.alignmentMinute', {
      value: i18n.number(alignment?.minute ?? 0),
    })}</Text>;
  }
  if (interval.recurrence === 'daily') {
    return <Text style={styles.secondary}>{i18n.t('pathManage.alignmentHour', {
      value: i18n.number(alignment?.hour ?? 0),
    })}</Text>;
  }
  if (interval.recurrence === 'weekly') {
    return <Text style={styles.secondary}>{alignment?.isoWeekday === undefined
      ? i18n.t('pathManage.alignmentProfileWeekStart')
      : i18n.t('pathManage.alignmentWeekday', {
        weekday: i18n.t(weekdayKeys[alignment.isoWeekday - 1]!),
      })}</Text>;
  }
  if (interval.recurrence === 'monthly') {
    return <Text style={styles.secondary}>{i18n.t('pathManage.alignmentDay', {
      value: i18n.number(alignment?.day ?? 1),
    })}</Text>;
  }
  return <Text style={styles.secondary}>{i18n.t('pathManage.alignmentYearly', {
    day: i18n.number(alignment?.day ?? 1),
    month: i18n.number(alignment?.month ?? 1),
  })}</Text>;
}

const styles = StyleSheet.create({
  changed: {
    ...mobileTheme.typography.caption,
    color: mobileTheme.colors.accent,
  },
  compactGroup: {
    backgroundColor: mobileTheme.colors.surface,
    borderRadius: mobileTheme.radii.md,
    gap: mobileTheme.spacing.sm,
    overflow: 'hidden',
    padding: mobileTheme.spacing.md,
  },
  error: {
    color: mobileTheme.colors.error,
  },
  footer: {
    ...mobileTheme.typography.caption,
    color: mobileTheme.colors.textMuted,
    marginTop: -mobileTheme.spacing.md,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  inlineStatus: {
    ...mobileTheme.typography.caption,
    color: mobileTheme.colors.textMuted,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  label: {
    ...mobileTheme.typography.caption,
    color: mobileTheme.colors.textMuted,
  },
  nameInput: {
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderWidth: 0,
    minHeight: 52,
  },
  proposedHeading: {
    alignItems: 'baseline',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'space-between',
  },
  secondary: {
    color: mobileTheme.colors.textMuted,
  },
  summary: {
    gap: mobileTheme.spacing.md,
  },
  summaryGroup: {
    gap: mobileTheme.spacing.xs,
  },
  summaryHeading: {
    ...mobileTheme.typography.subheading,
    marginBottom: mobileTheme.spacing.xs,
  },
  success: {
    color: mobileTheme.colors.accent,
  },
  value: {
    ...mobileTheme.typography.body,
  },
  visibilityPicker: {
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  warning: {
    ...mobileTheme.typography.body,
    color: mobileTheme.colors.text,
  },
});
