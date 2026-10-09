import { ReportAction } from './report-composer';
import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { chronologicalEvents, type SocialEvent } from '../social/domain/activity';
import type { StudioDependencies } from './app';
import { ConnectedTimeline } from './connected-timeline';
import { Avatar } from './avatar';
import { EmojiPicker } from './emoji-picker';
import { Comments, PeopleRoster } from './social-interactions';
import { duration } from './duration';

export function SocialFeed({ dependencies: d, username }: { dependencies: StudioDependencies; username?: string }) {
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'feed', username ?? null], initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => username ? d.social.activity(username, pageParam, signal) : d.social.feed(pageParam, signal),
    getNextPageParam: page => page.next ?? undefined, refetchOnWindowFocus: true,
  });
  const items = chronologicalEvents(query.data?.pages.flatMap(page => page.items) ?? []);
  return <section className="studio-social-feed"><header className="studio-section-header"><h2>{d.i18n.t('studio.social.latest')}</h2><button disabled={query.isFetching} onClick={() => void query.refetch()}>{d.i18n.t('studio.refresh')}</button></header>
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {!query.isPending && !query.isError && !items.length && <p>{d.i18n.t('studio.noActivity')}</p>}
    <ConnectedTimeline items={items} identity={item => item.id} instant={item => item.publishedAt} timeZone={() => Intl.DateTimeFormat().resolvedOptions().timeZone} i18n={d.i18n} className="studio-social-timeline" render={event => <EventCard event={event} dependencies={d} />} />
    {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.moreHistory')}</button>}
  </section>;
}
function EventCard({ event, dependencies: d }: { event: SocialEvent; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const [comments, setComments] = useState(false);
  const [picker, setPicker] = useState(false);
  const [roster, setRoster] = useState<string | null>(null);
  const appearance = useQuery({ queryKey: [d.accountScope, 'appearance', event.pathId], queryFn: ({ signal }) => d.paths.appearance(event.pathId, signal) });
  const react = useMutation({ mutationFn: (emoji: string) => d.social.reactions(event.id, emoji, event.reactions.some(value => value.emoji === emoji && value.selected), d.operationId()),
    onSuccess: async () => { setPicker(false); await client.invalidateQueries({ queryKey: [d.accountScope, 'social'] }); },
  });
  return <article className="studio-social-event">
    <Link to="/profile/$username" params={{ username: event.author.username }} className="studio-event-avatar" aria-label={event.author.name}><Avatar person={event.author} /></Link>
    <div className="studio-event-body"><header><Link to="/profile/$username" params={{ username: event.author.username }}><strong>{event.author.name}</strong> <span>@{event.author.username}</span></Link><time dateTime={new Date(event.publishedAt).toISOString()}>{d.i18n.time(event.publishedAt, { hour: 'numeric', minute: '2-digit' })}</time></header>
      <p>{d.i18n.t(event.kind === 'practice_session' ? 'studio.social.practiced' : 'studio.social.achieved', { path: event.pathName, time: duration(d.i18n, event.seconds) })}{event.edited && <small> · {d.i18n.t('studio.social.edited')}</small>}</p>
      <div className="studio-social-path" data-color={appearance.data?.color ?? 'lavender'}><span className="studio-emoji" aria-hidden="true">{appearance.data?.emoji ?? '✨'}</span><strong>{event.pathName}</strong><div><strong>{duration(d.i18n, event.seconds)}</strong><small>{d.i18n.t(event.kind === 'practice_session' ? 'studio.social.practice' : 'studio.social.achievement')}</small></div></div>
      <div className="studio-interactions"><ReportAction menu target={{ kind: 'feed_event', id: event.id }} dependencies={d} /><button aria-expanded={comments} onClick={() => setComments(!comments)}>{d.i18n.t('studio.social.comments', { count: event.comments })}</button>
        {event.reactions.map(value => <span className="studio-reaction" key={value.emoji} data-selected={value.selected}>
          <button disabled={!event.canReact || react.isPending} aria-pressed={value.selected} aria-label={d.i18n.t('studio.social.react', { emoji: value.emoji })} onClick={() => react.mutate(value.emoji)}>{value.emoji}</button>
          <button aria-label={d.i18n.t('studio.social.reactionPeople', { emoji: value.emoji, count: value.count })} onClick={() => setRoster(value.emoji)}>{d.i18n.number(value.count)}</button>
        </span>)}
        {event.canReact && <button aria-label={d.i18n.t('studio.social.addReaction')} aria-expanded={picker} onClick={() => setPicker(!picker)}>☺ +</button>}
      </div>
      {react.isError && <p role="alert">{d.i18n.t('studio.social.actionFailed')}</p>}
      {picker && <EmojiPicker i18n={d.i18n} choose={emoji => react.mutate(emoji)} disabled={react.isPending} />}
      {roster && <PeopleRoster dependencies={d} eventId={event.id} emoji={roster} close={() => setRoster(null)} />}
      {comments && <Comments event={event} dependencies={d} />}
    </div>
  </article>;
}
