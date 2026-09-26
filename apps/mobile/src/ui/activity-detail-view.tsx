import { getLocales } from 'expo-localization';
import { StyleSheet, useWindowDimensions, View } from 'react-native';
import {
  activityBelongsToProfile,
  pagedCollectionPresentation,
  priorNoteForProfile,
  type ActivityDetail,
  type ActivityRevision,
} from '../activity-history';
import { createDeviceTranslator } from '../i18n';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { formatCompactDuration } from './compact-duration';
import { NativePrimaryButton } from './native-primary-button';
import { ActionButton, SectionHeading, StatusBanner, Surface, ThemedText } from './primitives';
import { SettingsActionRow, SettingsSection } from './settings-list';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function ActivityDetailView({
  activity,
  archived,
  busy,
  deletionBusy,
  deletionErrorText,
  deletionRetryable = false,
  errorText,
  hasMoreRevisions,
  manualBusy,
  onEdit,
  onDelete,
  onLoadMoreRevisions,
  onRetry,
  onRetryDeletion,
  onRetryRevisions,
  participantLabel,
  profileID,
  revisionErrorText,
  revisions,
}: {
  activity: ActivityDetail | null;
  archived: boolean;
  busy: boolean;
  deletionBusy: boolean;
  deletionErrorText?: string;
  deletionRetryable?: boolean;
  errorText?: string;
  hasMoreRevisions: boolean;
  manualBusy: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onLoadMoreRevisions: () => void;
  onRetry: () => void;
  onRetryDeletion: () => void;
  onRetryRevisions: () => void;
  participantLabel?: string;
  profileID: string;
  revisionErrorText?: string;
  revisions: readonly ActivityRevision[];
}) {
  const { fontScale, width } = useWindowDimensions();
  const compact = needsCompactVerticalLayout(width, fontScale);

  if (!activity) {
    return <View style={styles.stack}>
      {errorText
        ? <StatusBanner text={errorText} tone="error" />
        : <StatusBanner text={i18n.t('common.loading')} />}
      {errorText ? <ActionButton label={i18n.t('common.retry')} onPress={onRetry} /> : null}
    </View>;
  }

  const ownsActivity = activityBelongsToProfile(activity, profileID);
  const revisionPresentation = pagedCollectionPresentation({
    busy,
    error: Boolean(revisionErrorText),
    hasMore: hasMoreRevisions,
    itemCount: revisions.length,
  });

  return <View style={styles.stack}>
    <View style={styles.summary}>
      <ThemedText style={styles.label}>{i18n.t('timer.elapsedLabel')}</ThemedText>
      <ThemedText style={styles.duration}>{formatCompactDuration(activity.activity.durationSeconds, i18n)}</ThemedText>
    </View>
    <Surface>
      <DetailRow
        compact={compact}
        label={i18n.t('pathDetails.started')}
        value={activityInstant(activity.activity.startedAt, activity.activity.occurrenceTimeZone)}
      />
      <DetailRow
        compact={compact}
        label={i18n.t('pathDetails.ended')}
        value={activityInstant(activity.activity.endedAt, activity.activity.occurrenceTimeZone)}
      />
      <DetailRow
        compact={compact}
        label={i18n.t('activity.occurrenceTimeZone')}
        value={activity.activity.occurrenceTimeZone}
      />
      <DetailRow
        compact={compact}
        label={i18n.t('pathDetails.participantLabel')}
        value={participantLabel ?? activity.activity.participantId}
      />
      {ownsActivity && activity.activity.note
        ? <DetailRow compact={compact} label={i18n.t('activity.note')} value={activity.activity.note} />
        : null}
    </Surface>
    {!archived && ownsActivity
      ? <NativePrimaryButton
        disabled={deletionBusy || manualBusy}
        label={i18n.t('pathDetails.edit')}
        onPress={onEdit}
        systemImage="pencil"
      />
      : null}
    {manualBusy ? <StatusBanner text={i18n.t('common.loading')} /> : null}
    {!archived && ownsActivity
      ? <SettingsSection>
        <SettingsActionRow
          accessibilityLabel={i18n.t('pathDetails.delete')}
          disabled={deletionBusy || manualBusy}
          label={i18n.t('pathDetails.delete')}
          onPress={onDelete}
        />
      </SettingsSection>
      : null}
    {deletionBusy ? <StatusBanner text={i18n.t('pathDetails.deleting')} /> : null}
    {deletionErrorText
      ? <View style={styles.deletionFeedback}>
        <StatusBanner text={deletionErrorText} tone="error" />
        {deletionRetryable
          ? <ActionButton
              disabled={deletionBusy || manualBusy}
              label={i18n.t('common.retry')}
              onPress={onRetryDeletion}
              variant="secondary"
            />
          : null}
      </View>
      : null}

    <View style={styles.section}>
      <SectionHeading>{i18n.t('pathDetails.revisions')}</SectionHeading>
      {revisionPresentation.showEmpty
        ? <ThemedText style={styles.label}>{i18n.t('pathDetails.revisionsEmpty')}</ThemedText>
        : revisions.map((revision) => <Surface key={revision.version}>
          <ThemedText accessibilityRole="header" style={styles.revisionHeading}>{i18n.t('pathDetails.revision', {
            version: i18n.number(revision.version),
          })}</ThemedText>
          <ThemedText style={styles.revisionDate}>{i18n.t('pathDetails.revisionChanged', {
            date: revisionTimestamp(revision.replacedAt),
          })}</ThemedText>
          <DetailRow
            compact={compact}
            label={i18n.t('pathDetails.started')}
            value={activityInstant(revision.startedAt, revision.occurrenceTimeZone)}
          />
          <DetailRow
            compact={compact}
            label={i18n.t('pathDetails.ended')}
            value={activityInstant(revision.endedAt, revision.occurrenceTimeZone)}
          />
          <DetailRow
            compact={compact}
            label={i18n.t('timer.elapsedLabel')}
            value={formatCompactDuration(revision.durationSeconds, i18n)}
          />
          <DetailRow
            compact={compact}
            label={i18n.t('activity.occurrenceTimeZone')}
            value={revision.occurrenceTimeZone}
          />
          {priorNoteForProfile(revision, profileID)
            ? <DetailRow
              compact={compact}
              label={i18n.t('pathDetails.priorNote')}
              value={priorNoteForProfile(revision, profileID) ?? undefined}
            />
            : null}
        </Surface>)}
      {revisionPresentation.showError && revisionErrorText
        ? <StatusBanner text={revisionErrorText} tone="error" />
        : null}
      {revisionPresentation.showRetry
        ? <ActionButton disabled={busy} label={i18n.t('common.retry')} onPress={onRetryRevisions} variant="secondary" />
        : null}
      {revisionPresentation.showLoadMore
        ? <ActionButton disabled={busy} label={i18n.t('common.loadMore')} onPress={onLoadMoreRevisions} variant="secondary" />
        : null}
      {revisionPresentation.showLoading ? <StatusBanner text={i18n.t('common.loading')} /> : null}
    </View>
    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
  </View>;
}

