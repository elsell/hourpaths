import { useEffect, useRef, useState, useId } from 'react';
import { useBlocker } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { LifecycleReview } from '../paths/application/lifecycle';
import { executeLifecycle } from '../paths/application/lifecycle';

export function PathLifecycleReview({ review, dependencies: d, close }: { review: LifecycleReview; dependencies: StudioDependencies; close(): void }) {
  const client = useQueryClient();
  const dialog = useRef<HTMLDialogElement>(null);
  const active = useRef(true);
  const admitted = useRef(false);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const heading = useId();
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  useEffect(() => { dialog.current?.showModal(); return () => { active.current = false; }; }, []);
  const deleting = review.action === 'delete';
  async function submit() {
    if (admitted.current) return;
    admitted.current = true;
    setBusy(true);
    setFailed(false);
    try {
      await executeLifecycle(d.paths, review);
      if (!active.current) return;
      // Cancel older reads and clear their projections before authoritative refetch.
      await client.cancelQueries({ queryKey: [d.accountScope] });
      if (!active.current) return;
      await client.resetQueries({ queryKey: [d.accountScope] });
      if (active.current) close();
    } catch {
      if (active.current) setFailed(true);
    } finally {
      admitted.current = false;
      if (active.current) setBusy(false);
    }
  }
  return <dialog ref={dialog} className="studio-settings-guard" aria-labelledby={heading} onCancel={event => { event.preventDefault(); if (!admitted.current) close(); }}>
    <div><h2 id={heading}>{deleting ? d.i18n.t('pathDelete.heading', { pathName: review.name }) : d.i18n.t('pathArchive.heading')}</h2>
      {!deleting && <p>{review.name}</p>}
      <p>{d.i18n.t(deleting ? 'pathDelete.warning' : review.action === 'archive' ? 'pathArchive.warning' : 'pathArchive.unarchiveWarning')}</p>
      {deleting && <><p>{d.i18n.t('pathDelete.timerWarning')}</p><p>{d.i18n.t('pathDelete.archiveAlternative')}</p></>}
      {failed && <p role="alert">{d.i18n.t('errors.apiRejected')}</p>}
      <div className="studio-form-actions"><button autoFocus disabled={busy} onClick={close}>{d.i18n.t('common.cancel')}</button><button disabled={busy} onClick={() => void submit()}>{d.i18n.t(deleting ? 'pathDelete.confirm' : review.action === 'archive' ? 'pathArchive.confirm' : 'pathArchive.confirmUnarchive')}</button></div>
      {busy && <p role="status">{d.i18n.t('studio.loading')}</p>}
    </div>
  </dialog>;
}
