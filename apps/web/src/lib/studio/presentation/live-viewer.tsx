import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adjacentPage, liveProgress, type ActivePerson } from '../social/domain/activity';
import type { StudioDependencies } from './app';
import { Avatar } from './avatar';
import { duration } from './duration';

export function LiveViewer({ people, selected, close, refresh, dependencies: d }: {
  people: readonly ActivePerson[]; selected: string; close(): void; refresh(): void; dependencies: StudioDependencies;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [current, setCurrent] = useState(selected);
  const [now, setNow] = useState(d.now);
  const pages = people.flatMap(group => group.paths.map(path => ({ ...path, person: group.person })));
  const page = pages.find(value => value.id === current);
  const appearance = useQuery({ queryKey: [d.accountScope, 'appearance', page?.pathId], queryFn: ({ signal }) => d.paths.appearance(page!.pathId, signal), enabled: Boolean(page) });
  useEffect(() => { dialog.current?.showModal(); return () => dialog.current?.close(); }, []);
  useEffect(() => { const timer = setInterval(() => setNow(d.now()), 1000); return () => clearInterval(timer); }, [d]);
  useEffect(() => { if (!page) close(); }, [page, close]);
  const progress = page ? liveProgress(page, now) : null;
  useEffect(() => { if (progress?.goalExpired) refresh(); }, [progress?.goalExpired, refresh]);
  const go = (direction: -1 | 1) => { const next = adjacentPage(pages, current, direction); if (next) setCurrent(next); else close(); };
  if (!page || !progress) return null;
  const siblings = pages.filter(item => item.person.id === page.person.id);
  const goal = page.goal?.targetSeconds ?? page.overallTarget;
  const value = page.goal ? progress.goalSeconds : progress.totalSeconds;
  return <dialog ref={dialog} className="studio-live" data-color={appearance.data?.color ?? 'lavender'} onCancel={close} onClose={close} onKeyDown={event => {
    if (event.key === 'ArrowLeft') { event.preventDefault(); go(-1); }
    if (event.key === 'ArrowRight') { event.preventDefault(); go(1); }
  }} aria-label={d.i18n.t('studio.social.liveTitle', { name: page.person.name })}>
    <div className="studio-live-pages" aria-hidden="true">{siblings.map(item => <span key={item.id} data-current={item.id === current} />)}</div>
    <header><Avatar person={page.person} /><div><strong>{page.person.name}</strong><small>@{page.person.username}</small></div><button onClick={close} aria-label={d.i18n.t('studio.social.close')}>×</button></header>
    <button className="studio-live-zone studio-live-back" onClick={() => go(-1)} aria-label={d.i18n.t('studio.social.previous')} />
    <button className="studio-live-zone studio-live-next" onClick={() => go(1)} aria-label={d.i18n.t('studio.social.next')} />
    <section className="studio-live-content" key={page.person.id} aria-live="polite">
      <span className="studio-live-emoji" aria-hidden="true">{appearance.data?.emoji ?? '✨'}</span><h1>{page.pathName}</h1>
      <p className="studio-live-time">{duration(d.i18n, progress.sessionSeconds)}</p><p>{d.i18n.t('studio.social.tracking')}</p>
      {goal != null && value != null && !progress.goalExpired && <div className="studio-live-goal"><strong>{d.i18n.t('studio.social.goalProgress', { time: duration(d.i18n, value), goal: duration(d.i18n, goal) })}</strong>
        <progress value={Math.min(value, goal)} max={goal} aria-label={d.i18n.t('studio.progress')} />
        <small>{page.goal && ['hourly', 'daily', 'weekly', 'monthly', 'yearly'].includes(page.goal.recurrence) ? d.i18n.t(`studio.period.${page.goal.recurrence}` as 'studio.period.hourly') : d.i18n.t('studio.totalTime')}</small>
      </div>}
      {progress.goalExpired && <p>{d.i18n.t('studio.social.refreshingGoal')}</p>}
    </section>
  </dialog>;
}
