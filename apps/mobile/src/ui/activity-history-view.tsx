import { getLocales } from 'expo-localization';
import { Pressable, StyleSheet, useWindowDimensions, View } from 'react-native';
import {
  activityWasEdited,
  groupActivitiesByOccurrenceDay,
  pagedCollectionPresentation,
  retainedCalendarDayDate,
  retainedCalendarDayTimeZone,
  type ActivityDetail,
} from '../activity-history';
import { createDeviceTranslator } from '../i18n';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { formatCompactDuration } from './compact-duration';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ActionButton, StatusBanner, ThemedText } from './primitives';
import { SettingsIcon } from './settings-icon';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function ActivityHistoryView({
  activities,
  busy,
  errorText,
  hasMore,
  hideParticipant = false,
  onLoadMore,
  onOpenActivity,
  onRetry,
}: {
  activities: readonly ActivityDetail[];
  busy: boolean;
  errorText?: string;
  hasMore: boolean;
  hideParticipant?: boolean;
  onLoadMore: () => void;
  onOpenActivity: (activityID: string) => void;
  onRetry: () => void;
}) {
  const { fontScale, width } = useWindowDimensions();
  const compact = needsCompactVerticalLayout(width, fontScale);
  const presentation = pagedCollectionPresentation({
    busy,
    error: Boolean(errorText),
    hasMore,
    itemCount: activities.length,
  });

  if (presentation.showLoading && activities.length === 0) {
    return <StatusBanner text={i18n.t('common.loading')} />;
  }

  if (presentation.showError && errorText && activities.length === 0) {
    return <View style={styles.stack}>
      <StatusBanner text={errorText} tone="error" />
      <ActionButton label={i18n.t('common.retry')} onPress={onRetry} />
    </View>;
  }

  if (presentation.showEmpty) {
    return <NativeContentUnavailable
      description={i18n.t('pathDetails.historyEmptyExplanation')}
      systemImage="clock.arrow.circlepath"
      title={i18n.t('pathDetails.historyEmpty')}
    />;
  }

  return <View style={styles.stack}>
    {groupActivitiesByOccurrenceDay(activities).map((day) => <View key={day.localDate} style={styles.day}>
      <ThemedText accessibilityRole="header" style={styles.dayHeading}>
        {i18n.date(retainedCalendarDayDate(day.localDate), {
          dateStyle: 'full',
          timeZone: retainedCalendarDayTimeZone,
        })}
      </ThemedText>
      <View style={styles.rows}>
        {day.items.map((detail) => <ActivityHistoryRow
          compact={compact}
          detail={detail}
          hideParticipant={hideParticipant}
          key={detail.activity.id}
          onPress={() => onOpenActivity(detail.activity.id)}
        />)}
      </View>
    </View>)}
    {presentation.showError && errorText ? <StatusBanner text={errorText} tone="error" /> : null}
    {presentation.showRetry ? <ActionButton label={i18n.t('common.retry')} onPress={onRetry} variant="secondary" /> : null}
    {presentation.showLoadMore
      ? <ActionButton disabled={busy} label={i18n.t('common.loadMore')} onPress={onLoadMore} variant="secondary" />
      : null}
    {presentation.showLoading ? <StatusBanner text={i18n.t('common.loading')} /> : null}
  </View>;
}

function ActivityHistoryRow({
  compact,
  detail,
  hideParticipant,
  onPress,
}: {
  compact: boolean;
  detail: ActivityDetail;
  hideParticipant: boolean;
  onPress: () => void;
}) {
  const time = i18n.time(new Date(detail.activity.startedAt), {
    timeStyle: 'short',
    timeZone: detail.activity.occurrenceTimeZone,
  });
  const edited = activityWasEdited(detail);
  const duration = formatCompactDuration(detail.activity.durationSeconds, i18n);

  return <Pressable
    accessibilityLabel={i18n.t(hideParticipant
      ? edited ? 'pathDetails.memberHistoryRowEdited' : 'pathDetails.memberHistoryRow'
      : edited ? 'pathDetails.historyRowEdited' : 'pathDetails.historyRow', {
      duration,
      participant: detail.activity.participantId,
      time,
    })}
    accessibilityRole="button"
    onPress={onPress}
    style={({ pressed }) => [
      styles.row,
      compact ? styles.rowCompact : null,
      pressed ? styles.rowPressed : null,
    ]}
  >
    <View style={styles.rowCopy}>
      <View style={styles.rowHeading}>
        <ThemedText style={styles.time}>{time}</ThemedText>
        {edited ? <ThemedText style={styles.edited}>{i18n.t('pathDetails.edited')}</ThemedText> : null}
      </View>
      <View style={[styles.metadata, compact ? styles.metadataCompact : null]}>
        {!hideParticipant ? <ThemedText style={styles.participant}>{i18n.t('pathDetails.participant', {
          participant: detail.activity.participantId,
        })}</ThemedText> : null}
        <ThemedText style={styles.duration}>{duration}</ThemedText>
      </View>
    </View>
    <SettingsIcon systemName="chevron.right" variant="disclosure" />
  </Pressable>;
}

const styles = StyleSheet.create({
  day: {
    gap: mobileTheme.spacing.sm,
  },
  dayHeading: {
    color: mobileTheme.colors.textMuted,
    paddingHorizontal: mobileTheme.spacing.xs,
    ...mobileTheme.typography.caption,
  },
  edited: {
    backgroundColor: mobileTheme.colors.offlineSurface,
    borderRadius: mobileTheme.radii.pill,
    color: mobileTheme.colors.accent,
    overflow: 'hidden',
    paddingHorizontal: mobileTheme.spacing.xs,
    paddingVertical: mobileTheme.spacing.xxs,
    ...mobileTheme.typography.caption,
  },
  duration: {
    color: mobileTheme.colors.accent,
    fontVariant: ['tabular-nums'],
    ...mobileTheme.typography.caption,
  },
  metadata: {
    alignItems: 'baseline',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'space-between',
  },
  metadataCompact: {
    alignItems: 'flex-start',
    flexDirection: 'column',
    gap: mobileTheme.spacing.xxs,
  },
  participant: {
    color: mobileTheme.colors.textMuted,
    flexShrink: 1,
    ...mobileTheme.typography.caption,
  },
  row: {
    alignItems: 'center',
    backgroundColor: mobileTheme.colors.surfaceRaised,
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  rowCompact: {
    alignItems: 'flex-start',
    paddingVertical: mobileTheme.spacing.md,
  },
  rowCopy: {
    flex: 1,
    gap: mobileTheme.spacing.xxs,
    minWidth: 0,
  },
  rowHeading: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: mobileTheme.spacing.xs,
  },
  rowPressed: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  rows: {
    borderRadius: mobileTheme.radii.lg,
    gap: mobileTheme.sizes.border,
    overflow: 'hidden',
  },
  stack: {
    gap: mobileTheme.spacing.lg,
  },
  time: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
    fontWeight: '600',
  },
});
