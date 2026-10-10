import { ActivityTimingFields } from './activity-timing-fields';
import { useEffect, useRef, useState } from 'react';
import { Link, Navigate, useBlocker, useParams } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createManualActivityFormState, manualActivityParticipantNow, overrideManualActivityOccurrence, updateManualActivityDuration, serializeManualActivityForm, durationParts, secondsFromDurationParts, type ManualActivityFormState } from '@hourpaths/client-core';
import { reviewActivityWrite, saveReviewedActivity, ActivityInputInvalid } from '../history/application/activity-write';
import { ActivityFailure, type ActivityDetail } from '../history/domain/detail';
import type { ActivityFormDefaults, ActivityWrite } from '../history/domain/write';
import type { StudioDependencies } from './app';
import { StudioShell } from './studio-shell';
import { UnsavedChanges } from './unsaved-changes';

export function ActivityEditorPage({ dependencies: d, editing = false }: { dependencies: StudioDependencies; editing?: boolean }) {
  const { pathId, activityId } = useParams({ strict: false }) as { pathId: string; activityId?: string };
  const query = useQuery({ queryKey: [d.accountScope, 'activity-editor', pathId, editing ? activityId : null], staleTime: 0, gcTime: 0,
    queryFn: async ({ signal }) => {
      const defaults = await d.activities.defaults(pathId, signal);
      const detail = editing && activityId ? await d.activities.detail(pathId, activityId, signal) : null;
      if (!defaults.canTrack || (detail && !detail.owned)) throw new ActivityFailure(false);
      return { defaults, detail, loadedAt: d.now() };
    } });
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-settings-main studio-activity-detail">
    <h1>{d.i18n.t(editing ? 'activity.editHeading' : 'activity.addHeading')}</h1>
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {query.isError && <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button><Link to="/">{d.i18n.t('pathDetails.back')}</Link></div>}
    {query.data && <ActivityForm key={`${pathId}:${activityId ?? 'new'}`} pathId={pathId} defaults={query.data.defaults} detail={query.data.detail} loadedAt={query.data.loadedAt} dependencies={d} />}
  </main></StudioShell>;
}
function ActivityForm({ pathId, defaults, detail, loadedAt, dependencies: d }: { pathId: string; defaults: ActivityFormDefaults; detail: ActivityDetail | null; loadedAt: number; dependencies: StudioDependencies }) {
  const client = useQueryClient(), active = useRef(true), admitted = useRef(false);
  const initial = () => {
    if (!detail) return createManualActivityFormState(manualActivityParticipantNow(new Date(defaults.currentInstant).toISOString(), defaults.timeZone));
    const occurrence = manualActivityParticipantNow(new Date(detail.startedAt).toISOString(), detail.timeZone);
    return { localDate: occurrence.localDate, localTime: occurrence.localTime, durationSeconds: String(detail.seconds), occurrenceTouched: true };
  };
  const [form, setForm] = useState<ManualActivityFormState>(initial);
  const [parts, setParts] = useState(() => durationParts(detail ? String(detail.seconds) : ''));
  const [note, setNote] = useState(detail?.note ?? ''), [dirty, setDirty] = useState(false), [busy, setBusy] = useState(false);
  const [error, setError] = useState<'invalid' | 'note' | 'future_end' | 'request' | null>(null), [retryable, setRetryable] = useState(true);
  const [saved, setSaved] = useState<string | null>(null);
  const submission = useRef<{ fingerprint: string; review: ActivityWrite } | null>(null);
  const zone = detail?.timeZone ?? defaults.timeZone;
  const now = () => manualActivityParticipantNow(new Date(defaults.currentInstant).toISOString(), zone, Math.max(0, d.now() - loadedAt));
  useEffect(() => () => { active.current = false; }, []);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function submit() {
    if (admitted.current) return;
    const validated = serializeManualActivityForm(form, now());
    if (!validated.ok) { setError(validated.reason === 'future_end' ? 'future_end' : 'invalid'); return; }
    let review: ActivityWrite;
    try {
      const input = { localDate: validated.fields.localDate, localTime: validated.fields.localTime, seconds: validated.fields.durationSeconds, note };
      const fingerprint = JSON.stringify(input);
      if (submission.current?.fingerprint !== fingerprint) submission.current = { fingerprint, review: reviewActivityWrite(pathId, detail?.id ?? null, input, d.operationId()) };
      review = submission.current.review;
    } catch (cause) { setError(cause instanceof ActivityInputInvalid && cause.field === 'note' ? 'note' : 'invalid'); return; }
    admitted.current = true; setBusy(true); setError(null);
    try {
      const result = await saveReviewedActivity(d.activities, review);
      if (!active.current) return;
      await client.cancelQueries({ queryKey: [d.accountScope] });
      if (!active.current) return;
      client.removeQueries({ predicate: query => query.queryKey[0] === d.accountScope && ['tracking', 'history', 'activity', 'activity-revisions', 'statistics', 'path-statistics', 'path-history', 'social'].includes(String(query.queryKey[1])) });
      setDirty(false); admitted.current = false; setSaved(result.id);
    } catch (cause) { if (active.current) { setError('request'); setRetryable(!(cause instanceof ActivityFailure) || cause.retryable); } }
    finally { admitted.current = false; if (active.current) setBusy(false); }
  }
  if (saved) return <Navigate to="/paths/$pathId/activities/$activityId" params={{ pathId, activityId: saved }} search={{ activitySaved: true }} />;
  const changed = () => { setDirty(true); setError(null); setRetryable(true); };
  return <>
    <UnsavedChanges dirty={dirty && !busy} i18n={d.i18n} />
    <form className="studio-form studio-activity-form" onSubmit={event => { event.preventDefault(); void submit(); }}>
      <h2>{defaults.pathName}</h2><p>{d.i18n.t('activity.timeZone', { timeZone: zone })}</p>
      <fieldset disabled={busy}>
        <ActivityTimingFields i18n={d.i18n} form={form} parts={parts}
          onOccurrence={patch => { changed(); setForm(overrideManualActivityOccurrence(form, patch)); }}
          onParts={next => { changed(); setParts(next); setForm(updateManualActivityDuration(form, secondsFromDurationParts(next), now())); }} />
        <details open={Boolean(detail?.note)}><summary>{d.i18n.t('activity.note')}</summary><label>{d.i18n.t('activity.note')}<textarea rows={3} value={note} onChange={event => { changed(); setNote(event.target.value); }} /></label><small>{d.i18n.t('activity.notePrivacy')}</small></details>
        <div className="studio-form-actions">{busy ? <button type="button" disabled>{d.i18n.t('common.cancel')}</button> : <Link to="/">{d.i18n.t('common.cancel')}</Link>}<button type="submit" disabled={busy || (error === 'request' && !retryable)}>{d.i18n.t(busy ? 'activity.saving' : error === 'request' ? 'common.retry' : detail ? 'activity.saveEdit' : 'activity.save')}</button></div>
      </fieldset>
      {error && <p role="alert">{d.i18n.t(error === 'request' ? 'errors.apiRejected' : error === 'future_end' ? 'activity.futureEnd' : error === 'note' ? 'activity.noteInvalid' : 'activity.invalid')}</p>}
      {busy && <p role="status">{d.i18n.t('activity.saving')}</p>}
    </form>
  </>;
}
