import { useEffect, useRef, useState } from 'react';
import { Link, useBlocker, useParams } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { Visibility, VisibilityContext, VisibilityReview } from '../paths/domain/visibility';
import type { VisibilityCommands } from '../paths/ports/visibility-commands';
import type { Path } from '../paths/domain/path';
import { chooseVisibility } from '../paths/application/visibility';
import { StudioShell } from './studio-shell';
import { ConfirmationDialog } from './confirmation-dialog';
import { UnsavedChanges } from './unsaved-changes';
import { useOwnedOperation } from './use-owned-operation';

export function PathVisibilityPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId } = useParams({ strict: false }) as { pathId: string };
  return <VisibilityPage key={pathId} pathId={pathId} dependencies={d} />;
}
function VisibilityPage({ pathId, dependencies: d }: { pathId: string; dependencies: StudioDependencies }) {
  const [commands] = useState(() => d.paths.visibilityCommands(d.operationId));
  const [identity] = useState(d.operationId);
  const [generation, setGeneration] = useState(0);
  useEffect(() => () => commands.dispose(), [commands]);
  const query = useQuery({ queryKey: [d.accountScope, 'pathVisibility', pathId, identity], queryFn: ({ signal }) => commands.load(pathId, signal),
    staleTime: Infinity, gcTime: 0, structuralSharing: false, retry: false, refetchOnReconnect: false, refetchOnWindowFocus: false });
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-main studio-activity-detail"><header className="studio-header"><div><Link to="/paths/$pathId" params={{ pathId }}>{d.i18n.t('studio.path')}</Link><h1>{d.i18n.t('pathVisibility.heading')}</h1></div></header>
    {query.isPending ? <p role="status">{d.i18n.t('common.loading')}</p> : !query.data ? <div role="alert"><p>{d.i18n.t('pathDetails.unavailableTitle')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> :
      <VisibilityForm key={generation} initial={query.data} commands={commands} dependencies={d} refresh={async () => { const result = await query.refetch(); if (result.isError || !result.data) throw new Error('visibility_refresh_failed'); setGeneration(value => value + 1); }} />}
  </main></StudioShell>;
}
function VisibilityForm({ initial, commands, dependencies: d, refresh }: { initial: VisibilityContext; commands: VisibilityCommands; dependencies: StudioDependencies; refresh(): Promise<void> }) {
  const [context, setContext] = useState(initial), [choice, setChoice] = useState<Visibility>(initial.path.visibility);
  const [review, setReview] = useState<VisibilityReview | null>(null), [retry, setRetry] = useState<VisibilityReview | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(false), [requiresReview, setRequiresReview] = useState(false), [saved, setSaved] = useState(false);
  const admitted = useRef(false), operation = useOwnedOperation(d), client = useQueryClient();
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  const label = (value: Visibility) => d.i18n.t(`pathVisibility.option.${value}`);
  async function recover() {
    if (admitted.current) return;
    admitted.current = true; setBusy(true); setRequiresReview(true); setReview(null);
    try { await refresh(); }
    catch { if (operation.active()) setError(true); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  async function save(value: VisibilityReview) {
    if (admitted.current || requiresReview) return;
    admitted.current = true; setBusy(true); setError(false); setSaved(false);
    try {
      const result = await operation.run(signal => commands.submit(value, signal));
      if (!operation.active()) return;
      if (result.kind === 'applied') {
        setContext(result.value); setChoice(result.value.path.visibility); setRetry(null); setReview(null); setSaved(true);
        client.setQueryData([d.accountScope, 'path', value.pathId], result.value.path);
        client.setQueriesData<readonly Path[]>({ queryKey: [d.accountScope, 'paths'] }, previous => previous?.map(path => path.id === value.pathId ? result.value.path : path));
        void client.invalidateQueries({ predicate: query => query.queryKey[0] === d.accountScope && !['pathVisibility', 'ownedOperation'].includes(String(query.queryKey[1])) });
        d.notifications.invalidate();
      } else if (result.kind === 'failed') { setError(true); setRequiresReview(result.requiresReview); setRetry(result.requiresReview ? null : value); setReview(null); }
    } catch { if (operation.active()) { setError(true); setRetry(value); setReview(null); } }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  function submit() {
    if (admitted.current || requiresReview) return;
    if (retry) { void save(retry); return; }
    try {
      const decision = chooseVisibility(commands, context, choice);
      if (decision.kind === 'confirm') setReview(decision.review);
      else if (decision.kind === 'submit') void save(decision.review);
    } catch { setError(true); setRequiresReview(true); }
  }
  if (!context.path.canManageVisibility || context.path.archived) return <p role="alert">{d.i18n.t('pathDetails.unavailableTitle')}</p>;
  return <section className="studio-settings-card"><UnsavedChanges dirty={!busy && choice !== context.path.visibility} i18n={d.i18n} /><h2>{context.path.name}</h2><p>{d.i18n.t('pathVisibility.current', { visibility: label(context.path.visibility) })}</p>
    <form onSubmit={event => { event.preventDefault(); submit(); }}><fieldset className="studio-choice-group" disabled={busy || requiresReview || !!retry}><legend>{d.i18n.t('pathVisibility.choiceLabel')}</legend>{context.options.map(value => <label key={value}><input type="radio" name="path-visibility" value={value} checked={choice === value} onChange={() => { setChoice(value); setSaved(false); setError(false); }} />{label(value)}</label>)}</fieldset>
      <div className="studio-form-actions"><button type="button" disabled={busy || !!retry || requiresReview} onClick={() => { setChoice(context.path.visibility); setError(false); }}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={busy || requiresReview || (!retry && choice === context.path.visibility)}>{d.i18n.t(retry ? 'common.retry' : 'pathVisibility.save')}</button></div>
    </form>
    {busy && <p role="status">{d.i18n.t('pathVisibility.saving')}</p>}{saved && <p role="status">{d.i18n.t('pathVisibility.saved')}</p>}
    {error && <div role="alert"><p>{d.i18n.t(requiresReview ? 'studio.settings.conflict' : 'studio.pathVisibilityUnconfirmed')}</p><button disabled={busy} onClick={() => void recover()}>{d.i18n.t('common.refresh')}</button></div>}
    {review && <ConfirmationDialog title={d.i18n.t('pathVisibility.confirmation.heading')} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t('pathVisibility.confirmation.confirm')} cancel={() => setReview(null)} confirm={() => void save(review)}>
      <p>{d.i18n.t('pathVisibility.confirmation.transition', { pathName: review.name, current: label(review.current), proposed: label(review.proposed) })}</p><p>{d.i18n.t('pathVisibility.confirmation.historyExposure')}</p><p>{d.i18n.t('pathVisibility.confirmation.unchangedScope')}</p>
    </ConfirmationDialog>}
  </section>;
}
