import { Fragment } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { ActivityDetail } from '../activity-history';
import { activityWasEdited } from '../activity-history';
import { formatGoalDuration } from './compact-duration';
import { StatusBanner } from './primitives';
import { SettingsActionRow, SettingsNavigationRow, SettingsSection, SettingsSeparator } from './settings-list';

export function RecentPathActivity({ activities, busy, errorText, i18n, onRetry, onSeeAll, onOpen, onAdd, participantName, ownerID }: {
  activities: readonly ActivityDetail[];
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
    {busy ? <StatusBanner text={i18n.t('common.loading')} /> : null}
    {errorText ? <StatusBanner text={errorText} tone="error" actionLabel={i18n.t('common.retry')} onAction={onRetry} /> : null}
    {!busy && !errorText && activities.length === 0 ? <StatusBanner text={i18n.t('pathDetails.historyEmpty')} /> : null}
    <SettingsSection>
      {activities.slice(0, 2).map((detail) => {
        const activity = detail.activity;
        const date = i18n.date(new Date(activity.startedAt), { dateStyle: 'medium', timeZone: activity.occurrenceTimeZone });
        const time = i18n.time(new Date(activity.startedAt), { timeStyle: 'short', timeZone: activity.occurrenceTimeZone });
        const duration = formatGoalDuration(activity.durationSeconds, i18n);
        return <Fragment key={activity.id}>
          <SettingsNavigationRow
            accessibilityLabel={i18n.t(activityWasEdited(detail) ? 'pathDetails.historyRowEdited' : 'pathDetails.historyRow', { duration, time, participant: participantName(activity.participantId) })}
            label={date}
            context={activity.participantId === ownerID && activity.note ? activity.note : participantName(activity.participantId)}
            value={duration}
            onPress={() => onOpen(activity.id)}
          />
          <SettingsSeparator />
        </Fragment>;
      })}
      <SettingsNavigationRow accessibilityLabel={i18n.t('pathDetails.seeAll')} label={i18n.t('pathDetails.seeAll')} onPress={onSeeAll} />
      {onAdd ? <><SettingsSeparator /><SettingsActionRow accessibilityLabel={i18n.t('activity.add')} label={i18n.t('activity.add')} onPress={onAdd} /></> : null}
    </SettingsSection>
  </>;
}
