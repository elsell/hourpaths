import { useEffect, useRef, useState } from 'react';
import { useIsMutating, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';

export function SessionSettings({ dependencies: d, name }: { dependencies: StudioDependencies; name: string }) {
  const [open, setOpen] = useState(false);
  const mutations = useIsMutating();
  return <section className="studio-settings-card"><h3>{d.i18n.t('studio.session.heading')}</h3><p>{d.i18n.t('settings.account.footer')}</p><button disabled={mutations > 0} onClick={() => setOpen(true)}>{d.i18n.t('studio.session.signOut')}</button>{open && <SignOutDialog dependencies={d} name={name} close={() => setOpen(false)} />}</section>;
}
function SignOutDialog({ dependencies: d, name, close }: { dependencies: StudioDependencies; name: string; close(): void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const operation = useOwnedOperation(d), client = useQueryClient();
  const [reviewKey] = useState(() => [d.accountScope, 'signOutReview', d.operationId()]);
  const query = useQuery({ queryKey: reviewKey, queryFn: ({ signal }) => d.session.review(signal), staleTime: 0, gcTime: 0 });
  const mutation = useMutation({ mutationFn: () => operation.run(signal => d.session.stopAndSave(query.data ?? [], signal)), onError: () => {
    if (!operation.active()) return;
    void client.invalidateQueries({ queryKey: [d.accountScope, 'tracking'] });
    void query.refetch();
  } });
  useEffect(() => { dialog.current?.showModal(); }, []);
  const count = query.data?.length ?? 0;
  return <dialog ref={dialog} className="studio-settings-guard studio-session-dialog" aria-labelledby="studio-sign-out-title" onCancel={event => { event.preventDefault(); if (!mutation.isPending) close(); }}>
    <div><h2 id="studio-sign-out-title">{d.i18n.t(count ? 'settings.account.activeTimers.title' : 'settings.account.signOutConfirmTitle')}</h2>
      {query.isPending || query.isFetching ? <p role="status">{d.i18n.t('studio.session.reviewing')}</p> : query.isError ? <div role="alert"><p>{d.i18n.t('studio.session.reviewFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <>
        <p>{d.i18n.t(count ? 'settings.account.activeTimers.message' : 'settings.account.signOutConfirmMessage', { count, name })}</p>
        {count > 0 && <ul>{query.data?.map(timer => <li key={timer.timerId}>{timer.pathName}</li>)}</ul>}
      </>}
      {mutation.isError && <p role="alert">{d.i18n.t('settings.account.activeTimers.stopFailed')}</p>}
      {mutation.isPending && <p role="status">{d.i18n.t('settings.account.signingOut')}</p>}
      <div className="studio-session-actions"><button disabled={mutation.isPending} onClick={close}>{d.i18n.t('common.cancel')}</button>
        {(count > 0 || query.isError || query.isPending) && <button disabled={mutation.isPending} onClick={() => { try { d.session.keepRunning(); } catch { close(); } }}>{d.i18n.t('settings.account.activeTimers.keepRunningAndSignOut')}</button>}
        {!query.isError && <button className="studio-primary" disabled={query.isPending || query.isFetching || mutation.isPending} onClick={() => mutation.mutate()}>{d.i18n.t(count ? 'settings.account.activeTimers.stopAndSignOut' : 'studio.session.signOut')}</button>}
      </div>
    </div>
  </dialog>;
}
