import { useEffect, useRef, useState } from 'react';
import { Link, useBlocker, useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { audiences, type Audience, type AudiencePreference } from '../nudges/domain/nudge';
import { StudioShell } from './studio-shell';
import { useOwnedOperation } from './use-owned-operation';
export function NudgeAudiencePage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId } = useParams({ strict: false }) as { pathId: string };
  return <NudgeAudience key={pathId} dependencies={d} pathId={pathId} />;
}
function NudgeAudience({ dependencies: d, pathId }: { dependencies: StudioDependencies; pathId: string }) {
  const path = useQuery({ queryKey: [d.accountScope, 'nudgePath', pathId], queryFn: ({ signal }) => d.paths.read(pathId, signal), staleTime: 0, gcTime: 0, refetchOnReconnect: false });
  const preference = useQuery({ queryKey: [d.accountScope, 'nudgeAudience', pathId], queryFn: ({ signal }) => d.nudges.audience(pathId, signal), staleTime: 0, gcTime: 0, refetchOnReconnect: false });
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-main studio-activity-detail"><header className="studio-header"><div><Link to="/paths/$pathId" params={{ pathId }}>{d.i18n.t('studio.path')}</Link><h1>{d.i18n.t('nudge.audience.heading')}</h1>{path.data && <p>{path.data.name}</p>}</div></header>
    {path.isPending || preference.isPending ? <p role="status">{d.i18n.t('common.loading')}</p> : path.isError || !path.data || !preference.data ? <div role="alert"><p>{d.i18n.t('nudge.audience.loadError')}</p><button onClick={() => { void path.refetch(); void preference.refetch(); }}>{d.i18n.t('common.retry')}</button></div> : <AudienceForm key={preference.data.userId} dependencies={d} initial={preference.data} refresh={async () => { const result = await preference.refetch(); if (result.isError || !result.data) throw new Error('preference_refresh_failed'); return result.data; }} />}
  </main></StudioShell>;
}
function AudienceForm({ dependencies: d, initial, refresh }: { dependencies: StudioDependencies; initial: AudiencePreference; refresh(): Promise<AudiencePreference> }) {
  const [current, setCurrent] = useState(initial), [chosen, setChosen] = useState<Audience>(initial.audience), [busy, setBusy] = useState(false), [error, setError] = useState(false), [conflict, setConflict] = useState(false), [saved, setSaved] = useState(false);
  const [commands] = useState(() => d.nudges.commands(d.operationId));
  const admitted = useRef(false), operation = useOwnedOperation(d);
  useEffect(() => () => commands.dispose(), [commands]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function recover() {
    if (admitted.current) return;
    admitted.current = true; setBusy(true);
    try {
      const value = await refresh();
      if (!operation.active()) return;
      setCurrent(value); setChosen(value.audience); setConflict(false); setError(false); setSaved(false);
    } catch { if (operation.active()) setError(true); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  async function save() {
    if (admitted.current || conflict || chosen === current.audience) return;
    admitted.current = true; setBusy(true); setError(false); setSaved(false);
    try {
      const result = await operation.run(signal => commands.save(current, chosen, signal));
      if (!operation.active()) return;
      if (result.kind === 'applied') { setCurrent(result.value); setChosen(result.value.audience); setSaved(true); }
      else if (result.kind === 'failed') { setError(true); setConflict(result.conflict); }
    } catch { if (operation.active()) setError(true); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  return <section className="studio-settings-card"><fieldset disabled={busy || conflict} className="studio-choice-group"><legend>{d.i18n.t('nudge.audience.label')}</legend>{audiences.map(value => <label key={value}><input type="radio" name="nudge-audience" checked={chosen === value} onChange={() => { setChosen(value); setSaved(false); }} />{d.i18n.t(`nudge.audience.${value}`)}</label>)}</fieldset><p>{d.i18n.t('nudge.audience.footer')}</p>
    {error && <p role="alert">{d.i18n.t(conflict ? 'studio.nudgeAudienceConflict' : 'studio.nudgeAudienceSaveFailed')}</p>}{conflict && <button disabled={busy} onClick={() => void recover()}>{d.i18n.t('common.refresh')}</button>}
    {busy && <p role="status">{d.i18n.t('nudge.audience.saving')}</p>}{saved && <p role="status">{d.i18n.t('nudge.audience.saved')}</p>}
    <button disabled={busy || conflict || chosen === current.audience} onClick={() => void save()}>{d.i18n.t(error ? 'common.retry' : 'common.save')}</button>
  </section>;
}
