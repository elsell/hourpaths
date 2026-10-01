import { useEffect, useRef, useState, useId } from 'react';
import { Link, useBlocker, Navigate, useParams } from '@tanstack/react-router';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { ActivitySnapshot, ActivityDeletion } from '../history/domain/detail';
import { reviewActivityDeletion, deleteReviewedActivity } from '../history/application/activity-deletion';
import { StudioShell } from './studio-shell';
import { duration } from './duration';

export function ActivityDetailPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId, activityId } = useParams({ strict: false }) as { pathId: string; activityId: string };
  const [review, setReview] = useState<ActivityDeletion | null>(null);
  const query = useQuery({ queryKey: [d.accountScope, 'activity', pathId, activityId], queryFn: ({ signal }) => d.activities.detail(pathId, activityId, signal) });
  const revisions = useInfiniteQuery({
    queryKey: [d.accountScope, 'activity-revisions', pathId, activityId], initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) => d.activities.revisions(pathId, activityId, pageParam, signal), enabled: !!query.data,
    getNextPageParam: (page, _pages, _last, previous) => page.next && !previous.includes(page.next) ? page.next : undefined,
  });
  const history = [...new Map((revisions.data?.pages.flatMap(page => page.items) ?? []).map(item => [item.version, item])).values()];
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-settings-main studio-activity-detail">
    <Link to="/">{d.i18n.t('pathDetails.backToHistory')}</Link>
    <header className="studio-header"><h1>{d.i18n.t('pathDetails.activityHeading')}</h1></header>
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {query.isError && <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div>}
    {query.data && <>
      <section className="studio-settings-card"><h2>{query.data.pathName}</h2><ActivityValues value={query.data} dependencies={d} />
        {query.data.owned && <button onClick={() => setReview(reviewActivityDeletion(query.data!, d.operationId()))}>{d.i18n.t('pathDetails.delete')}</button>}
      </section>
      <section className="studio-settings-card"><h2>{d.i18n.t('pathDetails.revisions')}</h2>
        {revisions.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
        {revisions.isError && <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={() => void revisions.refetch()}>{d.i18n.t('common.retry')}</button></div>}
        {!revisions.isPending && !revisions.isError && !history.length && <p>{d.i18n.t('pathDetails.revisionsEmpty')}</p>}
        <ol className="studio-activity-revisions">{history.map(value => <li key={value.version}><h3>{d.i18n.t('pathDetails.revision', { version: d.i18n.number(value.version) })}</h3><p>{d.i18n.t('pathDetails.revisionChanged', { date: d.i18n.date(value.replacedAt, { dateStyle: 'medium', timeStyle: 'short', timeZone: value.timeZone }) })}</p><ActivityValues value={value} dependencies={d} /></li>)}</ol>
        {revisions.hasNextPage && <button disabled={revisions.isFetching} onClick={() => void revisions.fetchNextPage()}>{d.i18n.t('studio.moreHistory')}</button>}
      </section>
    </>}
    {review && <DeleteActivityReview review={review} dependencies={d} close={() => setReview(null)} />}
  </main></StudioShell>;
}
function ActivityValues({ value, dependencies: d }: { value: ActivitySnapshot; dependencies: StudioDependencies }) {
  return <dl className="studio-settings-identity">
    <div><dt>{d.i18n.t('pathDetails.started')}</dt><dd><time dateTime={new Date(value.startedAt).toISOString()}>{d.i18n.date(value.startedAt, { dateStyle: 'medium', timeStyle: 'medium', timeZone: value.timeZone })}</time></dd></div>
    <div><dt>{d.i18n.t('pathDetails.ended')}</dt><dd><time dateTime={new Date(value.endedAt).toISOString()}>{d.i18n.date(value.endedAt, { dateStyle: 'medium', timeStyle: 'medium', timeZone: value.timeZone })}</time></dd></div>
    <div><dt>{d.i18n.t('activity.durationValue')}</dt><dd>{duration(d.i18n, value.seconds)}{value.seconds >= 3600 && value.seconds % 60 !== 0 && <small>{d.i18n.t('pathDetails.duration', { seconds: d.i18n.number(value.seconds) })}</small>}</dd></div>
    <div><dt>{d.i18n.t('activity.occurrenceTimeZone')}</dt><dd>{value.timeZone}</dd></div>
    <div><dt>{d.i18n.t('pathDetails.revision', { version: d.i18n.number(value.version) })}</dt><dd /> </div>
    {value.note !== null && <div><dt>{d.i18n.t('activity.note')}</dt><dd><p className="studio-private-note">{value.note}</p><small>{d.i18n.t('activity.notePrivacy')}</small></dd></div>}
  </dl>;
}
function DeleteActivityReview({ review, dependencies: d, close }: { review: ActivityDeletion; dependencies: StudioDependencies; close(): void }) {
  const client = useQueryClient();
  const [deleted, setDeleted] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null), active = useRef(true), admitted = useRef(false);
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  const heading = useId();
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  useEffect(() => { dialog.current?.showModal(); return () => { active.current = false; }; }, []);
  async function submit() {
    if (admitted.current) return;
    admitted.current = true; setBusy(true); setFailed(false);
    try {
      await deleteReviewedActivity(d.activities, review);
      if (!active.current) return;
      await client.cancelQueries({ queryKey: [d.accountScope] });
      if (!active.current) return;
      client.removeQueries({ queryKey: [d.accountScope] });
      admitted.current = false;
      setDeleted(true);
    } catch { if (active.current) setFailed(true); }
    finally { admitted.current = false; if (active.current) setBusy(false); }
  }
  if (deleted) return <Navigate to="/" />;
  return <dialog ref={dialog} className="studio-settings-guard" aria-labelledby={heading} onCancel={event => { event.preventDefault(); if (!admitted.current) close(); }}><div>
    <h2 id={heading}>{d.i18n.t('pathDetails.deleteConfirmationHeading')}</h2><p>{d.i18n.t('pathDetails.deleteConfirmation')}</p>
    {failed && <p role="alert">{d.i18n.t('errors.apiRejected')}</p>}
    <button autoFocus disabled={busy} onClick={close}>{d.i18n.t('common.cancel')}</button><button disabled={busy} onClick={() => void submit()}>{d.i18n.t(failed ? 'common.retry' : 'pathDetails.confirmDelete')}</button>
    {busy && <p role="status">{d.i18n.t('pathDetails.deleting')}</p>}
  </div></dialog>;
}
