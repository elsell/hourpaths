import { useState, useEffect } from 'react';
import { Link } from '@tanstack/react-router';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { statsCalendarGroups } from '@hourpaths/client-core';
import type { StudioDependencies } from './app';
import type { Path } from '../paths/domain/path';
import { ActivityChart, ContributionGrid } from './statistics-charts';
import { ConnectedTimeline } from './connected-timeline';
import { duration } from './duration';

export function PathStatisticsPanel({ path, dependencies: d }: { path: Path; dependencies: StudioDependencies }) {
  const [expandedHistory, expandHistory] = useState(false);
  const [selected, select] = useState<string | null>(null);
  const viewer = useQuery({ queryKey: [d.accountScope, 'viewer'], queryFn: ({ signal }) => d.social.viewer(signal) });
  const people = useInfiniteQuery({ queryKey: [d.accountScope, 'path-statistics-members', path.id], initialPageParam: '',
    queryFn: ({ pageParam, signal }) => d.sharing.members(path.id, pageParam, signal), getNextPageParam: page => page.nextCursor || undefined, staleTime: 0, gcTime: 0 });
  const members = people.isError ? [] : [...new Map((people.data?.pages.flatMap(page => page.items) ?? []).filter(member => member.role !== 'supporter').map(member => [member.userId, member])).values()];
  const participant = selected ?? (path.canTrack ? viewer.data ?? '' : '');
  const identity = [d.accountScope, path.id, participant].join(':');
  useEffect(() => expandHistory(false), [identity]);
  const stats = useQuery({ queryKey: [d.accountScope, 'path-statistics', path.id, participant], enabled: !!participant,
    queryFn: ({ signal }) => d.statistics.path(path.id, participant, signal), staleTime: 0, gcTime: 0 });
  const history = useInfiniteQuery({ queryKey: [d.accountScope, 'path-history', path.id, participant], enabled: !!participant, initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) => d.pathHistory.read(path.id, path.name, participant, pageParam, signal), getNextPageParam: page => page.next ?? undefined, staleTime: 0, gcTime: 0 });
  const data = stats.isError || stats.isFetching ? undefined : stats.data;
  const entries = history.isError ? [] : history.data?.pages.flatMap(page => page.items) ?? [];
  const failed = people.isError || viewer.isError || (!!participant && (stats.isError || history.isError));
  return <section className="studio-path-statistics">
    <header className="studio-section-header"><h2>{d.i18n.t('pathStats.heading')}</h2><label>{d.i18n.t('pathStats.participant')} <select className="studio-select" value={participant} onChange={event => select(event.target.value)}>
      {!participant && <option value="">{d.i18n.t('pathStats.choose')}</option>}
      {path.canTrack && viewer.data && <option value={viewer.data}>{d.i18n.t('pathStats.yours')}</option>}
      {members.filter(member => !path.canTrack || member.userId !== viewer.data).map(member => <option key={member.userId} value={member.userId}>{member.displayName}</option>)}
    </select></label></header>
    {!path.canTrack && !participant && <ul className="studio-distribution-table">{members.map(member => <li key={member.userId}><button onClick={() => select(member.userId)}>{member.displayName}</button> <strong>{duration(d.i18n, member.totalTrackedSeconds)}</strong></li>)}</ul>}
    {people.hasNextPage && <button disabled={people.isFetching} onClick={() => void people.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
    {failed && <div role="alert"><p>{d.i18n.t('stats.unavailable')}</p><button onClick={() => { void people.refetch(); void viewer.refetch(); if (participant) { void stats.refetch(); void history.refetch(); } }}>{d.i18n.t('common.retry')}</button></div>}
    {participant && stats.isPending && <p role="status">{d.i18n.t('common.loading')}</p>}
    {data && <>
      <div className="studio-stat-cards"><section><span>{d.i18n.t('stats.total')}</span><strong>{duration(d.i18n, data.totalSeconds)}</strong></section><section><span>{d.i18n.t('pathStats.sessions')}</span><strong>{d.i18n.number(data.sessionCount)}</strong></section><section><span>{d.i18n.t('pathStats.average')}</span><strong>{duration(d.i18n, data.averageSeconds)}</strong></section></div>
      <ActivityChart buckets={statsCalendarGroups(data.days, 'month', data.weekStartsOn)} unit="month" selection={identity} dependencies={d} />
      <ContributionGrid statistics={data} intensity="quartile" selection={identity} dependencies={d} />
    </>}
    {participant && <section className="studio-history"><h2>{d.i18n.t('pathDetails.recentActivity')}</h2>
      {history.isPending && <p role="status">{d.i18n.t('common.loading')}</p>}
      {!history.isPending && !history.isError && !entries.length && <p>{d.i18n.t('pathDetails.historyEmpty')}</p>}
      <ConnectedTimeline items={expandedHistory ? entries : entries.slice(0, 2)} identity={entry => entry.id} instant={entry => entry.startedAt} timeZone={entry => entry.timeZone} i18n={d.i18n} render={entry => <>
        <time dateTime={new Date(entry.startedAt).toISOString()}>{d.i18n.time(entry.startedAt, { hour: 'numeric', minute: '2-digit', timeZone: entry.timeZone })}</time>
        <Link to="/paths/$pathId/activities/$activityId" params={{ pathId: path.id, activityId: entry.id }}>{duration(d.i18n, entry.seconds)}</Link>
        {(entry.version ?? 1) > 1 && <small>{d.i18n.t('pathDetails.edited')}</small>}
      </>} />
      {!expandedHistory && (entries.length > 2 || history.hasNextPage) && <button onClick={() => expandHistory(true)}>{d.i18n.t('pathDetails.seeAll')}</button>}
      {expandedHistory && history.hasNextPage && <button disabled={history.isFetching} onClick={() => void history.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
    </section>}
  </section>;
}
