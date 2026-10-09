import { createWeekStartOperationOwner, WeekStartFailure } from '@hourpaths/client-core';
import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { WeekStartPreference } from '../preferences/domain/preferences';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';
import { UnsavedChanges } from './unsaved-changes';

export function WeekStartSettings({ owner, dependencies: d }: { owner: string; dependencies: StudioDependencies }) {
  const query = useQuery({ queryKey: [d.accountScope, 'preferences', 'week-start'], queryFn: ({ signal }) => d.preferences.weekStart(owner, signal) });
  return <section className="studio-settings-card"><h3>{d.i18n.t('settings.weekStart.heading')}</h3><p>{d.i18n.t('settings.weekStart.explanation')}</p>
    {query.data ? <WeekStartForm initial={query.data} dependencies={d} /> : query.isError ? <p role="alert">{d.i18n.t('settings.weekStart.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p> : <p role="status">{d.i18n.t('common.loading')}</p>}
  </section>;
}
function WeekStartForm({ initial, dependencies: d }: { initial: WeekStartPreference; dependencies: StudioDependencies }) {
  const [saved, setSaved] = useState(initial), [draft, setDraft] = useState(initial.firstDayOfWeek);
  const [busy, setBusy] = useState(false), [error, setError] = useState<'conflict' | 'failed' | null>(null), [success, setSuccess] = useState(false);
  const [owner] = useState(() => createWeekStartOperationOwner(initial.userId, d.operationId));
  const operation = useOwnedOperation(d), cache = useQueryClient(), admitted = useRef(false);
  useEffect(() => () => owner.cancel(), [owner]);
  const weekdayLabel = (day: number) => d.i18n.date(Date.UTC(2026, 0, 4 + day), {
    weekday: 'long', timeZone: 'UTC',
  });
  const dirty = saved.firstDayOfWeek !== draft;
  useEffect(() => { if (!busy && !dirty) { setSaved(initial); setDraft(initial.firstDayOfWeek); } }, [initial, busy, dirty]);
  async function refresh() {
    if (admitted.current) return; admitted.current = true; setBusy(true);
    try {
      const value = await operation.run(signal => d.preferences.weekStart(initial.userId, signal));
      if (!operation.active()) return;
      owner.cancel(); setSaved(value); setDraft(value.firstDayOfWeek); setError(null); setSuccess(false);
      cache.setQueryData([d.accountScope, 'preferences', 'week-start'], value);
    } catch { if (operation.active()) setError('failed'); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  async function save() {
    if (admitted.current || !dirty || error === 'conflict') return;
    admitted.current = true; setBusy(true); setError(null); setSuccess(false);
    const result = await owner.submit({ userId: saved.userId, reviewedFirstDayOfWeek: saved.firstDayOfWeek, proposedFirstDayOfWeek: draft }, (value, key) => operation.run(signal => d.preferences.saveWeekStart(value, key, signal)));
    if (operation.active()) {
      if (result.kind === 'applied') {
        setSaved(result.preference); setDraft(result.preference.firstDayOfWeek); setSuccess(true);
        cache.setQueryData([d.accountScope, 'preferences', 'week-start'], result.preference);
        void cache.invalidateQueries({ queryKey: [d.accountScope, 'statistics'] });
        void cache.invalidateQueries({ queryKey: [d.accountScope, 'preferences', 'week-start'] });
      } else if (result.kind === 'failed') setError(result.cause instanceof WeekStartFailure && result.cause.kind === 'conflict' ? 'conflict' : 'failed');
      setBusy(false);
    }
    admitted.current = false;
  }
  return <form className="studio-settings-form" onSubmit={event => { event.preventDefault(); void save(); }}>
    <UnsavedChanges dirty={dirty || busy} i18n={d.i18n} />
    <fieldset disabled={busy}><label>{d.i18n.t('settings.weekStart.heading')}<select value={draft} onChange={event => { setDraft(Number(event.target.value)); setSuccess(false); }}>{Array.from({ length: 7 }, (_, index) => index + 1).map(day => <option key={day} value={day}>{weekdayLabel(day)}</option>)}</select></label>
      <div className="studio-settings-actions"><button type="button" disabled={!dirty} onClick={() => { owner.cancel(); setDraft(saved.firstDayOfWeek); setSuccess(false); }}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={!dirty || error === 'conflict'}>{d.i18n.t(busy ? 'settings.weekStart.saving' : 'common.save')}</button></div>
    </fieldset>
    {error && <p role="alert">{d.i18n.t(error === 'conflict' ? 'settings.weekStart.conflict' : 'settings.weekStart.saveFailed')}</p>}
    {error === 'conflict' && <button type="button" disabled={busy} onClick={() => void refresh()}>{d.i18n.t('common.refresh')}</button>}
    {success && <p role="status">{d.i18n.t('settings.weekStart.saved')}</p>}
  </form>;
}
