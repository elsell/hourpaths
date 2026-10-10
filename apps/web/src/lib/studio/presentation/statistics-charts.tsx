import { useEffect, useRef, useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { calendarGroups, contributionLevel, contributionWeeks, type Bucket, type CalendarUnit, type Statistics } from '../analytics/domain/statistics';
import { duration } from './duration';
import { calendarLabel } from './statistics-format';

export function ActivityChart({ buckets, unit, selection, dependencies: d }: { buckets: readonly Bucket[]; unit: string; selection: string; dependencies: StudioDependencies }) {
  const scroll = useRef<HTMLDivElement>(null);
  useEffect(() => { if (scroll.current) scroll.current.scrollLeft = scroll.current.scrollWidth; }, [selection]);
  const maximum = Math.max(1, ...buckets.map(bucket => bucket.seconds));
  return <section className="studio-stat-panel"><h2>{d.i18n.t('stats.activity')}</h2>
    <div className="studio-chart-scroll" ref={scroll} tabIndex={0} role="region" aria-label={d.i18n.t('stats.activity')}><ol className="studio-bars">{buckets.map(bucket => <li key={bucket.key}>
      <strong>{duration(d.i18n, bucket.seconds)}</strong>
      <svg viewBox="0 0 32 160" preserveAspectRatio="none" aria-hidden="true"><rect x="4" y={160 - bucket.seconds / maximum * 155} width="24" height={bucket.seconds / maximum * 155} rx="4" /></svg>
      <time dateTime={bucket.key}>{calendarLabel(bucket.key, unit, d.i18n)}</time>
      <span className="studio-sr-only">{d.i18n.t('duration.compactSeconds', { seconds: d.i18n.number(bucket.seconds) })}</span>
    </li>)}</ol></div>
  </section>;
}
export function ContributionGrid({ statistics: data, selection, dependencies: d }: { statistics: Statistics; selection: string; dependencies: StudioDependencies }) {
  const [grouping, setGrouping] = useState<CalendarUnit>('day');
  const [selected, setSelected] = useState<string | null>(null);
  const scroll = useRef<HTMLDivElement>(null);
  useEffect(() => { setSelected(null); if (scroll.current) scroll.current.scrollLeft = scroll.current.scrollWidth; }, [selection, grouping]);
  const weeks = contributionWeeks(data.days, data.weekStartsOn, { startDate: data.firstDate, endDate: data.lastDate });
  const maximum = Math.max(1, ...data.days.map(day => day.seconds));
  const chosen = weeks.flat().find(day => day?.date === selected);
  const label = (date: string, seconds: number) => d.i18n.t('stats.chartValue', { label: d.i18n.date(new Date(date + 'T12:00:00Z'), { dateStyle: 'long', timeZone: 'utc' }), duration: d.i18n.t('duration.compactSeconds', { seconds: d.i18n.number(seconds) }) });
  const summary = calendarGroups(data.days, grouping, data.weekStartsOn);
  return <section className="studio-stat-panel"><header className="studio-section-header"><h2>{d.i18n.t('stats.contribution.title')}</h2><label><span className="studio-sr-only">{d.i18n.t('stats.calendarGrouping')}</span><select value={grouping} onChange={e => setGrouping(e.target.value as CalendarUnit)}>{(['day', 'week', 'month', 'year'] as const).map(unit => <option key={unit} value={unit}>{d.i18n.t(`stats.calendar.${unit}`)}</option>)}</select></label></header>
    {grouping === 'day' ? <div className="studio-contribution-body"><div className="studio-weekdays" aria-hidden="true">{Array.from({ length: 7 }, (_, index) => <span key={index}>{d.i18n.date(Date.UTC(2026, 0, 5 + data.weekStartsOn - 1 + index), { weekday: 'short', timeZone: 'utc' })}</span>)}</div>
      <div className="studio-contribution-scroll" ref={scroll} tabIndex={0} role="region" aria-label={d.i18n.t('stats.contribution.title')}><div className="studio-contribution">{weeks.map((week, index) => {
        const month = week.find(day => day && (index === 0 || day.date.endsWith('-01')));
        return <div className="studio-contribution-week" key={week.find(day => day)?.date ?? index}><span className="studio-contribution-month">{month ? d.i18n.date(new Date(month.date + 'T12:00:00Z'), { month: 'short', timeZone: 'utc' }) : ''}</span>
          {week.map((day, row) => day ? <button key={day.date} className="studio-contribution-day" data-level={contributionLevel(day.seconds, maximum)} aria-label={label(day.date, day.seconds)} title={label(day.date, day.seconds)} aria-pressed={day.date === selected} onClick={() => setSelected(day.date)} /> : <span className="studio-contribution-placeholder" key={row} />)}
        </div>;
      })}</div></div></div> : <ul className="studio-calendar-summary">{summary.map(bucket => <li key={bucket.key}><time>{calendarLabel(bucket.key, grouping === 'week' ? 'day' : grouping, d.i18n)}</time><strong>{duration(d.i18n, bucket.seconds)}</strong><span className="studio-sr-only">{d.i18n.t('duration.compactSeconds', { seconds: d.i18n.number(bucket.seconds) })}</span></li>)}</ul>}
    <div className="studio-contribution-legend"><span>{d.i18n.t('stats.contribution.less')}</span>{[0, 1, 2, 3, 4].map(level => <span key={level} data-level={level} aria-hidden="true" />)}<span>{d.i18n.t('stats.contribution.more')}</span></div>
    {chosen && grouping === 'day' && <p role="status">{label(chosen.date, chosen.seconds)}</p>}
  </section>;
}
export function PathDistribution({ statistics: data, dependencies: d }: { statistics: Statistics; dependencies: StudioDependencies }) {
  const appearances = useQueries({ queries: data.distribution.map(path => ({ queryKey: [d.accountScope, 'appearance', path.id], queryFn: ({ signal }: { signal: AbortSignal }) => d.paths.appearance(path.id, signal) })) });
  let offset = 0;
  return <section className="studio-stat-panel"><h2>{d.i18n.t('stats.distribution')}</h2><div className="studio-distribution">
    <svg className="studio-donut" viewBox="0 0 120 120" aria-hidden="true"><circle cx="60" cy="60" r="44" fill="none" stroke="#e1e5ed" strokeWidth="16" />{data.distribution.map((path, index) => {
      const fraction = data.seconds ? path.seconds / data.seconds : 0;
      const start = offset; offset += fraction;
      return <circle key={path.id} cx="60" cy="60" r="44" fill="none" strokeWidth="16" pathLength="1" strokeDasharray={[fraction, 1 - fraction].join(' ')} strokeDashoffset={-start} transform="rotate(-90 60 60)" data-color={appearances[index].data?.color ?? 'lavender'} />;
    })}</svg>
    <table className="studio-distribution-table"><thead><tr><th>{d.i18n.t('studio.path')}</th><th>{d.i18n.t('stats.total')}</th><th>{d.i18n.t('studio.stats.share')}</th></tr></thead><tbody>{data.distribution.map((path, index) => <tr key={path.id}>
      <td><span className="studio-emoji" data-color={appearances[index].data?.color} aria-hidden="true">{appearances[index].data?.emoji ?? '✨'}</span><strong>{path.name}</strong></td>
      <td><span aria-hidden="true">{duration(d.i18n, path.seconds)}</span><span className="studio-sr-only">{d.i18n.t('duration.compactSeconds', { seconds: d.i18n.number(path.seconds) })}</span></td>
      <td>{d.i18n.number(data.seconds ? path.seconds / data.seconds : 0, { style: 'percent', maximumFractionDigits: 1 })}</td>
    </tr>)}</tbody></table>
  </div></section>;
}
