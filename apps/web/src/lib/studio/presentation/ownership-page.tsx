import { useEffect, useRef, useState } from 'react';
import { Link, useBlocker, useParams } from '@tanstack/react-router';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import { ownershipTransferExpiration } from '../../path-ownership-transfer';
import { ownershipCommands } from '../ownership/application/commands';
import type { TransferCommand } from '../ownership/domain/transfer';
import type { StudioDependencies } from './app';
import { ConfirmationDialog } from './confirmation-dialog';
import { StudioShell } from './studio-shell';
export function OwnershipPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId } = useParams({ strict: false }) as { pathId: string };
  return <OwnershipContent key={pathId} pathId={pathId} dependencies={d} />;
}
function OwnershipContent({ pathId, dependencies: d }: { pathId: string; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const [commands] = useState(() => ownershipCommands(d.ownership, d.operationId));
  const active = useRef(true), admitted = useRef(false), attempted = useRef(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [selected, setSelected] = useState<TransferCommand | null>(null);
  const [now, setNow] = useState(d.now);
  const key = [d.accountScope, 'ownership', pathId];
  const query = useQuery({ queryKey: key, queryFn: async ({ signal }) => {
    const path = await d.paths.read(pathId, signal);
    const pending = await d.ownership.pending(pathId, signal);
    return { path, pending };
  }, staleTime: 0, gcTime: 0, refetchOnWindowFocus: true });
  const path = !query.isError && !query.isFetching ? query.data?.path : undefined;
  const transfer = query.data?.pending;
  const pending = transfer && Date.parse(transfer.expiresAt) > now ? transfer : null;
  const canInitiate = !!path?.canTransferOwnership && !path.archived && !pending;
  const candidates = useInfiniteQuery({ queryKey: [...key, 'candidates'], enabled: canInitiate,
    initialPageParam: '', queryFn: ({ pageParam, signal }) => d.ownership.candidates(pathId, pageParam, signal),
    getNextPageParam: (page, pages, last, previous) => page.nextCursor && !previous.includes(page.nextCursor) ? page.nextCursor : undefined });
  const people = [...new Map(candidates.data?.pages.flatMap(page => page.items).map(item => [item.userId, item]) ?? []).values()];
  useEffect(() => { const timer = setInterval(() => setNow(d.now()), 1000); return () => { clearInterval(timer); active.current = false; commands.dispose(); }; }, [commands, d]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  function close() { attempted.current = false; commands.clear(); setSelected(null); setError(''); }
  async function refresh() {
    close();
    await client.cancelQueries({ predicate: q => q.queryKey[0] === d.accountScope });
    if (!active.current) return;
    client.removeQueries({ predicate: q => q.queryKey[0] === d.accountScope && q.queryKey[1] !== 'ownership' });
    await client.resetQueries({ queryKey: key });
  }
  async function review(recipientId: string) {
    if (admitted.current) return;
    admitted.current = true; setBusy(true); setError(''); setNotice(''); attempted.current = false; commands.clear();
    try {
      const value = await d.ownership.review(pathId, recipientId);
      if (active.current) setSelected({ kind: 'initiate', pathId, review: value });
    } catch { if (active.current) setError(d.i18n.t('pathOwnership.unavailable')); }
    finally { admitted.current = false; if (active.current) setBusy(false); }
  }
  async function submit() {
    if (!selected || admitted.current) return;
    admitted.current = true; setBusy(true); setError('');
    try {
      attempted.current = true;
      const result = await commands.submit(selected);
      if (!active.current || !result) return;
      setNotice(d.i18n.t(`studio.ownership.${result.state}`));
      await refresh();
    } catch { if (active.current) setError(d.i18n.t('studio.ownership.failed')); }
    finally { admitted.current = false; if (active.current) setBusy(false); }
  }
  const selectedTime = selected?.kind === 'initiate' ? selected.review : selected?.transfer;
  const expiration = selectedTime ? ownershipTransferExpiration(selectedTime.reviewedAt, selectedTime.expiresAt, selectedTime.viewerTimeZone, d.i18n) : undefined;
  const pendingExpiration = pending ? ownershipTransferExpiration(pending.reviewedAt, pending.expiresAt, pending.viewerTimeZone, d.i18n) : undefined;
  const expired = !!selectedTime && Date.parse(selectedTime.expiresAt) <= now;
  const label = selected?.kind === 'initiate' ? 'pathOwnership.confirm' : selected?.kind === 'accept' ? 'pathOwnership.accept' : selected?.kind === 'decline' ? 'pathOwnership.decline' : 'pathOwnership.cancel';
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-main studio-activity-detail">
    <header className="studio-header"><div><Link to="/paths/$pathId" params={{ pathId }}>{d.i18n.t('studio.path')}</Link><h1>{d.i18n.t('pathOwnership.heading')}</h1>{path && <p>{path.name}</p>}</div><button disabled={busy || query.isFetching} onClick={() => void refresh()}>{d.i18n.t('common.refresh')}</button></header>
    {notice && <p role="status">{notice}</p>}{error && !selected && <p role="alert">{error}</p>}
    {query.isPending || query.isFetching ? <p role="status">{d.i18n.t('common.loading')}</p> : !path ? <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button disabled={busy} onClick={() => void refresh()}>{d.i18n.t('common.retry')}</button></div> : <>
      {path.archived && <p>{d.i18n.t('pathArchive.readOnly')}</p>}
      {pending ? <section className="studio-settings-card"><h2>{d.i18n.t('pathOwnership.pendingHeading')}</h2><h3>{pending.counterpart.displayName}</h3><p>@{pending.counterpart.username}</p>
        <p>{d.i18n.t(pending.counterpartRole === 'recipient' ? 'pathOwnership.creatorPendingExplanation' : 'pathOwnership.recipientPendingExplanation')}</p>
        <p>{pendingExpiration ? d.i18n.t('studio.ownership.expires', { exact: pendingExpiration.exact }) : d.i18n.t('pathOwnership.unavailable')}</p>
        {!path.archived && pendingExpiration && <div className="studio-form-actions">{(pending.counterpartRole === 'recipient' ? ['cancel'] as const : ['accept', 'decline'] as const).map(kind => <button key={kind} disabled={busy} onClick={() => { attempted.current = false; commands.clear(); setError(''); setSelected({ kind, transfer: pending }); }}>{d.i18n.t(`pathOwnership.${kind}`)}</button>)}</div>}
      </section> : canInitiate ? <section className="studio-settings-card"><h2>{d.i18n.t('pathOwnership.selectRecipient')}</h2><p>{d.i18n.t('pathOwnership.explanation')}</p>
        {candidates.isPending ? <p role="status">{d.i18n.t('pathOwnership.loadingCandidates')}</p> : candidates.isError ? <div role="alert"><p>{d.i18n.t('pathOwnership.candidatesUnavailable')}</p><button disabled={busy} onClick={() => void candidates.refetch()}>{d.i18n.t('common.retry')}</button></div> : people.length ? <ul className="studio-settings-people">{people.map(person => <li key={person.userId}><div><strong>{person.displayName}</strong><small>@{person.username}</small>{person.administrator && <small>{d.i18n.t('pathOwnership.administrator')}</small>}</div><button disabled={busy} aria-label={d.i18n.t('pathOwnership.candidateAccessibility', { name: person.displayName })} onClick={() => void review(person.userId)}>{d.i18n.t('pathOwnership.reviewHeading')}</button></li>)}</ul> : <p>{d.i18n.t('pathOwnership.candidatesEmptyDescription')}</p>}
        {candidates.hasNextPage && !candidates.isError && <button disabled={busy || candidates.isFetching} onClick={() => void candidates.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
      </section> : !pending && <p>{d.i18n.t('studio.ownership.empty')}</p>}
    </>}
    {selected && <ConfirmationDialog title={d.i18n.t('pathOwnership.reviewHeading')} busy={busy} confirmDisabled={(expired && !attempted.current) || !expiration} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(label)} cancel={close} confirm={() => void submit()}>
      <strong>{query.data?.path.name}</strong><h3>{selected.kind === 'initiate' ? selected.review.recipient.displayName : selected.transfer.counterpart.displayName}</h3><p>@{selected.kind === 'initiate' ? selected.review.recipient.username : selected.transfer.counterpart.username}</p>
      {selected.kind === 'initiate' ? <><p>{d.i18n.t('pathOwnership.recipientBecomesCreator')}</p><p>{d.i18n.t('pathOwnership.creatorBecomesAdministrator')}</p><p>{d.i18n.t('pathOwnership.noChangeUntilAccepted')}</p></> : <p>{d.i18n.t(selected.kind === 'accept' ? 'studio.ownership.acceptEffect' : 'studio.ownership.unchangedRoles')}</p>}
      <p>{d.i18n.t('studio.ownership.unchangedActivity')}</p><p>{selected.kind === 'initiate' ? expiration?.summary : expiration && d.i18n.t('studio.ownership.expires', { exact: expiration.exact })}</p>
      {(error || expired || !expiration) && <div role="alert"><p>{error || d.i18n.t('pathOwnership.unavailable')}</p><button disabled={busy} onClick={() => void refresh()}>{d.i18n.t('common.refresh')}</button></div>}
    </ConfirmationDialog>}
  </main></StudioShell>;
}
