import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { ranges, StatisticsUnavailable, type Selection, type Statistics } from '../analytics/domain/statistics';
import { StudioShell } from './studio-shell';
import { ActivityTimeline } from './activity-timeline';
import { ActivityChart, ContributionGrid, PathDistribution } from './statistics-charts';
import { periodLabel, rangeLabels } from './statistics-format';
import { duration } from './duration';

export function StatisticsPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const [selection, setSelection] = useState<Selection>({ range: 'week', pathIds: [] });
  const [paths, setPaths] = useState<Statistics['paths']>([]);
  const query = useQuery({ queryKey: [d.accountScope, 'statistics', selection], queryFn: ({ signal }) => d.statistics.load(selection, signal), refetchOnWindowFocus: true });
  useEffect(() => { if (query.data) setPaths(query.data.paths); }, [query.data]);
  const data = query.isError && (!(query.error instanceof StatisticsUnavailable) || !query.error.retryable) ? undefined : query.data;
  const signature = JSON.stringify(selection);
  const toggle = (id: string) => setSelection(current => ({ ...current, pathIds: current.pathIds.includes(id) ? current.pathIds.filter(value => value !== id) : [...current.pathIds, id].sort() }));
  return <StudioShell page="stats" i18n={d.i18n}><main className="studio-main studio-stats-main"><header className="studio-header"><div><h1>{d.i18n.t('stats.title')}</h1><p>{d.i18n.t('studio.stats.intro')}</p></div><button onClick={() => void query.refetch()} disabled={query.isFetching}>{d.i18n.t('stats.refresh')}</button></header>
    <div className="studio-stats-controls"><div className="studio-range-control" role="group" aria-label={d.i18n.t('stats.rangeLabel')}>{ranges.map(range => <button key={range} aria-pressed={selection.range === range} onClick={() => setSelection({ ...selection, range, anchor: undefined })}>{d.i18n.t(rangeLabels[range])}</button>)}</div>
      <details className="studio-stat-filter"><summary>{selection.pathIds.length ? d.i18n.t('stats.selectedPaths', { count: selection.pathIds.length }) : d.i18n.t('stats.allPaths')}</summary><div><button aria-pressed={!selection.pathIds.length} onClick={() => setSelection({ ...selection, pathIds: [] })}>{d.i18n.t('stats.allPaths')}</button>{paths.map(path => <label key={path.id}><input type="checkbox" checked={selection.pathIds.includes(path.id)} onChange={() => toggle(path.id)} />{path.name}{path.archived && <small>{d.i18n.t('stats.archived')}</small>}</label>)}</div></details>
    </div>
    <div className="studio-stats-period"><button aria-label={d.i18n.t('stats.previous')} disabled={!data?.previous || query.isPending || selection.range === 'all_time'} onClick={() => setSelection({ ...selection, anchor: data?.previous ?? undefined })}>‹</button><strong>{data ? periodLabel(data, d.i18n) : d.i18n.t(rangeLabels[selection.range])}</strong><button aria-label={d.i18n.t('stats.next')} disabled={!data?.next || query.isPending || selection.range === 'all_time'} onClick={() => setSelection({ ...selection, anchor: data?.next ?? undefined })}>›</button>{selection.anchor && <button onClick={() => setSelection({ ...selection, anchor: undefined })}>{d.i18n.t('stats.current')}</button>}</div>
    {query.isFetching && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {query.isError && <p role="alert">{d.i18n.t('stats.unavailable')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {data && <><div className="studio-stat-cards"><section><span>{d.i18n.t('stats.total')}</span><strong>{duration(d.i18n, data.seconds)}</strong></section><section><span>{d.i18n.t('studio.stats.activeDays')}</span><strong>{d.i18n.number(data.days.filter(day => day.seconds > 0).length)}</strong></section><section><span>{d.i18n.t('studio.stats.activePaths')}</span><strong>{d.i18n.number(data.distribution.filter(path => path.seconds > 0).length)}</strong></section></div>
      {data.seconds === 0 && <div className="studio-stat-panel"><h2>{d.i18n.t('stats.emptyTitle')}</h2><p>{d.i18n.t('stats.emptyDescription')}</p></div>}
      <ActivityChart buckets={data.buckets} unit={data.bucketUnit} selection={signature} dependencies={d} />
      <ContributionGrid statistics={data} selection={signature} dependencies={d} />
      <PathDistribution statistics={data} dependencies={d} />
    </>}
  </main><ActivityTimeline paths={d.paths} history={d.history} accountScope={d.accountScope} i18n={d.i18n} /></StudioShell>;
}
