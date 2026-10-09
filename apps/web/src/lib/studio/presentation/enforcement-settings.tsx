import { appealAvailable, createAppealSubmissionOwner, EnforcementFailure, type EnforcementNotice } from '@hourpaths/client-core';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import type { StudioDependencies } from './app';
import { UnsavedChanges } from './unsaved-changes';

export function EnforcementSettings({ dependencies: d }: { dependencies: StudioDependencies }) {
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'enforcement'], initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => { if (!d.enforcement) throw new EnforcementFailure(); return d.enforcement.list(pageParam); },
    getNextPageParam: page => page.nextCursor });
  return <>
    {query.isPending && <p role="status">{d.i18n.t('common.loading')}</p>}
    {query.isError && <p role="alert">{d.i18n.t('enforcement.loadFailed')}</p>}
    {query.data?.pages[0].items.length === 0 && <section className="studio-settings-card"><p>{d.i18n.t('enforcement.empty')}</p></section>}
    {query.data?.pages.flatMap(page => page.items).map(notice => <Notice key={notice.id} notice={notice} dependencies={d} />)}
    <button disabled={query.isFetching} onClick={() => void query.refetch()}>{d.i18n.t(query.isError ? 'common.retry' : 'common.refresh')}</button>
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
  </>;
}
function Notice({ notice, dependencies: d }: { notice: EnforcementNotice; dependencies: StudioDependencies }) {
  const client = useQueryClient(), live = useRef(true), admitted = useRef(false);
  const [owner] = useState(() => createAppealSubmissionOwner(d.operationId));
  const [receipt, setReceipt] = useState(notice.appeal), [explanation, setExplanation] = useState('');
  const [editing, setEditing] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState<'failed' | 'conflict' | null>(null);
  useEffect(() => { live.current = true; return () => { live.current = false; owner.cancel(); }; }, [owner]);
  useEffect(() => { if (notice.appeal) setReceipt(notice.appeal); }, [notice.appeal]);
  async function submit() {
    if (admitted.current || !d.enforcement) return; admitted.current = true; setBusy(true); setError(null);
    const result = await owner.submit(notice.id, explanation, (id, text, key) => d.enforcement!.appeal(id, text, key));
    if (live.current) {
      if (result.kind === 'submitted') { setReceipt(result.appeal); setEditing(false); setExplanation(''); void client.invalidateQueries({ queryKey: [d.accountScope, 'enforcement'] }); }
      if (result.kind === 'failed') setError(result.cause instanceof EnforcementFailure && result.cause.kind === 'conflict' ? 'conflict' : 'failed');
      setBusy(false);
    }
    admitted.current = false;
  }
  const date = (value: string) => d.i18n.date(Date.parse(value), { dateStyle: 'medium', timeStyle: 'short' });
  return <section className="studio-settings-card"><h3>{d.i18n.t(`enforcement.action.${notice.action}`)}</h3>
    {notice.affectedComment && <p>{d.i18n.t('enforcement.affectedComment', { date: date(notice.affectedComment.createdAt), id: notice.affectedComment.id })}</p>}
    <time dateTime={notice.issuedAt}>{date(notice.issuedAt)}</time><p>{notice.policyReason}</p>
    {notice.until && <p>{d.i18n.t('enforcement.until', { date: date(notice.until) })}</p>}
    {receipt ? <><p role="status">{d.i18n.t(receipt.outcome ? `enforcement.${receipt.outcome}` : 'enforcement.pending')}</p>{receipt.explanation && <p>{receipt.explanation}</p>}{receipt.decisionReason && <p>{receipt.decisionReason}</p>}</> : <>
      <p>{d.i18n.t('enforcement.appealDeadline', { date: date(notice.appealDeadline) })}</p>
      {appealAvailable(notice, Date.now()) ? editing ? <form className="studio-settings-form" onSubmit={event => { event.preventDefault(); void submit(); }}>
        <UnsavedChanges dirty={!!explanation || busy} i18n={d.i18n} />
        <fieldset disabled={busy}><label>{d.i18n.t('enforcement.explanation')}<textarea value={explanation} onChange={event => setExplanation(event.target.value)} /></label>
          <div className="studio-settings-actions"><button type="button" onClick={() => { setEditing(false); setError(null); }}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={error === 'conflict'}>{d.i18n.t(busy ? 'enforcement.submitting' : 'enforcement.submit')}</button></div>
        </fieldset>
      </form> : <button onClick={() => setEditing(true)}>{d.i18n.t('enforcement.appeal')}</button> : <p>{d.i18n.t('enforcement.closed')}</p>}
    </>}
    {error && <p role="alert">{d.i18n.t(`enforcement.${error}`)}</p>}
  </section>;
}
