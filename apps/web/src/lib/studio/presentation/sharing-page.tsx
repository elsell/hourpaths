import { useEffect, useRef, useState } from 'react';
import { Link, useBlocker, useParams } from '@tanstack/react-router';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import { sharingCommands, pathInvitationFailureMessageKey } from '../sharing/application/sharing';
import type { ManagedPendingPathInvitation, PathInvitationRecipientReview, PathInvitationRole } from '../sharing/domain/invitations';
import type { StudioDependencies } from './app';
import { StudioShell } from './studio-shell';
import { ConfirmationDialog } from './confirmation-dialog';
import { UnsavedChanges } from './unsaved-changes';
export function SharingPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId } = useParams({ strict: false }) as { pathId: string };
  const context = useQuery({ queryKey: [d.accountScope, 'sharingContext', pathId], queryFn: ({ signal }) => d.sharing.context(pathId, signal), refetchOnWindowFocus: true, refetchInterval: 30_000 });
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-main studio-activity-detail"><Link to="/">{d.i18n.t('common.back')}</Link><header className="studio-header"><h1>{d.i18n.t('pathInvitation.heading')}</h1></header>
    {context.isPending ? <p role="status">{d.i18n.t('common.loading')}</p> : context.isError ? <div role="alert"><p>{d.i18n.t('pathInvitation.unavailable')}</p><button onClick={() => void context.refetch()}>{d.i18n.t('common.retry')}</button></div> : <SharingForm key={pathId} pathId={pathId} name={context.data.name} dependencies={d} />}
  </main></StudioShell>;
}
function SharingForm({ pathId, name, dependencies: d }: { pathId: string; name: string; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const [commands] = useState(() => sharingCommands(d.sharing, d.operationId));
  const active = useRef(true), admitted = useRef(false);
  const [username, setUsername] = useState(''), [role, setRole] = useState<PathInvitationRole>('participant');
  const [review, setReview] = useState<PathInvitationRecipientReview | null>(null);
  const [selected, setSelected] = useState<ManagedPendingPathInvitation | null>(null);
  const [busy, setBusy] = useState(false), [reviewing, setReviewing] = useState(false);
  const [failure, setFailure] = useState<string | null>(null), [notice, setNotice] = useState('');
  const key = [d.accountScope, 'managedInvitations', pathId];
  const pending = useInfiniteQuery({ queryKey: key, initialPageParam: '', queryFn: ({ pageParam, signal }) => d.sharing.pending(pathId, pageParam, signal), getNextPageParam: (page, pages, last, previous) => page.nextCursor && !previous.includes(page.nextCursor) ? page.nextCursor : undefined });
  const items = [...new Map(pending.data?.pages.flatMap(page => page.items).map(item => [item.invitation.id, item]) ?? []).values()];
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  useEffect(() => () => { active.current = false; commands.dispose(); }, [commands]);
  async function lookup() {
    if (reviewing || admitted.current) return;
    setReviewing(true); setFailure(null); setReview(null);
    const result = await commands.review(pathId, username.trim());
    if (!active.current) return;
    setReviewing(false);
    if (result.kind === 'reviewed') setReview(result.review);
    else if (result.kind === 'failed') setFailure(d.i18n.t(pathInvitationFailureMessageKey(result.failure)));
  }
  async function mutate(action: 'send' | 'cancel') {
    if (admitted.current || (action === 'send' ? !review : !selected)) return;
    admitted.current = true; setBusy(true); setFailure(null);
    try {
      const result = action === 'send' ? await commands.send(review!, role) : await commands.cancel(pathId, selected!.invitation.id);
      if (!active.current) return;
      if (result.kind === 'failed') { setFailure(d.i18n.t(pathInvitationFailureMessageKey(result.failure))); return; }
      if (result.kind === 'superseded' || result.kind === 'cancelled') return;
      setNotice(action === 'send' ? d.i18n.t('pathInvitation.sent', review!.recipient) : d.i18n.t('pathInvitation.managed.canceled'));
      if (action === 'send') { setReview(null); setUsername(''); commands.clearReview(); }
      else { setSelected(null); commands.clearCancellation(); }
      await client.cancelQueries({ queryKey: key });
      await client.resetQueries({ queryKey: key });
    } catch { if (active.current) setFailure(d.i18n.t('pathInvitation.retry')); }
    finally { admitted.current = false; if (active.current) setBusy(false); }
  }
  return <>
    <h2>{name}</h2><UnsavedChanges dirty={!busy && !!username.trim()} i18n={d.i18n} />
    {notice && <p role="status">{notice}</p>}
    <form className="studio-form studio-activity-form" onSubmit={event => { event.preventDefault(); void lookup(); }}><fieldset disabled={busy || reviewing}>
      <label>{d.i18n.t('pathInvitation.usernameLabel')}<input required value={username} autoComplete="off" onChange={event => { commands.clearReview(); setUsername(event.target.value); setReview(null); setFailure(null); }} /></label>
      <p>{d.i18n.t('pathInvitation.usernameHint')}</p>
      <label>{d.i18n.t('pathInvitation.roleLabel')}<select value={role} onChange={event => setRole(event.target.value as PathInvitationRole)}><option value="participant">{d.i18n.t('pathInvitation.role.participant')}</option><option value="supporter">{d.i18n.t('pathInvitation.role.supporter')}</option></select></label>
      <p>{d.i18n.t(role === 'participant' ? 'pathInvitation.role.participantEffect' : 'pathInvitation.role.supporterEffect')}</p>
      <button type="submit" disabled={!username.trim()}>{d.i18n.t(reviewing ? 'pathInvitation.reviewing' : 'pathInvitation.review')}</button>
    </fieldset>
    {review && <section className="studio-settings-review"><h3>{d.i18n.t('pathInvitation.confirmHeading')}</h3><p>{d.i18n.t('pathInvitation.confirmSend', { ...review.recipient, role: d.i18n.t(`pathInvitation.role.${role}`) })}</p><button type="button" className="studio-primary" disabled={busy} onClick={() => void mutate('send')}>{d.i18n.t(busy ? 'pathInvitation.sending' : 'pathInvitation.send')}</button></section>}
    {failure && !selected && <p role="alert">{failure}</p>}
    </form>
    <section className="studio-settings-card"><header className="studio-section-header"><h2>{d.i18n.t('pathInvitation.managed.heading')}</h2><button disabled={busy || pending.isFetching} onClick={() => void pending.refetch()}>{d.i18n.t('common.refresh')}</button></header>
      {pending.isPending ? <p role="status">{d.i18n.t('pathInvitation.managed.loading')}</p> : pending.isError ? <div role="alert"><p>{d.i18n.t('pathInvitation.managed.unavailableHeading')}</p><button disabled={busy} onClick={() => void pending.refetch()}>{d.i18n.t('common.retry')}</button></div> : items.length ? <ul className="studio-settings-people">{items.map(item => <li key={item.invitation.id}><div><strong>{d.i18n.t('pathInvitation.managed.recipient', item.recipient)}</strong><small>{d.i18n.t('pathInvitation.managed.role', { role: d.i18n.t(`pathInvitation.role.${item.invitation.offeredRole}`) })}</small><small>{d.i18n.t('pathInvitation.managed.inviter', item.inviter)}</small><small>{d.i18n.date(Date.parse(item.invitation.createdAt), { dateStyle: 'medium', timeStyle: 'short' })}</small></div><button disabled={busy} onClick={() => { setFailure(null); setSelected(item); }}>{d.i18n.t('pathInvitation.managed.cancel')}</button></li>)}</ul> : <p>{d.i18n.t('pathInvitation.managed.emptyDescription')}</p>}
      {pending.hasNextPage && !pending.isError && <button disabled={busy || pending.isFetching} onClick={() => void pending.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
    </section>
    {selected && <ConfirmationDialog title={d.i18n.t('pathInvitation.managed.cancelConfirmationHeading')} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(busy ? 'pathInvitation.managed.canceling' : 'pathInvitation.managed.cancel')} cancel={() => { commands.clearCancellation(); setSelected(null); setFailure(null); }} confirm={() => void mutate('cancel')}><p>{d.i18n.t('pathInvitation.managed.cancelConfirmationBody', { ...selected.recipient, role: d.i18n.t(`pathInvitation.role.${selected.invitation.offeredRole}`) })}</p>{failure && <p role="alert">{failure}</p>}</ConfirmationDialog>}
  </>;
}
