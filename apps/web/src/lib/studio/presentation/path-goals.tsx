import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { GoalAlignment, Path, PathGoals, Recurrence } from '../paths/domain/path';

const alignmentFields: Record<Recurrence, readonly (keyof GoalAlignment)[]> = {
  hourly: ['minute'], daily: ['hour', 'minute'], weekly: ['isoWeekday', 'hour', 'minute'],
  monthly: ['day', 'hour', 'minute'], yearly: ['month', 'day', 'hour', 'minute'],
};
const limits = { minute: [0, 59], hour: [0, 23], day: [1, 31], month: [1, 12], isoWeekday: [1, 7] } as const;
export function PathGoalsEditor({ path, dependencies: d, close }: { path: Path; dependencies: StudioDependencies; close(): void }) {
  const client = useQueryClient();
  const [interval, setInterval] = useState(!!path.goal);
  const [overall, setOverall] = useState(path.overallTarget !== null);
  const [duration, setDuration] = useState(String(path.goal?.targetSeconds ?? 1800));
  const [total, setTotal] = useState(String(path.overallTarget ?? 3600));
  const [recurrence, setRecurrence] = useState<Recurrence>(path.goal?.recurrence ?? 'daily');
  const [alignment, setAlignment] = useState<GoalAlignment>(path.goal?.alignment ?? { hour: 0, minute: 0 });
  const [review, setReview] = useState<PathGoals | null>(null);
  const mutation = useMutation({ mutationFn: (goals: PathGoals) => d.paths.saveGoals(path, goals, d.operationId()), onSuccess: async () => {
    await Promise.all([client.invalidateQueries({ queryKey: [d.accountScope, 'path', path.id] }), client.invalidateQueries({ queryKey: [d.accountScope, 'paths'] }), client.invalidateQueries({ queryKey: [d.accountScope, 'tracking', path.id] })]); close();
  } });
  const summary = (value: PathGoals) => <dl><dt>{d.i18n.t('pathCreate.intervalHeading')}</dt><dd>{value.goal ? <>{d.i18n.t('studio.duration', { minutes: Math.floor(value.goal.targetSeconds / 60), seconds: value.goal.targetSeconds % 60 })} · {d.i18n.t(`pathCreate.recurrence.${value.goal.recurrence}`)}</> : d.i18n.t('pathManage.noInterval')}</dd><dt>{d.i18n.t('pathCreate.overallHeading')}</dt><dd>{value.overallTarget === null ? d.i18n.t('pathManage.noOverall') : d.i18n.t('studio.duration', { minutes: Math.floor(value.overallTarget / 60), seconds: value.overallTarget % 60 })}</dd></dl>;
  return <section className="studio-form studio-path-details">
    <header><h2>{path.name}</h2><button onClick={close} disabled={mutation.isPending}>{d.i18n.t('common.cancel')}</button></header>
    {!path.canManageGoals ? summary(path) : review ? <div>
      <h3>{d.i18n.t('pathManage.reviewHeading')}</h3><p>{d.i18n.t('pathManage.reviewExplanation')}</p>
      <div className="studio-form-pair"><section><h4>{d.i18n.t('pathManage.current')}</h4>{summary(path)}</section><section><h4>{d.i18n.t('pathManage.proposed')}</h4>{summary(review)}</section></div>
      <div className="studio-form-actions"><button disabled={mutation.isPending} onClick={() => setReview(null)}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={mutation.isPending} onClick={() => mutation.mutate(review)}>{d.i18n.t('common.save')}</button></div>
    </div> : <form onSubmit={event => { event.preventDefault(); setReview({ goal: interval ? { targetSeconds: Number(duration), recurrence, alignment } : null, overallTarget: overall ? Number(total) : null }); }}>
      <fieldset><legend>{d.i18n.t('pathManage.goalsHeading')}</legend><p>{d.i18n.t('pathManage.explanation')}</p>
      <label className="studio-checkbox"><input type="checkbox" checked={interval} onChange={event => setInterval(event.target.checked)} />{d.i18n.t('pathCreate.intervalEnabled')}</label>
      {interval && <><div className="studio-form-pair"><label>{d.i18n.t('pathCreate.targetSeconds')}<input type="number" min={1} step={1} required value={duration} onChange={event => setDuration(event.target.value)} /></label><label>{d.i18n.t('pathCreate.recurrence')}<select value={recurrence} onChange={event => { const value = event.target.value as Recurrence; setRecurrence(value); setAlignment(Object.fromEntries(alignmentFields[value].map(field => [field, limits[field][0]]))); }}>{(Object.keys(alignmentFields) as Recurrence[]).map(value => <option key={value} value={value}>{d.i18n.t(`pathCreate.recurrence.${value}`)}</option>)}</select></label></div>
      <div className="studio-form-pair">{alignmentFields[recurrence].map(field => <label key={field}>{d.i18n.t(field === 'isoWeekday' ? 'pathCreate.alignment.weekday' : `pathCreate.alignment.${field}`)}<input type="number" required min={limits[field][0]} max={limits[field][1]} step={1} value={alignment[field] ?? limits[field][0]} onChange={event => setAlignment({ ...alignment, [field]: Number(event.target.value) })} /></label>)}</div></>}
      <label className="studio-checkbox"><input type="checkbox" checked={overall} onChange={event => setOverall(event.target.checked)} />{d.i18n.t('pathCreate.overallEnabled')}</label>
      {overall && <label>{d.i18n.t('pathCreate.targetSeconds')}<input type="number" min={1} step={1} required value={total} onChange={event => setTotal(event.target.value)} /></label>}
      <div className="studio-form-actions"><button className="studio-primary" type="submit">{d.i18n.t('pathManage.reviewHeading')}</button></div></fieldset>
    </form>}
    {mutation.isError && <p role="alert">{d.i18n.t('errors.apiRejected')}</p>}
  </section>;
}
