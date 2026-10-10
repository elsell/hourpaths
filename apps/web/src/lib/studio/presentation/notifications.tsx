import { ReportAction } from './report-composer';
import { useEffect, useState, useSyncExternalStore } from 'react';
import { Link, Navigate } from '@tanstack/react-router';
import type { Notification, NotificationTarget } from '../notifications/domain/notification';
import type { StudioDependencies } from './app';
import { StudioShell } from './studio-shell';
import { ConfirmationDialog } from './confirmation-dialog';
function Destination({ target }: { target: NotificationTarget }) {
  switch (target.kind) {
    case 'reminder': return null;
    case 'invitations': return <Navigate to="/invitations" />;
    case 'people': return <Navigate to="/people" />;
    case 'profile': return <Navigate to="/profile/$username" params={{ username: target.username }} />;
    case 'ownership': return <Navigate to="/paths/$pathId/ownership" params={{ pathId: target.pathId }} />;
    case 'path': return <Navigate to="/paths/$pathId" params={{ pathId: target.pathId }} />;
  }
}
export function NotificationsPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const service = d.notifications;
  const state = useSyncExternalStore(service.subscribe, service.snapshot, service.snapshot);
  const [deleting, setDeleting] = useState<Notification | null>(null), [destination, setDestination] = useState<NotificationTarget | null>(null);
  const [openReminderId, setOpenReminderId] = useState<string | null>(null);
  const openReminder = state.items.find(item => item.id === openReminderId)?.target;
  const [active] = useState(() => ({ current: true }));
  useEffect(() => { void service.refresh(); return () => { active.current = false; }; }, [service, active]);
  const busy = state.loading || state.mutating;
  async function open(item: Notification) {
    if (!await service.mutate({ kind: 'read', notificationId: item.id }) || !active.current) return;
    if (item.target?.kind === 'reminder') {
      await service.refresh();
      if (active.current && !service.snapshot().error) setOpenReminderId(item.id);
    } else setDestination(item.target);
  }
  async function remove() {
    if (!deleting) return;
    if (await service.mutate({ kind: 'delete', notificationId: deleting.id }) && active.current) setDeleting(null);
  }
  if (destination) return <Destination target={destination} />;
  return <StudioShell page="notifications" i18n={d.i18n}><main className="studio-main studio-notification-main">
    <header className="studio-header"><div><h1>{d.i18n.t('notification.heading')}</h1>{state.loaded && <p>{d.i18n.t('notification.unreadCount', { count: state.unreadCount })}</p>}</div><div className="studio-form-actions"><button disabled={busy} onClick={() => void service.refresh()}>{d.i18n.t('common.refresh')}</button><button disabled={busy || !state.loaded || !state.unreadCount} onClick={() => void service.mutate({ kind: 'read-all' })}>{d.i18n.t('notification.markAllRead')}</button></div></header>
    {state.loading && <p role="status">{d.i18n.t('notification.loading')}</p>}
    {state.error && <div role="alert"><p>{d.i18n.t(state.error === 'history' ? 'notification.error' : 'notification.mutationError')}</p>{state.error === 'history' && <button disabled={busy} onClick={() => void service.refresh()}>{d.i18n.t('common.retry')}</button>}</div>}
    {state.loaded && !state.items.length && <section className="studio-settings-card"><h2>{d.i18n.t('notification.empty')}</h2><p>{d.i18n.t('notification.emptyDescription')}</p></section>}
    {(['actionable', 'informational'] as const).map(section => {
      const items = state.items.filter(item => item.presentation === section).sort((a, b) => b.createdAt - a.createdAt);
      return items.length ? <section key={section} className="studio-notification-section"><h2>{d.i18n.t(section === 'actionable' ? 'notification.actionableHeading' : 'notification.informationalHeading')}</h2><ul className="studio-notification-list">{items.map(item => <li key={item.id} data-unread={!item.read}>
        <button className="studio-notification-open" disabled={busy} onClick={() => void open(item)} aria-label={d.i18n.t('notification.rowAccessibility', { message: item.message, state: d.i18n.t(item.read ? 'notification.read' : 'notification.unread'), date: d.i18n.date(item.createdAt, { dateStyle: 'medium', timeStyle: 'short' }) })}><span className="studio-notification-message">{!item.read && <span className="studio-unread-dot" aria-hidden="true" />}{item.message}</span><time dateTime={new Date(item.createdAt).toISOString()}>{d.i18n.date(item.createdAt, { dateStyle: 'medium', timeStyle: 'short' })}</time></button>
        {item.reportTarget && <ReportAction menu target={item.reportTarget} dependencies={d} />}<button disabled={busy} onClick={() => setDeleting(item)}>{d.i18n.t('notification.delete')}</button>
      </li>)}</ul></section> : null;
    })}
    {openReminder?.kind === 'reminder' && <ConfirmationDialog title={d.i18n.t('notification.goalReminderChooserTitle')} busy={busy} hideCancel cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t('common.done')} cancel={() => setOpenReminderId(null)} confirm={() => setOpenReminderId(null)}><ul>{openReminder.paths.map(path => <li key={path.id}><Link to="/paths/$pathId" params={{ pathId: path.id }}>{path.name}</Link></li>)}</ul></ConfirmationDialog>}
    {state.nextCursor && <button disabled={busy} onClick={() => void service.loadMore()}>{d.i18n.t('notification.loadMore')}</button>}
    {deleting && <ConfirmationDialog title={d.i18n.t('notification.delete')} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t('notification.delete')} cancel={() => setDeleting(null)} confirm={() => void remove()}><p>{deleting.message}</p><p>{d.i18n.t('studio.notifications.deleteExplanation')}</p>{state.error === 'mutation' && <p role="alert">{d.i18n.t('notification.mutationError')}</p>}</ConfirmationDialog>}
  </main></StudioShell>;
}