function DetailRow({
  compact,
  label,
  value,
}: {
  compact: boolean;
  label: string;
  value?: string;
}) {
  return <View style={[styles.detailRow, compact ? styles.detailRowCompact : null]}>
    <ThemedText style={value ? styles.label : styles.valueStandalone}>{label}</ThemedText>
    {value ? <ThemedText selectable style={[styles.value, compact ? styles.valueCompact : null]}>{value}</ThemedText> : null}
  </View>;
}

function activityInstant(instant: string, timeZone: string) {
  return i18n.date(new Date(instant), {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone,
  });
}

function revisionTimestamp(instant: string) {
  return i18n.date(new Date(instant), {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

const styles = StyleSheet.create({
  summary: { gap: mobileTheme.spacing.xxs, paddingVertical: mobileTheme.spacing.sm },
  duration: { ...mobileTheme.typography.title, fontVariant: ['tabular-nums'] },
  detailRow: {
    alignItems: 'flex-start',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'space-between',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingVertical: mobileTheme.spacing.xs,
  },
  detailRowCompact: {
    flexDirection: 'column',
    gap: mobileTheme.spacing.xxs,
  },
  deletionFeedback: {
    gap: mobileTheme.spacing.sm,
  },
  label: {
    color: mobileTheme.colors.textMuted,
    flexShrink: 0,
    ...mobileTheme.typography.caption,
  },
  revisionHeading: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.subheading,
  },
  revisionDate: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  section: {
    gap: mobileTheme.spacing.sm,
  },
  stack: {
    gap: mobileTheme.spacing.lg,
  },
  value: {
    color: mobileTheme.colors.text,
    flex: 1,
    textAlign: 'right',
    ...mobileTheme.typography.body,
  },
  valueCompact: {
    textAlign: 'left',
  },
  valueStandalone: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
  },
});
