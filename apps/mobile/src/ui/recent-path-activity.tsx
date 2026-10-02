import { Fragment } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { ActivityDetail } from '../activity-history';
import { activityWasEdited } from '../activity-history';
import { formatGoalDuration } from './compact-duration';
import { StatusBanner, ThemedText } from './primitives';
import { SettingsActionRow, SettingsNavigationRow, SettingsValueRow, SettingsSection, SettingsSeparator } from './settings-list';

export function RecentPathActivity({ retained = false, incomplete = false, activities, busy, errorText, i18n, onRetry, onSeeAll, onOpen, onAdd, participantName, ownerID }: {
  activities: readonly ActivityDetail[];
  retained?: boolean;
  incomplete?: boolean;
  busy: boolean;
  errorText?: string;
  i18n: Translator;
  onRetry: () => void;
  onSeeAll: () => void;
  onOpen: (id: string) => void;
  onAdd?: () => void;
  participantName: (id: string) => string;
  ownerID: string;
}) {
  return <>
    {retained ? <StatusBanner tone="offline" text={i18n.t(incomplete ? 'offline.historyIncomplete' : 'offline.historyRetained')} /> : null}
    {busy ? <StatusBanner text={i18n.t('common.loading')} /> : null}
    {errorText ? <StatusBanner text={errorText} tone="error" actionLabel={i18n.t('common.retry')} onAction={onRetry} /> : null}
    {!busy && !errorText && !(retained && incomplete) && activities.length === 0 ? <StatusBanner text={i18n.t('pathDetails.historyEmpty')} /> : null}
    <SettingsSection>
      {activities.slice(0, 2).map((detail) => {
        const activity = detail.activity;
        const date = i18n.date(new Date(activity.startedAt), { dateStyle: 'medium', timeZone: activity.occurrenceTimeZone });
        const time = i18n.time(new Date(activity.startedAt), { timeStyle: 'short', timeZone: activity.occurrenceTimeZone });
        const duration = formatGoalDuration(activity.durationSeconds, i18n);
        return <Fragment key={activity.id}>
          {detail.retained || detail.pending ? <SettingsValueRow label={date} value={duration} /> : <SettingsNavigationRow
            accessibilityLabel={i18n.t(activityWasEdited(detail) ? 'pathDetails.historyRowEdited' : 'pathDetails.historyRow', { duration, time, participant: participantName(activity.participantId) })}
            label={date}
            context={activity.participantId === ownerID && activity.note ? activity.note : participantName(activity.participantId)}
            value={duration}
            onPress={() => onOpen(activity.id)}
          />}
          {detail.pending ? <ThemedText>{i18n.t('offline.pending')}</ThemedText> : null}
          <SettingsSeparator />
        </Fragment>;
      })}
      <SettingsNavigationRow accessibilityLabel={i18n.t('pathDetails.seeAll')} label={i18n.t('pathDetails.seeAll')} onPress={onSeeAll} />
      {onAdd ? <><SettingsSeparator /><SettingsActionRow accessibilityLabel={i18n.t('activity.add')} label={i18n.t('activity.add')} onPress={onAdd} /></> : null}
    </SettingsSection>
  </>;
}
