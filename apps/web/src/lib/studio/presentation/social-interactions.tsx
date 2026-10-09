import { ReportAction } from './report-composer';
import { useRef, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { Comment, SocialEvent } from '../social/domain/activity';
import { Avatar } from './avatar';

export function PeopleRoster({ dependencies: d, eventId, emoji, commentId, close }: { dependencies: StudioDependencies; eventId: string; emoji?: string; commentId?: string; close(): void }) {
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'roster', eventId, emoji, commentId], initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => commentId ? d.social.heartPeople(eventId, commentId, pageParam, signal) : d.social.reactionPeople(eventId, emoji!, pageParam, signal), getNextPageParam: page => page.next ?? undefined });
  const people = [...new Map((query.data?.pages.flatMap(page => page.items) ?? []).map(person => [person.id, person])).values()];
  return <section className="studio-social-panel"><header className="studio-section-header"><h3>{emoji} {d.i18n.t('studio.social.reactions')}</h3><button onClick={close}>{d.i18n.t('studio.social.close')}</button></header>
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    <ul className="studio-person-list">{people.map(person => <li key={person.id}><Link to="/profile/$username" params={{ username: person.username }}><Avatar person={person} /><span><strong>{person.name}</strong><small>@{person.username}</small></span></Link></li>)}</ul>
    {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.social.morePeople')}</button>}
  </section>;
}
export function Comments({ event, dependencies: d }: { event: SocialEvent; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const [draft, setDraft] = useState('');
  const submission = useRef<{ id: string; text: string } | null>(null);
  const viewer = useQuery({ queryKey: [d.accountScope, 'viewer'], queryFn: ({ signal }) => d.social.viewer(signal) });
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'comments', event.id], initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => d.social.comments(event.id, pageParam, signal), getNextPageParam: page => page.next ?? undefined });
  const add = useMutation({ mutationFn: () => { const text = draft.trim(); if (submission.current?.text !== text) submission.current = { id: d.operationId(), text }; return d.social.addComment(event.id, text, submission.current.id); }, onSuccess: async () => { submission.current = null; setDraft(''); await client.invalidateQueries({ queryKey: [d.accountScope, 'social'] }); }, onError: () => client.invalidateQueries({ queryKey: [d.accountScope, 'social', 'comments', event.id] }) });
  const comments = [...new Map((query.data?.pages.flatMap(page => page.items) ?? []).map(comment => [comment.id, comment])).values()];
  return <section className="studio-social-panel">
    <h3>{d.i18n.t('studio.social.comments', { count: event.comments })}</h3>
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    <ul className="studio-comments">{comments.map(comment => <CommentRow key={comment.id} comment={comment} event={event} viewerId={viewer.data} dependencies={d} />)}</ul>
    {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.social.moreComments')}</button>}
    {event.canComment && <form className="studio-comment-form" onSubmit={e => { e.preventDefault(); add.mutate(); }}><label>{d.i18n.t('studio.social.writeComment')}<textarea maxLength={2000} value={draft} onChange={e => setDraft(e.target.value)} disabled={add.isPending} /></label><button type="submit" disabled={add.isPending || !draft.trim()}>{d.i18n.t('studio.social.post')}</button></form>}
    {add.isError && <p role="alert">{d.i18n.t('studio.social.actionFailed')}</p>}
  </section>;
}
function CommentRow({ comment, event, viewerId, dependencies: d }: { comment: Comment; event: SocialEvent; viewerId?: string; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const [edit, setEdit] = useState<{ original: Comment; text: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [roster, setRoster] = useState(false);
  const [history, setHistory] = useState(false);
  const mutation = useMutation({ mutationFn: (action: 'heart' | 'edit' | 'delete') => action === 'heart' ? d.social.heart(event.id, comment, d.operationId()) : action === 'delete' ? d.social.deleteComment(event.id, comment.id, d.operationId()) : d.social.editComment(event.id, edit!.original, edit!.text.trim(), d.operationId()),
    onSuccess: async () => { setEdit(null); setDeleting(false); await client.invalidateQueries({ queryKey: [d.accountScope, 'social'] }); } });
  return <li><Link to="/profile/$username" params={{ username: comment.author.username }} className="studio-comment-author"><Avatar person={comment.author} /><strong>{comment.author.name}</strong></Link>
    <p className="studio-comment-text">{comment.text}</p>
    <div className="studio-interactions"><ReportAction menu target={{ kind: 'comment', id: comment.id }} dependencies={d} /><time dateTime={new Date(comment.createdAt).toISOString()}>{d.i18n.date(comment.createdAt, { month: 'short', day: 'numeric' })}</time>
      <button disabled={mutation.isPending} aria-pressed={comment.hearted} onClick={() => mutation.mutate('heart')}>{d.i18n.t('studio.social.heart')}</button><button onClick={() => setRoster(!roster)}>{d.i18n.number(comment.hearts)}</button>
      {comment.edited && <button onClick={() => setHistory(!history)}>{d.i18n.t('studio.social.edited')}</button>}
      {viewerId === comment.author.id && <><button disabled={mutation.isPending} onClick={() => setEdit({ original: comment, text: comment.text })}>{d.i18n.t('studio.social.edit')}</button><button disabled={mutation.isPending} onClick={() => setDeleting(true)}>{d.i18n.t('studio.social.delete')}</button></>}
    </div>
    {edit && <form className="studio-comment-form" onSubmit={e => { e.preventDefault(); mutation.mutate('edit'); }}><label>{d.i18n.t('studio.social.editComment')}<textarea maxLength={2000} value={edit.text} disabled={mutation.isPending} onChange={e => setEdit({ ...edit, text: e.target.value })} /></label><button disabled={mutation.isPending || !edit.text.trim()}>{d.i18n.t('common.save')}</button><button type="button" onClick={() => setEdit(null)}>{d.i18n.t('common.cancel')}</button></form>}
    {deleting && <div><p>{d.i18n.t('studio.social.deleteConfirm')}</p><button disabled={mutation.isPending} onClick={() => mutation.mutate('delete')}>{d.i18n.t('studio.social.delete')}</button><button onClick={() => setDeleting(false)}>{d.i18n.t('common.cancel')}</button></div>}
    {mutation.isError && <p role="alert">{d.i18n.t('studio.social.actionFailed')}</p>}
    {roster && <PeopleRoster eventId={event.id} commentId={comment.id} close={() => setRoster(false)} dependencies={d} />}
    {history && <CommentHistory eventId={event.id} commentId={comment.id} dependencies={d} />}
  </li>;
}
function CommentHistory({ eventId, commentId, dependencies: d }: { eventId: string; commentId: string; dependencies: StudioDependencies }) {
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'commentHistory', eventId, commentId], initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => d.social.commentHistory(eventId, commentId, pageParam, signal), getNextPageParam: page => page.next ?? undefined });
  return <section><h4>{d.i18n.t('studio.social.editHistory')}</h4>{query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    <ol>{query.data?.pages.flatMap(page => page.items).map(item => <li key={item.version}><time>{d.i18n.date(item.createdAt)}</time><p className="studio-comment-text">{item.text}</p></li>)}</ol>
    {query.isError && <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.moreHistory')}</button>}
  </section>;
}
