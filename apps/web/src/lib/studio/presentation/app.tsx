import type { BlockingRepository } from '../blocking/ports/blocking-repository';
import { NotificationsPage } from './notifications';
import { NotificationsContext } from './notification-navigation';
import type { Notifications } from '../notifications/ports/notifications';
import { OwnershipPage } from './ownership-page';
import type { OwnershipRepository } from '../ownership/ports/ownership-repository';
import { InvitationInbox } from './invitation-inbox';
import { SharingPage } from './sharing-page';
import type { SharingRepository } from '../sharing/ports/sharing-repository';
import type { AccountSession } from '../session/ports/account-session';
import { SettingsPage } from './settings-page';
import type { PreferencesRepository } from '../preferences/ports/preferences-repository';
import { StatisticsPage } from './statistics-page';
import type { StatisticsRepository } from '../analytics/ports/statistics-repository';
import { StudioShell } from './studio-shell';
import { FollowingPage, ProfilePage, PeoplePage } from './social-pages';
import type { SocialRepository } from '../social/ports/social-repository';
import { useEffect, useState } from 'react';
import { createRootRoute, createRoute, createRouter, RouterProvider, Outlet, useSearch, useParams } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { Translator } from '@hourpaths/i18n';
import type { PathRepository } from '../paths/ports/path-repository';
import type { Path } from '../paths/domain/path';
import { movePath } from '../paths/domain/order';
import { currentProgress } from '../paths/domain/progress';
import './studio.css';
import { duration } from './duration';
import { PathGoalsEditor } from './path-goals';
import { PathActions } from './path-actions';
import { CreatePath } from './create-path';
import { ActivityTimeline } from './activity-timeline';
import type { HistoryRepository } from '../history/ports/history-source';
import type { ActivityRepository } from '../history/ports/activity-repository';
import { ActivityDetailPage } from './activity-detail';
import { ActivityEditorPage } from './activity-editor';

