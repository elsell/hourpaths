import { useEffect, useId, useRef, useState } from 'react';
import { useBlocker } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import type { Translator } from '@hourpaths/i18n';
import type { AccountDeletionService } from '../account/ports/account-deletion';

export function AccountDeletionSettings({ service, i18n }: { service: AccountDeletionService; i18n: Translator }) {
  const [review, setReview] = useState(false);
  return <section className="studio-settings-card"><h3>{i18n.t('accountDelete.heading')}</h3><p>{i18n.t('accountDelete.intro')}</p><button onClick={() => setReview(true)}>{i18n.t('accountDelete.heading')}</button>{review && <DeletionDialog service={service} i18n={i18n} close={() => setReview(false)} />}</section>;
}
function DeletionDialog({ service, i18n, close }: { service: AccountDeletionService; i18n: Translator; close(): void }) {
  const dialog = useRef<HTMLDialogElement>(null), admitted = useRef(false);
  const heading = useId();
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const review = useQuery({ queryKey: ['account-deletion-review', heading], queryFn: () => service.review(), retry: false, gcTime: 0 });
  useEffect(() => { dialog.current?.showModal(); }, []);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function submit() {
    if (busy || !review.data) return;
    admitted.current = true; setConfirmed(true); setBusy(true); setFailed(false);
    try { await service.confirm(review.data.owner); }
    catch { setFailed(true); }
    finally { setBusy(false); }
  }
  return <dialog ref={dialog} className="studio-settings-guard" aria-labelledby={heading} onCancel={event => { event.preventDefault(); if (!admitted.current) close(); }}><div>
    <h2 id={heading}>{i18n.t('accountDelete.heading')}</h2>
    {review.data && <p><strong>{review.data.name}</strong></p>}
    {(['warning', 'paths', 'timers', 'retention'] as const).map(key => <p key={key}>{i18n.t(`accountDelete.${key}`)}</p>)}
    {review.isPending && <p role="status">{i18n.t('common.loading')}</p>}
    {review.isError && <div role="alert"><p>{i18n.t('accountDelete.reviewFailed')}</p><button onClick={() => void review.refetch()}>{i18n.t('common.retry')}</button></div>}
    {failed && <p role="alert">{i18n.t('accountDelete.failed')}</p>}
    {busy && <p role="status">{i18n.t('accountDelete.progress')}</p>}
    <div className="studio-form-actions">{!confirmed && <button autoFocus onClick={close}>{i18n.t('common.cancel')}</button>}<button disabled={busy || !review.data} onClick={() => void submit()}>{i18n.t(confirmed ? 'common.retry' : 'accountDelete.confirm')}</button></div>
  </div></dialog>;
}

export function AccountDeletionRecovery({ service, owners, i18n, signIn }: { service: AccountDeletionService; owners: string[]; i18n: Translator; signIn(): Promise<void> }) {
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  async function resume() {
    setBusy(true); setFailed(false);
    try { for (const owner of owners) await service.resume(owner); }
    catch { setFailed(true); }
    finally { setBusy(false); }
  }
  useEffect(() => { void resume(); }, []);
  return <main className="studio studio-entry"><section className="studio-entry-card"><h1>{i18n.t('accountDelete.recovery')}</h1><p>{i18n.t('accountDelete.recoveryDetail')}</p>
    {failed && <p role="alert">{i18n.t('accountDelete.failed')}</p>}{busy && <p role="status">{i18n.t('accountDelete.progress')}</p>}
    <button disabled={busy} onClick={() => void resume()}>{i18n.t('common.retry')}</button>{failed && <button disabled={busy} onClick={() => { void signIn().catch(() => setFailed(true)); }}>{i18n.t('auth.signIn')}</button>}
  </section></main>;
}
