import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { NewPath, Recurrence } from '../paths/domain/path';

export function CreatePath({ dependencies: d, close }: { dependencies: StudioDependencies; close(): void }) {
  const client = useQueryClient();
  const [name, setName] = useState('');
  const [visibility, setVisibility] = useState<NewPath['visibility']>('private');
  const [interval, setInterval] = useState(false);
  const [overall, setOverall] = useState(false);
  const [minutes, setMinutes] = useState('30');
  const [overallMinutes, setOverallMinutes] = useState('60');
  const [recurrence, setRecurrence] = useState<Recurrence>('daily');
  const [submission, setSubmission] = useState<{ fingerprint: string; id: string } | null>(null);
  const mutation = useMutation({
    mutationFn: ({ draft, id }: { draft: NewPath; id: string }) => d.paths.create(draft, id),
    onSuccess: async () => { await client.invalidateQueries({ queryKey: [d.accountScope, 'paths'] }); close(); },
  });
  return <form className="studio-form" onSubmit={event => {
    event.preventDefault();
    const draft: NewPath = { name: name.trim(), visibility,
      ...(interval ? { intervalGoal: { targetSeconds: Number(minutes) * 60, recurrence } } : {}),
      ...(overall ? { overallTarget: { targetSeconds: Number(overallMinutes) * 60 } } : {}),
    };
    const fingerprint = JSON.stringify(draft);
    const id = submission?.fingerprint === fingerprint ? submission.id : d.operationId();
    setSubmission({ fingerprint, id }); mutation.mutate({ draft, id });
  }}>
    <h2>{d.i18n.t('pathCreate.heading')}</h2>
    <fieldset disabled={mutation.isPending}>
      <label>{d.i18n.t('pathCreate.nameLabel')}<input required maxLength={100} value={name} onChange={event => setName(event.target.value)} /></label>
      <label>{d.i18n.t('pathVisibility.choiceLabel')}<select value={visibility} onChange={event => setVisibility(event.target.value as NewPath['visibility'])}>{(['private', 'followers', 'public'] as const).map(value => <option value={value} key={value}>{d.i18n.t(`pathVisibility.option.${value}`)}</option>)}</select></label>
      <label className="studio-checkbox"><input type="checkbox" checked={interval} onChange={event => setInterval(event.target.checked)} />{d.i18n.t('pathCreate.intervalEnabled')}</label>
      {interval && <div className="studio-form-pair"><label>{d.i18n.t('pathCreate.duration.minutes')}<input type="number" required min={1} step={1} value={minutes} onChange={event => setMinutes(event.target.value)} /></label><label>{d.i18n.t('pathCreate.recurrence')}<select value={recurrence} onChange={event => setRecurrence(event.target.value as Recurrence)}>{(['hourly', 'daily', 'weekly', 'monthly', 'yearly'] as const).map(value => <option value={value} key={value}>{d.i18n.t(`pathCreate.recurrence.${value}`)}</option>)}</select></label></div>}
      <label className="studio-checkbox"><input type="checkbox" checked={overall} onChange={event => setOverall(event.target.checked)} />{d.i18n.t('pathCreate.overallEnabled')}</label>
      {overall && <label>{d.i18n.t('pathCreate.duration.minutes')}<input type="number" required min={1} step={1} value={overallMinutes} onChange={event => setOverallMinutes(event.target.value)} /></label>}
      <div className="studio-form-actions"><button type="button" onClick={close}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" type="submit" disabled={!name.trim()}>{d.i18n.t(mutation.isPending ? 'pathCreate.submitting' : 'pathCreate.submit')}</button></div>
    </fieldset>
    {mutation.isError && <p role="alert">{d.i18n.t('errors.temporarilyUnavailable')}</p>}
  </form>;
}
