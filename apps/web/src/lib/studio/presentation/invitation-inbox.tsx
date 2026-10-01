import { useEffect, useRef, useState } from 'react';
import { Link, useBlocker } from '@tanstack/react-router';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { PendingInvitation, InvitationDecision } from '../sharing/domain/inbox';
import { StudioShell } from './studio-shell';
import { ConfirmationDialog } from './confirmation-dialog';
export function InvitationInbox({ dependencies: d }: { dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const [commands] = useState(() => d.sharing.inboxCommands(d.operationId));
  const active = useRef(true), admitted = useRef(false);
  const [selected, setSelected] = useState<{ item: PendingInvitation; decision: InvitationDecision } | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [accepted, setAccepted] = useState<{ id: string; name: string } | null>(null);
  const key = [d.accountScope, 'invitationInbox'];
  const query = useInfiniteQuery({ queryKey: key, initialPageParam: '', queryFn: ({ pageParam, signal }) => d.sharing.inbox(pageParam, signal), getNextPageParam: (page, pages, last, previous) => page.nextCursor && !previous.includes(page.nextCursor) ? page.nextCursor : undefined });
  const items = [...new Map(query.data?.pages.flatMap(page => page.items).map(item => [item.invitation.id, item]) ?? []).values()];
  useEffect(() => () => { active.current = false; commands.dispose(); }, [commands]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  function close() { commands.clear(); setSelected(null); setError(''); }
  async function submit() {
    if (!selected || admitted.current) return;
    admitted.current = true; setBusy(true); setError('');
    try {
      const result = await commands.respond(selected.item, selected.decision);
      if (!active.current) return;
      if (result.kind === 'failed') { setError(d.i18n.t(commands.failureMessage(result.failure))); return; }
      if (result.kind !== 'accepted' && result.kind !== 'rejected') return;
      setNotice(d.i18n.t(result.kind === 'accepted' ? 'pathInvitation.accepted' : 'pathInvitation.rejected', result.kind === 'accepted' ? { role: d.i18n.t(`pathInvitation.role.${result.role}`) } : {}));
      setAccepted(result.kind === 'accepted' ? { id: result.pathId, name: selected.item.pathName } : null);
      close();
      await client.cancelQueries({ predicate: query => query.queryKey[0] === d.accountScope });
      if (!active.current) return;
      client.removeQueries({ predicate: query => query.queryKey[0] === d.accountScope && query.queryKey[1] !== 'invitationInbox' });
      await client.resetQueries({ queryKey: key });
    } catch { if (active.current) setError(d.i18n.t('pathInvitation.retry')); }
    finally { admitted.current = false; if (active.current) setBusy(false); }
  }
  const warning = selected?.decision === 'accept' ? selected.item.warning : undefined;
  return <StudioShell page="invitations" i18n={d.i18n}><main className="studio-main studio-activity-detail">
    <header className="studio-header"><h1>{d.i18n.t('pathInvitation.pendingHeading')}</h1><button disabled={busy || query.isFetching} onClick={() => void query.refetch()}>{d.i18n.t('common.refresh')}</button></header>
    {notice && <div role="status"><p>{notice}</p>{accepted && !busy && <Link to="/paths/$pathId" params={{ pathId: accepted.id }}>{d.i18n.t('studio.openPath', { name: accepted.name })}</Link>}</div>}
    {error && !selected && <p role="alert">{error}</p>}
    {query.isPending ? <p role="status">{d.i18n.t('pathInvitation.loading')}</p> : query.isError ? <div role="alert"><p>{d.i18n.t('pathInvitation.unavailableHeading')}</p><button disabled={busy} onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : items.length ? <ul className="studio-settings-people studio-settings-card">{items.map(item => <li key={item.invitation.id}><div><h2>{item.pathName}</h2><p>{d.i18n.t('pathInvitation.pendingContext', { ...item.inviter, pathName: item.pathName })}</p><strong>{d.i18n.t(`pathInvitation.role.${item.invitation.offeredRole}`)}</strong><small>{d.i18n.date(Date.parse(item.invitation.createdAt), { dateStyle: 'medium', timeStyle: 'short' })}</small></div><div className="studio-member-actions">{(['accept', 'decline'] as const).map(decision => <button key={decision} className={decision === 'accept' ? 'studio-primary' : ''} disabled={busy} onClick={() => { commands.clear(); setError(''); setSelected({ item, decision }); }}>{d.i18n.t(decision === 'accept' ? 'pathInvitation.accept' : 'pathInvitation.reject')}</button>)}</div></li>)}</ul> : <section className="studio-settings-card"><h2>{d.i18n.t('pathInvitation.pendingEmpty')}</h2><p>{d.i18n.t('pathInvitation.pendingEmptyDescription')}</p></section>}
    {query.hasNextPage && !query.isError && <button disabled={busy || query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
    {selected && <ConfirmationDialog title={d.i18n.t(selected.decision === 'decline' ? 'pathInvitation.rejectConfirmationHeading' : warning ? 'pathInvitation.visibilityWarning.heading' : 'pathInvitation.accept')} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(busy ? selected.decision === 'accept' ? 'pathInvitation.accepting' : 'pathInvitation.rejecting' : selected.decision === 'decline' ? 'pathInvitation.reject' : warning ? 'pathInvitation.visibilityWarning.confirm' : 'pathInvitation.accept')} cancel={close} confirm={() => void submit()}>
      <p>{d.i18n.t(selected.decision === 'decline' ? 'pathInvitation.rejectConfirmationBody' : 'pathInvitation.pendingContext', { ...selected.item.inviter, pathName: selected.item.pathName })}</p>
      <p>{d.i18n.t(selected.item.invitation.offeredRole === 'participant' ? 'pathInvitation.participantTracking' : 'pathInvitation.supporterReadOnly')}</p>
      {warning && <><strong>{d.i18n.t(`pathInvitation.visibilityWarning.audience.${warning.pathVisibility}`)}</strong><p>{d.i18n.t('pathInvitation.visibilityWarning.exposure')}</p><p>{d.i18n.t('pathInvitation.visibilityWarning.privacyScope')}</p>{warning.hasRetainedActivity && <p>{d.i18n.t('pathInvitation.visibilityWarning.retainedActivity')}</p>}</>}
      {error && <div role="alert"><p>{error}</p><button disabled={busy} onClick={() => { close(); void query.refetch(); }}>{d.i18n.t('common.refresh')}</button></div>}
    </ConfirmationDialog>}
  </main></StudioShell>;
}
