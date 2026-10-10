import { useId, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { initialStatsCalendarSelection, statsCalendarRanges, statsRangeKeys } from '@hourpaths/client-core';
import type { StudioDependencies } from './app';
import type { Range } from '../analytics/domain/statistics';
import { ContributionGrid } from './statistics-charts';
import { periodLabel } from './statistics-format';

export function StatisticsCalendar({ pathIds, dependencies: d }: { pathIds: readonly string[]; dependencies: StudioDependencies }) {
  const rangeId = useId();
  const [period, setPeriod] = useState(initialStatsCalendarSelection);
  const selection = { ...period, pathIds };
  const navigable = period.range !== 'past_year' && period.range !== 'all_time';
  const query = useQuery({ queryKey: [d.accountScope, 'statistics-calendar', selection], queryFn: ({ signal }) => d.statistics.load(selection, signal), refetchOnWindowFocus: true });
  const data = query.isError ? undefined : query.data;
  return <div>
    <div className="studio-stats-controls"><label htmlFor={rangeId}>{d.i18n.t('stats.calendarRange')}</label><select id={rangeId} value={period.range} onChange={event => setPeriod({ ...period, range: event.target.value as Range, anchor: undefined })}>{statsCalendarRanges.map(value => <option key={value} value={value}>{d.i18n.t(statsRangeKeys[value])}</option>)}</select><button onClick={() => void query.refetch()} disabled={query.isFetching}>{d.i18n.t('stats.refresh')}</button></div>
    {data && <div className="studio-stats-period">
      {navigable && <button aria-label={d.i18n.t('stats.previous')} disabled={!data.previous} onClick={() => setPeriod({ ...period, anchor: data.previous ?? undefined })}>‹</button>}
      <strong>{periodLabel(data, d.i18n)}</strong>
      {navigable && <button aria-label={d.i18n.t('stats.next')} disabled={!data.next} onClick={() => setPeriod({ ...period, anchor: data.next ?? undefined })}>›</button>}
      {period.anchor && <button onClick={() => setPeriod({ ...period, anchor: undefined })}>{d.i18n.t('stats.current')}</button>}
    </div>}
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {query.isError && <p role="alert">{d.i18n.t('stats.unavailable')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {data && <ContributionGrid statistics={data} selection={JSON.stringify(selection)} dependencies={d} />}
  </div>;
}
