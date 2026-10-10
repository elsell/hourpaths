import { useEffect, useRef, useState, type ReactNode } from 'react';
import { View } from 'react-native';
import { statsCalendarGroups, statsDateLabel, statsDuration, type PathStatistics } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import type { ActivityDetail } from '../activity-history';
import { NativeChoicePicker } from './native-choice-picker';
import { NativeButton } from './native-button';
import { SectionHeading, StatusBanner, Surface } from './primitives';
import { SettingsSection, SettingsValueRow } from './settings-list';
import { StatsBars, StatsContributionGrid } from './stats-charts';
import { mobileTheme } from './tokens';

type Result = { statistics: PathStatistics; activities: readonly ActivityDetail[] };
export function PathStatisticsView({ pathId, ownerId, canTrack, members, i18n, load, refreshKey, renderRecent, loadMoreParticipants }: {
  pathId: string; ownerId: string; canTrack: boolean;
  members: readonly { userId: string; displayName: string; role: string }[];
  i18n: Translator; load(participant: string): Promise<Result>; refreshKey: string;
  renderRecent(participant: string, activities: readonly ActivityDetail[] | null, loading: boolean, retry: () => void): ReactNode;
  loadMoreParticipants?: () => void;
}) {
  const [participant, setParticipant] = useState(canTrack ? ownerId : '');
  const [retry, setRetry] = useState(0);
  const [state, setState] = useState<{ key: string; result?: Result; failed?: boolean }>({ key: '' });
  const loader = useRef(load); loader.current = load;
  const key = [pathId, participant, refreshKey, canTrack, members.map(member => [member.userId, member.role].join('/')).join(','), retry].join(':');
  useEffect(() => {
    if (!participant) return;
    let current = true;
    setState({ key });
    void loader.current(participant).then(result => { if (current) setState({ key, result }); }, () => { if (current) setState({ key, failed: true }); });
    return () => { current = false; };
  }, [key, participant]);
  const current = state.key === key ? state : undefined;
  const data = current?.result?.statistics;
  const choices = [ ...(canTrack ? [{ value: ownerId, label: i18n.t('pathStats.yours') }] : []),
    ...members.filter(member => member.role !== 'supporter' && (!canTrack || member.userId !== ownerId)).map(member => ({ value: member.userId, label: member.displayName })) ];
  return <View style={{ gap: mobileTheme.spacing.md }}>
    <SectionHeading>{i18n.t('pathStats.heading')}</SectionHeading>
    <NativeChoicePicker label={i18n.t('pathStats.participant')} accessibilityLabel={i18n.t('pathStats.participant')} value={participant} onChange={setParticipant} choices={participant ? choices : [{ value: '', label: i18n.t('pathStats.choose') }, ...choices]} />
    {loadMoreParticipants ? <NativeButton variant="quiet" label={i18n.t('common.loadMore')} onPress={loadMoreParticipants} /> : null}
    {participant && !data && !current?.failed ? <StatusBanner text={i18n.t('common.loading')} /> : null}
    {current?.failed ? <StatusBanner tone="error" text={i18n.t('stats.unavailable')} actionLabel={i18n.t('common.retry')} onAction={() => setRetry(value => value + 1)} /> : null}
    {data ? <>
      <SettingsSection>
        <SettingsValueRow label={i18n.t('stats.total')} value={statsDuration(data.totalSeconds, i18n)} />
        <SettingsValueRow label={i18n.t('pathStats.sessions')} value={i18n.number(data.sessionCount)} />
        <SettingsValueRow label={i18n.t('pathStats.average')} value={statsDuration(data.averageSeconds, i18n)} />
      </SettingsSection>
      <Surface><SectionHeading>{i18n.t('stats.activity')}</SectionHeading><StatsBars key={key} values={statsCalendarGroups(data.days, 'month', data.weekStartsOn).map(bucket => ({ key: bucket.key, label: statsDateLabel(bucket.key, 'month', i18n), seconds: bucket.seconds, color: mobileTheme.colors.accent }))} i18n={i18n} /></Surface>
      <Surface><SectionHeading>{i18n.t('stats.calendar')}</SectionHeading><StatsContributionGrid key={key} days={data.days} range={{ startDate: data.firstDate, endDate: data.lastDate }} weekStartsOn={data.weekStartsOn} intensity="quartile" i18n={i18n} /></Surface>
    </> : null}
    {participant ? renderRecent(participant, current?.result?.activities ?? null, !current?.result && !current?.failed, () => setRetry(value => value + 1)) : null}
  </View>;
}
