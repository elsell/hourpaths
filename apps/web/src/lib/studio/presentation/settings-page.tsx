import { PictureSettings } from './picture-settings';
import { ProfilePrivacySettings } from './profile-privacy';
import { createNotificationChannelOperationOwner } from '@hourpaths/client-core';
import { ProfileSettings } from './profile-settings';
import { ProviderSettings } from './provider-settings';
import { AccountDeletionSettings } from './account-deletion';
import { useOwnedOperation } from './use-owned-operation';
import { SessionSettings } from './session-settings';
import { UnsavedChanges } from './unsaved-changes';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { StudioShell } from './studio-shell';
import { PreferenceFailure, type NotificationChannelPreference, type Interactions, type TimeZonePreference } from '../preferences/domain/preferences';
import { PathAppearanceEditor } from './path-appearance';

const sections = ['account', 'appearance', 'notifications', 'privacy', 'blocked'] as const;
export function SettingsPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const params = useParams({ strict: false }) as { section?: string };
  const section = sections.find(value => value === params.section) ?? 'account';
  return <StudioShell page="settings" i18n={d.i18n}><main className="studio-settings-main">
    <header><h1>{d.i18n.t('settings.heading')}</h1><p>{d.i18n.t('studio.settings.intro')}</p></header>
    <div className="studio-settings-layout"><nav aria-label={d.i18n.t('settings.heading')}>{sections.map(value => <Link key={value} to="/settings/$section" params={{ section: value }} aria-current={section === value ? 'page' : undefined}>{d.i18n.t(`studio.settings.${value}`)}</Link>)}</nav>
      <div className="studio-settings-content" key={section}>
        <h2>{d.i18n.t(`studio.settings.${section}`)}</h2>
        {section === 'account' && <AccountSettings dependencies={d} />}
        {section === 'notifications' && <NotificationSettings dependencies={d} />}
        {section === 'privacy' && <InteractionSettings dependencies={d} />}
        {section === 'blocked' && <BlockedSettings dependencies={d} />}
        {section === 'appearance' && <AppearanceSettings dependencies={d} />}
      </div>
    </div>
  </main></StudioShell>;
}
function Status({ pending, error, retry, dependencies: d }: { pending: boolean; error: boolean; retry(): void; dependencies: StudioDependencies }) {
  return pending ? <p role="status">{d.i18n.t('studio.loading')}</p> : error ? <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button onClick={retry}>{d.i18n.t('common.retry')}</button></div> : null;
}
function SaveState({ pending, error, success, dependencies: d }: { pending: boolean; error: unknown; success: boolean; dependencies: StudioDependencies }) {
  return <>{pending && <p role="status">{d.i18n.t('studio.settings.saving')}</p>}{error ? <p role="alert">{d.i18n.t(error instanceof PreferenceFailure && error.kind === 'conflict' ? 'studio.settings.conflict' : 'studio.settings.saveFailed')}</p> : success && <p role="status">{d.i18n.t('studio.settings.saved')}</p>}</>;
}
function useIntent(d: StudioDependencies) {
  const current = useRef<{ value: string; id: string } | null>(null);
  return (value: unknown) => {
    const serialized = JSON.stringify(value);
    if (!current.current || current.current.value !== serialized) current.current = { value: serialized, id: d.operationId() };
    return current.current.id;
  };
}
function AccountSettings({ dependencies: d }: { dependencies: StudioDependencies }) {
  const identity = useQuery({ queryKey: [d.accountScope, 'preferences', 'identity'], queryFn: ({ signal }) => d.preferences.identity(signal) });
  const privacyKey = [identity.data?.id, 'privacy'].join(':');
  const zone = useQuery({ queryKey: [d.accountScope, 'preferences', 'zone'], queryFn: ({ signal }) => d.preferences.timeZone(signal) });
  return <><section className="studio-settings-card"><Status pending={identity.isPending} error={identity.isError} retry={() => void identity.refetch()} dependencies={d} />{identity.data && <dl className="studio-settings-identity"><div><dt>{d.i18n.t('studio.settings.name')}</dt><dd>{identity.data.name}</dd></div><div><dt>{d.i18n.t('settings.account.email')}</dt><dd>{identity.data.email}<small>{d.i18n.t('studio.settings.providerEmail')}</small></dd></div></dl>}</section>
    {identity.data && <ProfilePrivacySettings key={privacyKey} owner={identity.data.id} dependencies={d} />}
    {identity.data && <PictureSettings key={[identity.data.id,'picture'].join(':')} owner={identity.data.id} dependencies={d} />}
    {identity.data && <ProfileSettings key={identity.data.id} owner={identity.data.id} dependencies={d} />}
    {d.providers && <ProviderSettings service={d.providers} i18n={d.i18n} />}<section className="studio-settings-card"><h3>{d.i18n.t('studio.settings.preferences')}</h3><p>{d.i18n.t('studio.settings.locale')}</p><Status pending={zone.isPending} error={zone.isError} retry={() => void zone.refetch()} dependencies={d} />{zone.data && <TimeZoneForm initial={zone.data} dependencies={d} reload={() => zone.refetch()} />}</section><SessionSettings dependencies={d} name={identity.data?.name ?? ''} />{d.deletion && <AccountDeletionSettings service={d.deletion} i18n={d.i18n} />}</>;
}
function TimeZoneForm({ initial, dependencies: d, reload }: { initial: TimeZonePreference; dependencies: StudioDependencies; reload(): Promise<unknown> }) {
  const [saved, setSaved] = useState(initial);
  const [draft, setDraft] = useState(initial.zone);
  const [review, setReview] = useState<{ reviewed: string; proposed: string } | null>(null);
  const operation = useOwnedOperation(d), intent = useIntent(d), client = useQueryClient();
  const mutation = useMutation({ mutationFn: (change: { reviewed: string; proposed: string }) => operation.run(signal => d.preferences.changeTimeZone(change, intent(change), signal)), onSuccess: value => {
    if (!operation.active()) return;
    setSaved(value); setDraft(value.zone); setReview(null);
    client.setQueryData([d.accountScope, 'preferences', 'zone'], value);
    void client.invalidateQueries({ queryKey: [d.accountScope, 'tracking'] });
    void client.invalidateQueries({ queryKey: [d.accountScope, 'statistics'] });
  } });
  useEffect(() => { setSaved(initial); }, [initial]);
  const choices = Array.from(new Set([saved.zone, ...Intl.supportedValuesOf('timeZone')])).sort();
  return <form className="studio-settings-form" onSubmit={event => { event.preventDefault(); mutation.reset(); setReview({ reviewed: saved.zone, proposed: draft }); }}>
    <UnsavedChanges dirty={draft !== saved.zone || mutation.isPending} i18n={d.i18n} />
    <fieldset disabled={mutation.isPending || !!review}><label>{d.i18n.t('settings.timeZone.heading')}<select aria-label={d.i18n.t('settings.timeZone.heading')} value={draft} onChange={event => { setDraft(event.target.value); mutation.reset(); }}>{choices.map(value => <option key={value} value={value}>{value}</option>)}</select></label><button className="studio-primary" disabled={draft === saved.zone}>{d.i18n.t('common.save')}</button></fieldset>
    {review && <div className="studio-settings-review"><h3>{d.i18n.t('settings.timeZone.warningTitle')}</h3><p>{d.i18n.t('settings.timeZone.warning', { current: review.reviewed, proposed: review.proposed })}</p>
      <button type="button" disabled={mutation.isPending} onClick={() => { setReview(null); mutation.reset(); }}>{d.i18n.t('common.cancel')}</button>
      {mutation.error instanceof PreferenceFailure && mutation.error.kind === 'conflict' ? <button type="button" onClick={async () => { await reload(); setReview(null); mutation.reset(); }}>{d.i18n.t('common.retry')}</button> : <button type="button" disabled={mutation.isPending} onClick={() => mutation.mutate(review)}>{d.i18n.t('settings.timeZone.confirm')}</button>}
    </div>}
    <SaveState pending={mutation.isPending} error={mutation.error} success={mutation.isSuccess} dependencies={d} />
  </form>;
}
function PreferenceForm<T>({ initial, dependencies: d, save, children, cacheKey }: { initial: T; dependencies: StudioDependencies; save(value: T, key: string, signal: AbortSignal): Promise<T>; children(value: T, change: (value: T) => void): ReactNode; cacheKey: string }) {
  const [saved, setSaved] = useState(initial), [draft, setDraft] = useState(initial);
  const operation = useOwnedOperation(d), intent = useIntent(d), client = useQueryClient();
  const mutation = useMutation({ mutationFn: () => operation.run(signal => save(draft, intent(draft), signal)), onSuccess: value => {
    if (!operation.active()) return;
    setSaved(value); setDraft(value); client.setQueryData([d.accountScope, 'preferences', cacheKey], value);
    if (cacheKey === 'interactions') void client.invalidateQueries({ queryKey: [d.accountScope, 'social'] });
  } });
  const dirty = JSON.stringify(saved) !== JSON.stringify(draft);
  return <form className="studio-settings-form" onSubmit={event => { event.preventDefault(); mutation.mutate(); }}><UnsavedChanges dirty={dirty || mutation.isPending} i18n={d.i18n} /><fieldset disabled={mutation.isPending}>{children(draft, value => { setDraft(value); mutation.reset(); })}<div className="studio-settings-actions"><button type="button" disabled={!dirty} onClick={() => { setDraft(saved); mutation.reset(); }}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={!dirty || mutation.error instanceof PreferenceFailure && mutation.error.kind === 'conflict'}>{d.i18n.t('common.save')}</button></div></fieldset><SaveState pending={mutation.isPending} error={mutation.error} success={mutation.isSuccess} dependencies={d} />{mutation.error instanceof PreferenceFailure && mutation.error.kind === 'conflict' && <button type="button" onClick={() => { setDraft(saved); mutation.reset(); void client.resetQueries({ queryKey: [d.accountScope, 'preferences', cacheKey] }); }}>{d.i18n.t('common.retry')}</button>}</form>;
}
function InteractionSettings({ dependencies: d }: { dependencies: StudioDependencies }) {
  const query = useQuery({ queryKey: [d.accountScope, 'preferences', 'interactions'], queryFn: ({ signal }) => d.preferences.interactions(signal) });
  return <section className="studio-settings-card"><p>{d.i18n.t('settings.interactions.footer')}</p><Status pending={query.isPending} error={query.isError} retry={() => void query.refetch()} dependencies={d} />{query.data && <PreferenceForm<Interactions> initial={query.data} dependencies={d} cacheKey="interactions" save={(value, key, signal) => d.preferences.saveInteractions(value, key, signal)}>{(value, change) => <>{(['comments', 'reactions'] as const).map(field => <label className="studio-settings-toggle" key={field}><span>{d.i18n.t(`settings.interactions.${field}`)}</span><input type="checkbox" checked={value[field]} onChange={event => change({ ...value, [field]: event.target.checked })} /></label>)}</>}</PreferenceForm>}</section>;
}
function NotificationSettings({ dependencies: d }: { dependencies: StudioDependencies }) {
  const operation = useOwnedOperation(d), client = useQueryClient();
  const [owner] = useState(() => createNotificationChannelOperationOwner(d.operationId));
  useEffect(() => () => owner.cancel(), [owner, d.accountScope]);
  const queryKey = [d.accountScope, 'preferences', 'notification-channels'];
  const query = useQuery({ queryKey, queryFn: ({ signal }) => d.preferences.notificationChannels(signal) });
  const mutation = useMutation({
    mutationFn: (value: NotificationChannelPreference) => operation.run(async signal => {
      const result = await owner.submit(value, (next, key) => d.preferences.saveNotificationChannel(next, key, signal));
      if (result.kind === 'failed') throw result.cause;
      if (result.kind !== 'applied') throw new PreferenceFailure();
      return result.preference;
    }),
    onSuccess: saved => {
      if (!operation.active()) return;
      client.setQueryData<NotificationChannelPreference[]>(queryKey, rows => rows?.map(row => row.channel === saved.channel ? saved : row));
    },
  });
  return <section className="studio-settings-card">
    <p>{d.i18n.t('notification.channels.footer')}</p>
    <Status pending={query.isPending} error={query.isError} retry={() => void query.refetch()} dependencies={d} />
    <div className="studio-settings-form"><fieldset>
    {query.data?.map(row => <label className="studio-settings-toggle" key={row.channel}>
      <span>{d.i18n.t(`notification.channel.${row.channel}`)}</span>
      <input type="checkbox" checked={row.enabled} disabled={mutation.isPending || mutation.error instanceof PreferenceFailure && mutation.error.kind === 'conflict'} onChange={event => { mutation.reset(); mutation.mutate({ ...row, enabled: event.target.checked }); }} />
    </label>)}
    </fieldset></div>
    <SaveState pending={mutation.isPending} error={mutation.error} success={mutation.isSuccess} dependencies={d} />
    {mutation.isError && <button disabled={mutation.isPending} onClick={() => { owner.cancel(); mutation.reset(); void query.refetch(); }}>{d.i18n.t('common.retry')}</button>}
  </section>;
}
function BlockedSettings({ dependencies: d }: { dependencies: StudioDependencies }) {
  const operation = useOwnedOperation(d), client = useQueryClient();
  const [selected, setSelected] = useState<{ id: string; username: string } | null>(null);
  const intent = useIntent(d);
  const query = useInfiniteQuery({ queryKey: [d.accountScope, 'preferences', 'blocked'], initialPageParam: undefined as string | undefined, queryFn: ({ pageParam, signal }) => d.preferences.blocked(pageParam, signal), getNextPageParam: last => last.next });
  const mutation = useMutation({ mutationFn: (person: { id: string }) => operation.run(signal => d.preferences.unblock(person.id, intent(person.id), signal)), onSuccess: () => { if (!operation.active()) return; setSelected(null); void client.invalidateQueries({ queryKey: [d.accountScope] }); } });
  const people = query.data?.pages.flatMap(page => page.items) ?? [];
  return <section className="studio-settings-card"><Status pending={query.isPending} error={query.isError} retry={() => void query.refetch()} dependencies={d} />{!query.isPending && !query.isError && !people.length && <p>{d.i18n.t('blocking.emptyHeading')}</p>}<ul className="studio-settings-people">{people.map(person => <li key={person.id}><div><strong>{person.name}</strong><small>@{person.username}</small></div><button disabled={mutation.isPending} onClick={() => { setSelected(person); mutation.reset(); }}>{d.i18n.t('studio.settings.unblock')}</button></li>)}</ul>{query.hasNextPage && <button disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{d.i18n.t('studio.settings.more')}</button>}
    {selected && <div className="studio-settings-review"><p>{d.i18n.t('studio.settings.unblockConfirm', { username: selected.username })}</p><button disabled={mutation.isPending} onClick={() => setSelected(null)}>{d.i18n.t('common.cancel')}</button><button disabled={mutation.isPending} onClick={() => mutation.mutate(selected)}>{d.i18n.t('studio.settings.unblock')}</button></div>}<SaveState pending={mutation.isPending} error={mutation.error} success={mutation.isSuccess} dependencies={d} /></section>;
}
function AppearanceSettings({ dependencies: d }: { dependencies: StudioDependencies }) {
  const query = useQuery({ queryKey: [d.accountScope, 'paths', false], queryFn: ({ signal }) => d.paths.list(false, signal) });
  const [selected, setSelected] = useState('');
  return <section className="studio-settings-card"><p>{d.i18n.t('studio.settings.appearanceDetail')}</p><Status pending={query.isPending} error={query.isError} retry={() => void query.refetch()} dependencies={d} />{query.data && <ul className="studio-settings-people">{query.data.map(path => <li key={path.id}><strong>{path.name}</strong><button disabled={!!selected} onClick={() => setSelected(path.id)}>{d.i18n.t('home.appearance.title')}</button></li>)}</ul>}{selected && <div className="studio-settings-appearance"><PathAppearanceEditor key={selected} pathId={selected} guardChanges dependencies={d} close={() => setSelected('')} /></div>}</section>;
}