export interface StudioDependencies {
  blocking: BlockingRepository;
  notifications: Notifications;
  ownership: OwnershipRepository;
  sharing: SharingRepository;
  session: AccountSession;
  preferences: PreferencesRepository;
  paths: PathRepository;
  social: SocialRepository;
  statistics: StatisticsRepository;
  history: HistoryRepository;
  activities: ActivityRepository;
  accountScope: string;
  i18n: Translator;
  operationId(): string;
  now(): number;
}
export function StudioApp({ dependencies: d }: { dependencies: StudioDependencies }) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 30_000, refetchOnWindowFocus: false } } }));
  const [router] = useState(() => {
    const root = createRootRoute({ component: Outlet });
    const paths = createRoute({ getParentRoute: () => root, path: '/', validateSearch: (search: Record<string, unknown>) => ({ activityDeleted: search.activityDeleted === true, memberSteppedDown: search.memberSteppedDown === true }), component: () => <PathsPage dependencies={d} /> });
    const ownership = createRoute({ getParentRoute: () => root, path: '/paths/$pathId/ownership', component: () => <OwnershipPage dependencies={d} /> });
    const notifications = createRoute({ getParentRoute: () => root, path: '/notifications', component: () => <NotificationsPage dependencies={d} /> });
    const inbox = createRoute({ getParentRoute: () => root, path: '/invitations', component: () => <InvitationInbox dependencies={d} /> });
    const path = createRoute({ getParentRoute: () => root, path: '/paths/$pathId', component: () => <PathPage dependencies={d} /> });
    const sharing = createRoute({ getParentRoute: () => root, path: '/paths/$pathId/share', component: () => <SharingPage dependencies={d} /> });
    const following = createRoute({ getParentRoute: () => root, path: '/following', component: () => <FollowingPage dependencies={d} /> });
    const activity = createRoute({ getParentRoute: () => root, path: '/paths/$pathId/activities/$activityId', validateSearch: (search: Record<string, unknown>) => ({ activitySaved: search.activitySaved === true }), component: () => <ActivityDetailPage dependencies={d} /> });
    const addActivity = createRoute({ getParentRoute: () => root, path: '/paths/$pathId/activities/new', component: () => <ActivityEditorPage dependencies={d} /> });
    const editActivity = createRoute({ getParentRoute: () => root, path: '/paths/$pathId/activities/$activityId/edit', component: () => <ActivityEditorPage dependencies={d} editing /> });
    const people = createRoute({ getParentRoute: () => root, path: '/people', component: () => <PeoplePage dependencies={d} /> });
    const profile = createRoute({ getParentRoute: () => root, path: '/profile/$username', component: () => <ProfilePage dependencies={d} /> });
    const statistics = createRoute({ getParentRoute: () => root, path: '/stats', component: () => <StatisticsPage dependencies={d} /> });
    const settings = createRoute({ getParentRoute: () => root, path: '/settings/$section', component: () => <SettingsPage dependencies={d} /> });
    return createRouter({ routeTree: root.addChildren([paths, path, inbox, notifications, ownership, sharing, activity, addActivity, editActivity, following, people, profile, statistics, settings]), basepath: '/studio' });
  });
  useEffect(() => () => { void client.cancelQueries(); client.clear(); }, [client]);
  useEffect(() => {
    const key = [d.accountScope, 'notificationOperation'];
    d.notifications.start({
      run: operation => client.fetchQuery({ queryKey: key, queryFn: async ({ signal }) => ({ result: await operation(signal) }), staleTime: 0, gcTime: 0, retry: false }).then(value => value.result),
      cancel: () => { void client.cancelQueries({ queryKey: key }); client.removeQueries({ queryKey: key }); },
    });
    return () => d.notifications.dispose();
  }, [d, client]);
  return <NotificationsContext.Provider value={d.notifications}><QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider></NotificationsContext.Provider>;
}
function PathPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId } = useParams({ strict: false }) as { pathId: string };
  const query = useQuery({ queryKey: [d.accountScope, 'path', pathId], queryFn: ({ signal }) => d.paths.read(pathId, signal), staleTime: 0, gcTime: 0, refetchOnWindowFocus: true });
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-main"><header className="studio-header"><h1>{query.data && !query.isError && !query.isFetching ? query.data.name : d.i18n.t('studio.path')}</h1></header>
    {query.isPending || query.isFetching ? <p role="status">{d.i18n.t('common.loading')}</p> : query.isError ? <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <ul className="studio-paths"><PathRow key={pathId} path={query.data} dependencies={d} moving={false} initialDetails /></ul>}
  </main></StudioShell>;
}
function PathsPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const searchState = useSearch({ strict: false }) as { activityDeleted?: boolean; memberSteppedDown?: boolean };
  const reorder = useMutation({ mutationFn: (paths: readonly Path[]) => d.paths.reorder(paths, d.operationId()), onSuccess: () => client.invalidateQueries({ queryKey: [d.accountScope, 'paths'] }) });
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const [filter, setFilter] = useState<'all' | 'pinned' | 'archived'>('all');
  const query = useQuery({
    queryKey: [d.accountScope, 'paths', filter === 'archived'],
    queryFn: ({ signal }) => d.paths.list(filter === 'archived', signal),
  });
  const paths = query.data?.filter(path => (filter !== 'pinned' || path.pinned) && path.name.toLocaleLowerCase(d.i18n.locale).includes(search.toLocaleLowerCase(d.i18n.locale)));
  return <StudioShell page="paths" i18n={d.i18n}>
    <main className="studio-main">
      <header className="studio-header"><h1>{d.i18n.t('studio.paths')}</h1>
        <input type="search" aria-label={d.i18n.t('studio.searchPaths')} placeholder={d.i18n.t('studio.searchPaths')} value={search} onChange={event => setSearch(event.target.value)} />
        <button className="studio-primary" onClick={() => setCreating(true)}>{d.i18n.t('home.createPath')}</button>
      </header>
      {searchState.memberSteppedDown && <p role="status">{d.i18n.t('pathMembers.roleChangedParticipant')}</p>}
      {searchState.activityDeleted && <p role="status">{d.i18n.t('pathDetails.deleted')}</p>}
      {creating && <CreatePath dependencies={d} close={() => setCreating(false)} />}
      <div className="studio-filters">{(['all', 'pinned', 'archived'] as const).map(value => <button key={value} aria-pressed={filter === value} onClick={() => setFilter(value)}>{d.i18n.t(`studio.filter.${value}`)}</button>)}</div>
      <div className="studio-columns" aria-hidden="true"><span>{d.i18n.t('studio.path')}</span><span>{d.i18n.t('studio.progress')}</span><span>{d.i18n.t('studio.goal')}</span><span>{d.i18n.t('studio.timer')}</span></div>
      {query.isPending ? <p role="status">{d.i18n.t('studio.loading')}</p> : query.isError ? <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> :
        paths?.length ? <ul className="studio-paths">{paths.map(path => <PathRow key={path.id} path={path} dependencies={d} moving={reorder.isPending} move={filter === 'archived' ? undefined : direction => reorder.mutate(movePath(query.data ?? [], path.id, direction))} />)}</ul> : <p>{d.i18n.t('studio.empty')}</p>}
      {reorder.isError && <p role="alert">{d.i18n.t('errors.apiRejected')}</p>}
    </main>
    <ActivityTimeline paths={d.paths} history={d.history} accountScope={d.accountScope} i18n={d.i18n} />
  </StudioShell>;
}
function PathRow({ path, dependencies: d, move, moving, initialDetails = false }: { path: Path; dependencies: StudioDependencies; move?: (direction: -1 | 1) => void; moving: boolean; initialDetails?: boolean }) {
  const [details, setDetails] = useState(initialDetails);
  const client = useQueryClient();
  const appearance = useQuery({ queryKey: [d.accountScope, 'appearance', path.id], queryFn: ({ signal }) => d.paths.appearance(path.id, signal) });
  const key = [d.accountScope, 'tracking', path.id];
  const query = useQuery({ queryKey: key, queryFn: ({ signal }) => d.paths.tracking(path.id, signal), enabled: path.canTrack && !path.archived });
  const [now, setNow] = useState(d.now);
  useEffect(() => {
    if (!query.data?.activeSession) return;
    const interval = setInterval(() => setNow(d.now()), 1000);
    return () => clearInterval(interval);
  }, [query.data?.activeSession, d]);
  const progress = query.data ? currentProgress(query.data, now) : null;
  useEffect(() => {
    if (!progress?.needsPeriodRefresh) return;
    const timer = setTimeout(() => void query.refetch(), query.isError ? 30_000 : 1000);
    return () => clearTimeout(timer);
  }, [progress?.needsPeriodRefresh, query.dataUpdatedAt, query.errorUpdatedAt, query.isError]);
  const mutation = useMutation({
    mutationFn: () => {
      const session = query.data?.activeSession;
      return session ? d.paths.stop(path.id, session.id, d.operationId()) : d.paths.start(path.id, d.operationId());
    },
    onSuccess: value => { client.setQueryData(key, value); setNow(d.now()); void client.invalidateQueries({ queryKey: [d.accountScope, 'history'] }); },
  });
  const format = (seconds: number) => duration(d.i18n, seconds);
  const value = path.goal ? progress?.periodSeconds : progress?.totalSeconds;
  return <li className="studio-path" data-color={appearance.data?.color}>
    <PathActions path={path} dependencies={d} move={move} moving={moving} />
    <div className="studio-path-name"><span className="studio-emoji" aria-hidden="true">{appearance.data?.emoji ?? '✨'}</span><button className="studio-path-open" onClick={() => setDetails(!details)} aria-expanded={details}><strong>{path.name}</strong>{path.pinned && <small>{d.i18n.t('home.arrange.pinnedHeading')}</small>}</button></div>
    <div><strong className="studio-time">{value == null ? '—' : format(value)}</strong>
      {path.goal && progress && <small>{d.i18n.t('studio.lifetime', { time: format(progress.totalSeconds) })}</small>}
      {path.goal && value != null && <progress aria-label={d.i18n.t('studio.progress')} max={path.goal.targetSeconds} value={Math.min(value, path.goal.targetSeconds)} />}</div>
    <div>{path.goal ? <><strong>{format(path.goal.targetSeconds)}</strong><small>{d.i18n.t(`studio.period.${path.goal.recurrence}`)}</small></> : d.i18n.t('studio.totalTime')}</div>
    <div className="studio-timer-actions">{path.canTrack && !path.archived && <button className={query.data?.activeSession ? 'studio-stop' : ''} disabled={query.isPending || query.isError || mutation.isPending} onClick={() => mutation.mutate()}>
      {query.data?.activeSession ? <>{d.i18n.t('timer.stop')} · {format(progress?.sessionSeconds ?? 0)}</> : d.i18n.t('timer.start')}
    </button>}
    {(query.isError || mutation.isError) && <button onClick={() => { mutation.reset(); void query.refetch(); }}>{d.i18n.t('common.retry')}</button>}</div>
    {details && <PathGoalsEditor path={path} dependencies={d} close={() => setDetails(false)} />}
  </li>;
}
