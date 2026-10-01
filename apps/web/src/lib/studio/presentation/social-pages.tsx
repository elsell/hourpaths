import { ProfileBlocking } from './profile-blocking';
import { useCallback, useState } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { ActivePerson, Profile } from '../social/domain/activity';
import { StudioShell } from './studio-shell';
import { Avatar } from './avatar';
import { LiveViewer } from './live-viewer';
import { SocialFeed } from './social-feed';

export function FollowingPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const [selected, setSelected] = useState<string | null>(null);
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'active'], initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => d.social.active(pageParam, signal), getNextPageParam: page => page.next ?? undefined, refetchInterval: 15000, refetchOnWindowFocus: true });
  const groups = new Map<string, ActivePerson>();
  for (const group of query.data?.pages.flatMap(page => page.items) ?? []) {
    const paths = [...new Map([...(groups.get(group.person.id)?.paths ?? []), ...group.paths].map(path => [path.id, path])).values()];
    groups.set(group.person.id, { person: group.person, paths });
  }
  const people = query.isError ? [] : [...groups.values()].filter(group => group.paths.length);
  const refresh = useCallback(() => { void query.refetch(); }, [query.refetch]);
  return <StudioShell page="following" i18n={d.i18n}><main className="studio-main studio-social-main">
    <header className="studio-header"><div><h1>{d.i18n.t('studio.social.following')}</h1><p>{d.i18n.t('studio.social.intro')}</p></div><Link className="studio-link-button" to="/people">{d.i18n.t('studio.social.findPeople')}</Link></header>
    <section className="studio-active"><h2>{d.i18n.t('studio.social.active')}</h2>
      {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
      <div className="studio-active-people">{people.map(group => <button key={group.person.id} onClick={() => setSelected(group.paths[0].id)} aria-label={d.i18n.t('studio.social.activeLabel', { name: group.person.name, count: group.paths.length, paths: group.paths.map(path => path.pathName).join(', ') })}><Avatar person={group.person} segments={group.paths.length} /><span>{group.person.name}</span></button>)}</div>
      {!query.isPending && !query.isError && !people.length && <p>{d.i18n.t('studio.social.noActive')}</p>}
      {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={refresh}>{d.i18n.t('common.retry')}</button></p>}
      {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.social.morePeople')}</button>}
    </section>
    <SocialFeed dependencies={d} />
    {selected && <LiveViewer people={people} selected={selected} close={() => setSelected(null)} refresh={refresh} dependencies={d} />}
  </main></StudioShell>;
}
export function ProfilePage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { username = '' } = useParams({ strict: false });
  return <ProfileContent key={username} username={username} dependencies={d} />;
}
function ProfileContent({ username, dependencies: d }: { username: string; dependencies: StudioDependencies }) {
  const [selected, setSelected] = useState<string | null>(null);
  const query = useQuery({ queryKey: [d.accountScope, 'social', 'profile', username], queryFn: ({ signal }) => d.social.profile(username, signal), refetchOnWindowFocus: true });
  const paths = useQuery({ queryKey: [d.accountScope, 'social', 'profilePaths', username], queryFn: ({ signal }) => d.social.profilePaths(username, signal), refetchInterval: 15000, refetchOnWindowFocus: true, enabled: Boolean(query.data) });
  const refresh = useCallback(() => { void paths.refetch(); }, [paths.refetch]);
  const profile = query.data;
  const active = !query.isError && !paths.isError ? paths.data?.active : undefined;
  return <StudioShell page="following" i18n={d.i18n}><main className="studio-main studio-social-main">
    <header className="studio-header"><h1>{profile ? `@${profile.username}` : d.i18n.t('studio.social.profile')}</h1><Link to="/following">{d.i18n.t('studio.social.following')}</Link></header>
    {query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {profile && !query.isError && <><section className="studio-profile-header">
      <div className="studio-profile-top">{active?.paths.length ? <button className="studio-profile-avatar" onClick={() => setSelected(active.paths[0].id)} aria-label={d.i18n.t('studio.social.activeLabel', { name: profile.name, count: active.paths.length, paths: active.paths.map(path => path.pathName).join(', ') })}><Avatar person={profile} segments={active.paths.length} /></button> : <div className="studio-profile-avatar"><Avatar person={profile} /></div>}
      <div className="studio-profile-identity"><h2>{profile.name}</h2><span>@{profile.username}</span><dl>
        <div><dt>{d.i18n.t('studio.paths')}</dt><dd>{paths.data && !paths.isError ? d.i18n.number(paths.data.count) : '—'}</dd></div>
        <div><dt>{d.i18n.t('studio.social.followers')}</dt><dd>{d.i18n.number(profile.followers)}</dd></div>
        <div><dt>{d.i18n.t('studio.social.following')}</dt><dd>{d.i18n.number(profile.following)}</dd></div>
      </dl></div></div>
      {profile.description && <p className="studio-profile-description">{profile.description}</p>}<FollowButton profile={profile} dependencies={d} /><ProfileBlocking profile={profile} dependencies={d} />
      {paths.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={refresh}>{d.i18n.t('common.retry')}</button></p>}
    </section><SocialFeed dependencies={d} username={username} /></>}
    {selected && <LiveViewer people={active ? [active] : []} selected={selected} close={() => setSelected(null)} refresh={refresh} dependencies={d} />}
  </main></StudioShell>;
}
function FollowButton({ profile, dependencies: d }: { profile: Profile; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const mutation = useMutation({ mutationFn: () => d.social.relationship(profile, d.operationId()), onSuccess: () => client.invalidateQueries({ queryKey: [d.accountScope, 'social'] }) });
  if (profile.relationship === 'self') return null;
  return <div className="studio-follow"><button className={profile.relationship === 'none' ? 'studio-primary' : ''} disabled={mutation.isPending} onClick={() => mutation.mutate()}>{d.i18n.t(profile.relationship === 'following' ? 'studio.social.unfollow' : profile.relationship === 'requested' ? 'studio.social.cancelRequest' : 'studio.social.follow')}</button>{mutation.isError && <p role="alert">{d.i18n.t('studio.social.actionFailed')}</p>}</div>;
}
export function PeoplePage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const [draft, setDraft] = useState('');
  const [search, setSearch] = useState('');
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'search', search], enabled: search.length >= 2, initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => d.social.search(search, pageParam, signal), getNextPageParam: page => page.next ?? undefined });
  const items = [...new Map((query.data?.pages.flatMap(page => page.items) ?? []).map(person => [person.id, person])).values()];
  return <StudioShell page="following" i18n={d.i18n}><main className="studio-main studio-social-main"><header className="studio-header"><h1>{d.i18n.t('studio.social.findPeople')}</h1><Link to="/following">{d.i18n.t('studio.social.following')}</Link></header>
    <form className="studio-people-search" onSubmit={event => { event.preventDefault(); setSearch(draft.trim()); }}><label>{d.i18n.t('studio.social.searchPeople')}<input type="search" value={draft} onChange={event => setDraft(event.target.value)} /></label><button disabled={draft.trim().length < 2}>{d.i18n.t('studio.social.search')}</button></form>
    {query.isFetching && <p role="status">{d.i18n.t('studio.loading')}</p>}
    <ul className="studio-person-list">{items.map(profile => <li key={profile.id}><Link to="/profile/$username" params={{ username: profile.username }}><Avatar person={profile} /><span><strong>{profile.name}</strong><small>@{profile.username}</small></span></Link><FollowButton profile={profile} dependencies={d} /></li>)}</ul>
    {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {search.length >= 2 && query.isSuccess && !items.length && <p>{d.i18n.t('studio.social.noPeople')}</p>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.social.morePeople')}</button>}
    <FollowRequests dependencies={d} />
  </main></StudioShell>;
}
function FollowRequests({ dependencies: d }: { dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'requests'], initialPageParam: undefined as string | undefined, queryFn: ({ pageParam, signal }) => d.social.requests(pageParam, signal), getNextPageParam: page => page.next ?? undefined });
  const mutation = useMutation({ mutationFn: ({ id, accept }: { id: string; accept: boolean }) => d.social.answerRequest(id, accept, d.operationId()), onSuccess: () => client.invalidateQueries({ queryKey: [d.accountScope, 'social'] }) });
  const items = [...new Map((query.data?.pages.flatMap(page => page.items) ?? []).map(item => [item.id, item])).values()];
  return <section className="studio-social-panel"><h2>{d.i18n.t('studio.social.requests')}</h2>{query.isPending && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {!query.isPending && !query.isError && !items.length && <p>{d.i18n.t('studio.social.noRequests')}</p>}
    <ul className="studio-person-list">{items.map(item => <li key={item.id}><Link to="/profile/$username" params={{ username: item.person.username }}><Avatar person={item.person} /><strong>{item.person.name}</strong></Link><button disabled={mutation.isPending} onClick={() => mutation.mutate({ id: item.id, accept: true })}>{d.i18n.t('studio.social.accept')}</button><button disabled={mutation.isPending} onClick={() => mutation.mutate({ id: item.id, accept: false })}>{d.i18n.t('studio.social.decline')}</button></li>)}</ul>
    {query.isError && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {mutation.isError && <p role="alert">{d.i18n.t('studio.social.actionFailed')}</p>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.social.morePeople')}</button>}
  </section>;
}
