import { createContext, useContext, useSyncExternalStore } from 'react';
import { Link } from '@tanstack/react-router';
import type { Translator } from '@hourpaths/i18n';
import type { Notifications } from '../notifications/ports/notifications';
export const NotificationsContext = createContext<Notifications | null>(null);
export function NotificationNavigation({ i18n, current }: { i18n: Translator; current: boolean }) {
  const service = useContext(NotificationsContext);
  return service ? <NotificationBadge service={service} i18n={i18n} current={current} /> : null;
}
function NotificationBadge({ service, i18n, current }: { service: Notifications; i18n: Translator; current: boolean }) {
  const state = useSyncExternalStore(service.subscribe, service.snapshot, service.snapshot);
  return <Link to="/notifications" aria-current={current ? 'page' : undefined}>{i18n.t('notification.heading')}{state.loaded && state.unreadCount > 0 && <span className="studio-notification-count" aria-label={i18n.t('notification.unreadCount', { count: state.unreadCount })}>{i18n.number(state.unreadCount)}</span>}</Link>;
}
