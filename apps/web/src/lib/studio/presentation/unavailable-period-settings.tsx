import { createUnavailablePeriodOwner, UnavailablePeriodFailure } from '@hourpaths/client-core';
import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { UnavailablePeriodPreference } from '../preferences/domain/preferences';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';
import { UnsavedChanges } from './unsaved-changes';

export function UnavailablePeriodSettings({ owner, dependencies: d }: { owner: string; dependencies: StudioDependencies }) {
  const query = useQuery({ queryKey: [d.accountScope, 'preferences', 'unavailable-period'], queryFn: ({ signal }) => d.preferences.unavailablePeriod(owner, signal) });
  return <section className="studio-settings-card"><h3>{d.i18n.t('settings.quietHours.heading')}</h3><p>{d.i18n.t('settings.quietHours.explanation')}</p>
    {query.data ? <UnavailablePeriodForm initial={query.data} dependencies={d} /> : query.isError ? <p role="alert">{d.i18n.t('settings.quietHours.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p> : <p role="status">{d.i18n.t('common.loading')}</p>}
  </section>;
}
function UnavailablePeriodForm({ initial, dependencies: d }: { initial: UnavailablePeriodPreference; dependencies: StudioDependencies }) {
  const [saved, setSaved] = useState(initial), [draft, setDraft] = useState(initial);
  const [busy, setBusy] = useState(false), [error, setError] = useState<'conflict' | 'failed' | null>(null), [success, setSuccess] = useState(false);
  const [owner] = useState(() => createUnavailablePeriodOwner(initial.userId, d.operationId));
  const operation = useOwnedOperation(d), cache = useQueryClient(), admitted = useRef(false);
  useEffect(() => () => owner.cancel(), [owner]);
  const dirty = saved.enabled !== draft.enabled || saved.startMinute !== draft.startMinute || saved.endMinute !== draft.endMinute;
  const timeValue = (minute: number) => `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`;
  const parseTime = (value: string) => { const [hour, minute] = value.split(':').map(Number); return hour * 60 + minute; };
  useEffect(() => { if (!busy && !dirty) { setSaved(initial); setDraft(initial); } }, [initial, busy, dirty]);
  async function refresh() {
    if (admitted.current) return; admitted.current = true; setBusy(true);
    try {
      const value = await operation.run(signal => d.preferences.unavailablePeriod(initial.userId, signal));
      if (!operation.active()) return;
      owner.cancel(); setSaved(value); setDraft(value); setError(null); setSuccess(false);
      cache.setQueryData([d.accountScope, 'preferences', 'unavailable-period'], value);
    } catch { if (operation.active()) setError('failed'); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  async function save() {
    if (admitted.current || !dirty || error === 'conflict') return;
    admitted.current = true; setBusy(true); setError(null); setSuccess(false);
    const result = await owner.submit({ userId: saved.userId, enabled: draft.enabled, startMinute: draft.startMinute, endMinute: draft.endMinute, expectedRevision: saved.revision, reviewedTimeZone: saved.timeZone }, (value, key) => operation.run(signal => d.preferences.saveUnavailablePeriod(value, key, signal)));
    if (operation.active()) {
      if (result.kind === 'applied') {
        setSaved(result.preference); setDraft(result.preference); setSuccess(true);
        cache.setQueryData([d.accountScope, 'preferences', 'unavailable-period'], result.preference);
        void cache.invalidateQueries({ queryKey: [d.accountScope, 'preferences', 'unavailable-period'] });
      } else if (result.kind === 'failed') setError(result.cause instanceof UnavailablePeriodFailure && result.cause.kind === 'conflict' ? 'conflict' : 'failed');
      setBusy(false);
    }
    admitted.current = false;
  }
  return <form className="studio-settings-form" onSubmit={event => { event.preventDefault(); void save(); }}>
    <UnsavedChanges dirty={dirty || busy} i18n={d.i18n} />
    <fieldset disabled={busy}>
      <label className="studio-settings-toggle"><span>{d.i18n.t('settings.quietHours.enabled')}</span><input type="checkbox" checked={draft.enabled} onChange={event => { setDraft({ ...draft, enabled: event.target.checked }); setSuccess(false); }} /></label>
      {draft.enabled && <><label>{d.i18n.t('settings.quietHours.start')}<input type="time" required value={timeValue(draft.startMinute)} onChange={event => { if (event.target.value) setDraft({ ...draft, startMinute: parseTime(event.target.value) }); setSuccess(false); }} /></label>
      <label>{d.i18n.t('settings.quietHours.end')}<input type="time" required value={timeValue(draft.endMinute)} onChange={event => { if (event.target.value) setDraft({ ...draft, endMinute: parseTime(event.target.value) }); setSuccess(false); }} /></label></>}
      <p>{d.i18n.t('settings.quietHours.zone', { zone: saved.timeZone })}</p>
      <div className="studio-settings-actions"><button type="button" disabled={!dirty} onClick={() => { owner.cancel(); setDraft(saved); setSuccess(false); }}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={!dirty || error === 'conflict'}>{d.i18n.t(busy ? 'settings.quietHours.saving' : 'common.save')}</button></div>
    </fieldset>
    {error && <p role="alert">{d.i18n.t(error === 'conflict' ? 'settings.quietHours.conflict' : 'settings.quietHours.saveFailed')}</p>}
    {error === 'conflict' && <button type="button" disabled={busy} onClick={() => void refresh()}>{d.i18n.t('common.refresh')}</button>}
    {success && <p role="status">{d.i18n.t('settings.quietHours.saved')}</p>}
  </form>;
}
