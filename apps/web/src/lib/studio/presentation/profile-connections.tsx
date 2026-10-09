import { createFollowerRemovalOwner } from '@hourpaths/client-core';
import { useEffect, useRef, useState } from 'react';
import { Link, useParams, useBlocker } from '@tanstack/react-router';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ConnectionDirection } from '@hourpaths/client-core';
import type { Person } from '../social/domain/activity';
import type { StudioDependencies } from './app';
import { StudioShell } from './studio-shell';
import { Avatar } from './avatar';
import { ConfirmationDialog } from './confirmation-dialog';
import { useOwnedOperation } from './use-owned-operation';

export function ProfileConnectionsPage({ dependencies: d, direction }: { dependencies: StudioDependencies; direction: ConnectionDirection }) {
  const { username = '' } = useParams({ strict: false });
  return <Connections key={JSON.stringify([d.accountScope, username, direction])} dependencies={d} username={username} direction={direction} />;
}
function Connections({ dependencies: d, username, direction }: { dependencies: StudioDependencies; username: string; direction: ConnectionDirection }) {
  const client = useQueryClient(), operation = useOwnedOperation(d);
  const profile = useQuery({ queryKey: [d.accountScope, 'social', 'profile', username], queryFn: ({ signal }) => d.social.profile(username, signal), refetchInterval: 15000 });
  const allowed = !profile.isError && (profile.data?.relationship === 'self' || profile.data?.relationship === 'following');
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'social', 'connections', username, direction], enabled: allowed,
    initialPageParam: undefined as string | undefined, queryFn: ({ pageParam, signal }) => d.social.connections(username, direction, pageParam, signal),
    getNextPageParam: page => page.next ?? undefined, refetchOnWindowFocus: true, refetchInterval: 15000 });
  const [selected, setSelected] = useState<Person | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState(false);
  const admitted = useRef(false), owner = useRef<ReturnType<typeof createFollowerRemovalOwner> | null>(null);
  useEffect(() => () => owner.current?.cancel(), []);
  useEffect(() => { if (!allowed) { setSelected(null); owner.current?.cancel(); } }, [allowed]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  const items = allowed && !query.isError ? [...new Map((query.data?.pages.flatMap(page => page.items) ?? []).map(item => [item.id, item])).values()] : [];
  async function remove() {
    const viewer = profile.data;
    if (!viewer || viewer.relationship !== 'self' || !selected || admitted.current || !allowed) return;
    admitted.current = true; setBusy(true); setError(false);
    owner.current ??= createFollowerRemovalOwner(viewer.id, d.operationId);
    const result = await owner.current.remove(viewer.id, selected.id, (id, key) => operation.run(() => d.social.removeFollower(id, key)));
    if (operation.active()) {
      if (result.kind === 'applied') {
        setSelected(null);
        await client.invalidateQueries({ queryKey: [d.accountScope, 'social'] });
      } else if (result.kind === 'failed') setError(true);
      setBusy(false);
    }
    admitted.current = false;
  }
  const title = d.i18n.t(direction === 'followers' ? 'social.profileFollowers' : 'social.profileFollowing');
  return <StudioShell page="following" i18n={d.i18n}><main className="studio-main studio-social-main">
    <header className="studio-header"><div><h1>{title}</h1><Link to="/profile/$username" params={{ username }}>@{username}</Link></div></header>
    {(profile.isPending || (allowed && query.isPending)) && <p role="status">{d.i18n.t('studio.loading')}</p>}
    {!profile.isPending && !allowed && <p>{d.i18n.t('connections.hidden')}</p>}
    {(profile.isError || (allowed && query.isError)) && <p role="alert">{d.i18n.t('connections.loadFailed')} <button onClick={() => { void profile.refetch(); if (allowed) void query.refetch(); }}>{d.i18n.t('common.retry')}</button></p>}
    {allowed && !query.isPending && !query.isError && !items.length && <p>{d.i18n.t(direction === 'followers' ? 'connections.noFollowers' : 'connections.noFollowing')}</p>}
    <ul className="studio-person-list">{items.map(person => <li key={person.id}><Link to="/profile/$username" params={{ username: person.username }}><Avatar person={person} /><span><strong>{person.name}</strong><small>@{person.username}</small></span></Link>
      {profile.data?.relationship === 'self' && direction === 'followers' && <button disabled={busy} aria-label={d.i18n.t('connections.removeLabel', { name: person.name })} onClick={() => { setSelected(person); setError(false); }}>{d.i18n.t('connections.remove')}</button>}
    </li>)}</ul>
    {allowed && !query.isError && query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.social.morePeople')}</button>}
    {selected && <ConfirmationDialog title={d.i18n.t('connections.confirmTitle', { name: selected.name })} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(busy ? 'connections.removing' : 'connections.remove')} cancel={() => { setSelected(null); owner.current?.cancel(); }} confirm={() => void remove()}>
      <p>{d.i18n.t('connections.confirmDescription')}</p>{error && <p role="alert">{d.i18n.t('connections.removeFailed')}</p>}
    </ConfirmationDialog>}
  </main></StudioShell>;
}
