import type { MessageKey } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { Navigate, useBlocker } from '@tanstack/react-router';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { ConfirmationDialog } from './confirmation-dialog';
import { duration } from './duration';
import { allowsMemberAction, type MemberAction, type MemberReview, type MemberRole, type PathMember } from '../sharing/domain/members';
const roleKey = (role: MemberRole) => role === 'administrator' ? 'pathMembers.administrators' : `pathMembers.${role}` as const;
function actionKey(member: PathMember, action: MemberAction) {
  if (action === 'remove') return member.role === 'participant' ? 'pathMembers.removeParticipantAndData' : 'pathMembers.removeSupporter';
  if (action === 'administrator') return 'pathMembers.confirmGrantAdministrator';
  if (action === 'supporter') return 'pathMembers.confirmSupporter';
  if (member.role === 'administrator') return member.canStepDownAdministrator ? 'pathMembers.confirmStepDownAdministrator' : 'pathMembers.confirmRevokeAdministrator';
  return 'pathMembers.confirmParticipant';
}
export function MemberAccess({ dependencies: d, pathId, disabled, onBusy }: { dependencies: StudioDependencies; pathId: string; disabled: boolean; onBusy(value: boolean): void }) {
  const client = useQueryClient();
  const [commands] = useState(() => d.sharing.memberCommands(d.operationId));
  const active = useRef(true), admitted = useRef(false);
  const [review, setReview] = useState<MemberReview | null>(null);
  const [busy, setBusy] = useState(false), [loadingReview, setLoadingReview] = useState(false);
  const [failure, setFailure] = useState(false), [notice, setNotice] = useState(''), [steppedDown, setSteppedDown] = useState(false);
  const key = [d.accountScope, 'pathMembers', pathId];
  const query = useInfiniteQuery({ queryKey: key, initialPageParam: '', queryFn: ({ pageParam, signal }) => d.sharing.members(pathId, pageParam, signal), getNextPageParam: (page, pages, last, previous) => page.nextCursor && !previous.includes(page.nextCursor) ? page.nextCursor : undefined });
  const members = [...new Map(query.data?.pages.flatMap(page => page.items).map(member => [member.userId, member]) ?? []).values()];
  useEffect(() => () => { active.current = false; commands.dispose(); }, [commands]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function choose(member: PathMember, action: MemberAction) {
    if (disabled || admitted.current || loadingReview) return;
    setLoadingReview(true); onBusy(true); setFailure(false); setNotice('');
    try { const value = await commands.review(pathId, member.userId, action); if (active.current) setReview(value); }
    catch { if (active.current) setFailure(true); }
    finally { if (active.current) { setLoadingReview(false); onBusy(false); } }
  }
  async function submit() {
    if (!review || admitted.current) return;
    admitted.current = true; setBusy(true); onBusy(true); setFailure(false);
    try {
      const result = await commands.submit(review);
      if (!active.current) return;
      if (result.kind === 'failed') { setFailure(true); return; }
      if (result.kind !== 'applied') return;
      const message = review.action === 'remove' ? review.member.role === 'participant' ? 'pathMembers.removedParticipant' : 'pathMembers.removedSupporter' : review.action === 'administrator' ? 'pathMembers.roleChangedAdministrator' : review.action === 'supporter' ? 'pathMembers.roleChangedSupporter' : 'pathMembers.roleChangedParticipant';
      setNotice(d.i18n.t(message)); commands.clear(); setReview(null);
      await client.cancelQueries({ predicate: query => query.queryKey[0] === d.accountScope });
      if (!active.current) return;
      client.removeQueries({ predicate: query => query.queryKey[0] === d.accountScope && !['sharingContext', 'managedInvitations', 'pathMembers'].includes(String(query.queryKey[1])) });
      if (review.member.canStepDownAdministrator && review.action === 'participant') { setSteppedDown(true); return; }
      await client.resetQueries({ queryKey: key });
      await client.invalidateQueries({ queryKey: [d.accountScope, 'sharingContext', pathId] });
    } catch { if (active.current) setFailure(true); }
    finally { admitted.current = false; if (active.current) { setBusy(false); onBusy(false); } }
  }
  if (steppedDown) return <Navigate to="/" search={{ memberSteppedDown: true }} replace />;
  const locked = disabled || busy || loadingReview;
  return <section className="studio-settings-card">
    <header className="studio-section-header"><h2>{d.i18n.t('pathMembers.heading')}</h2><button disabled={locked || query.isFetching} onClick={() => void query.refetch()}>{d.i18n.t('common.refresh')}</button></header>
    {notice && <p role="status">{notice}</p>}
    {loadingReview && <p role="status">{d.i18n.t('common.loading')}</p>}
    {failure && !review && <p role="alert">{d.i18n.t('pathMembers.reviewUnavailableDescription')}</p>}
    {query.isPending ? <p role="status">{d.i18n.t('pathMembers.loading')}</p> : query.isError ? <div role="alert"><p>{d.i18n.t('pathMembers.unavailableDescription')}</p><button disabled={locked} onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <ul className="studio-settings-people">{members.map(member => <li key={member.userId}><div><strong>{d.i18n.t('pathMembers.identity', { ...member })}</strong><small>{d.i18n.t(roleKey(member.role))}</small><small>{d.i18n.t('pathMembers.sessions', { count: member.sessionCount })}</small><small>{d.i18n.t('pathMembers.totalTime', { duration: duration(d.i18n, member.totalTrackedSeconds) })}</small></div><div className="studio-member-actions">{(['participant', 'supporter', 'administrator', 'remove'] as const).filter(action => allowsMemberAction(member, action)).map(action => <button key={action} disabled={locked} onClick={() => void choose(member, action)}>{d.i18n.t(actionKey(member, action))}</button>)}</div></li>)}</ul>}
    {query.hasNextPage && !query.isError && <button disabled={locked || query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('common.loadMore')}</button>}
    {review && <ConfirmationDialog title={d.i18n.t(actionKey(review.member, review.action))} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(busy ? 'common.loading' : actionKey(review.member, review.action))} cancel={() => { commands.clear(); setReview(null); setFailure(false); }} confirm={() => void submit()}>
      <p><strong>{d.i18n.t('pathMembers.identityWithRole', { ...review.member, role: d.i18n.t(roleKey(review.member.role)) })}</strong></p>
      <p>{d.i18n.t('pathMembers.sessions', { count: review.member.sessionCount })}</p><p>{d.i18n.t('pathMembers.totalTime', { duration: duration(d.i18n, review.member.totalTrackedSeconds) })}</p>
      {(review.action === 'remove' || review.action === 'supporter') && <p>{d.i18n.t(review.member.runningTimer ? 'pathMembers.timerRunning' : 'pathMembers.timerNotRunning')}</p>}
      <MemberWarnings review={review} dependencies={d} />
      {failure && <p role="alert">{d.i18n.t(review.action === 'remove' ? 'pathMembers.removalUnavailable' : 'pathMembers.roleChangeUnavailable')}</p>}
    </ConfirmationDialog>}
  </section>;
}
function MemberWarnings({ review, dependencies: d }: { review: MemberReview; dependencies: StudioDependencies }) {
  let keys: MessageKey[];
  if (review.action === 'remove') keys = review.member.role === 'participant' ? ['pathMembers.participantActivityWarning', 'pathMembers.participantSocialWarning', 'pathMembers.offlineWarning', 'pathMembers.runningTimerWarning', 'pathMembers.reinviteWarning'] : ['pathMembers.supporterWarning'];
  else if (review.action === 'supporter') keys = ['pathMembers.roleChangeWarning', 'pathMembers.roleChangeTimerWarning'];
  else if (review.action === 'administrator') keys = ['pathMembers.grantAdministratorWarning'];
  else if (review.member.role === 'administrator') keys = [review.member.canStepDownAdministrator ? 'pathMembers.stepDownAdministratorWarning' : 'pathMembers.revokeAdministratorWarning'];
  else keys = ['pathMembers.roleParticipantEffect'];
  return <>{keys.map(key => <p key={key}>{d.i18n.t(key, { displayName: review.member.displayName })}</p>)}</>;
}
