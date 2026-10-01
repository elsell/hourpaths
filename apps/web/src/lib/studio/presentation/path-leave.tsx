import { useEffect, useRef, useState } from 'react';
import { Navigate, useBlocker } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { LeaveReview } from '../paths/domain/leave';
import { ConfirmationDialog } from './confirmation-dialog';
import { useOwnedOperation } from './use-owned-operation';
export function PathLeave({ pathId, dependencies: d, close }: { pathId: string; dependencies: StudioDependencies; close(): void }) {
  const client = useQueryClient(), operation = useOwnedOperation(d);
  const [commands] = useState(() => d.paths.leaveCommands(d.operationId));
  const [review, setReview] = useState<LeaveReview | null>(null), [loading, setLoading] = useState(true), [deleting, setDeleting] = useState(false), [busy, setBusy] = useState(false), [failed, setFailed] = useState(false), [attempted, setAttempted] = useState(false), [departed, setDeparted] = useState(false);
  const admitted = useRef(false);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function load() {
    setLoading(true); setFailed(false);
    try { const value = await operation.run(signal => commands.review(pathId, signal)); if (operation.active()) { setReview(value); setAttempted(value.retainActivity !== undefined); setDeleting(value.retainActivity === false); } }
    catch { if (operation.active()) setFailed(true); }
    finally { if (operation.active()) setLoading(false); }
  }
  useEffect(() => { void load(); return () => commands.dispose(); }, [commands]);
  async function leave() {
    if (!review || admitted.current) return;
    admitted.current = true; setBusy(true); setFailed(false); setAttempted(true);
    try {
      const result = await operation.run(signal => commands.submit(review, !deleting, signal));
      if (!operation.active()) return;
      if (result.kind === 'failed') { setFailed(true); return; }
      if (result.kind !== 'applied') return;
      await client.cancelQueries({ predicate: query => query.queryKey[0] === d.accountScope && query.queryKey[1] !== 'ownedOperation' });
      if (!operation.active()) return;
      client.removeQueries({ predicate: query => query.queryKey[0] === d.accountScope && query.queryKey[1] !== 'ownedOperation' });
      void d.notifications.invalidate();
      setDeparted(true);
    } catch { if (operation.active()) setFailed(true); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  if (departed) return <Navigate to="/" search={{ pathLeft: true }} replace />;
  return <ConfirmationDialog title={d.i18n.t(deleting ? 'pathLeave.deleteHeading' : 'pathLeave.heading', { pathName: review?.name ?? d.i18n.t('studio.path') })} busy={busy} confirmDisabled={loading || !review} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(failed && review ? 'common.retry' : deleting ? 'pathLeave.confirmDelete' : review?.participant ? 'pathLeave.keepActivity' : 'pathLeave.confirm')} cancel={close} confirm={() => void leave()}>
    {loading ? <p role="status">{d.i18n.t('common.loading')}</p> : !review ? <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={() => void load()}>{d.i18n.t('common.retry')}</button></div> : <>
      {deleting ? <><p>{d.i18n.t('pathLeave.deleteWarning')}</p><p>{d.i18n.t('studio.leaveDeleteConsequences')}</p><p>{d.i18n.t('studio.leaveDiscardTimer')}</p></> : <><p>{d.i18n.t(review.participant ? 'pathLeave.warning' : 'pathLeave.supporterWarning')}</p>{review.participant && <><p>{d.i18n.t('studio.leaveSaveTimer')}</p>{!attempted && <button disabled={busy} onClick={() => setDeleting(true)}>{d.i18n.t('pathLeave.deleteActivity')}</button>}</>}</>}
      {deleting && !attempted && <button disabled={busy} onClick={() => setDeleting(false)}>{d.i18n.t('common.back')}</button>}
      {failed && <p role="alert">{d.i18n.t('errors.apiRejected')}</p>}
      {busy && <p role="status">{d.i18n.t('pathLeave.leaving')}</p>}
    </>}
  </ConfirmationDialog>;
}
