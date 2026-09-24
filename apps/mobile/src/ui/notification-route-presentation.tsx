import { useEffect, useRef, useSyncExternalStore, type ReactNode } from 'react';
import type { NotificationJourneyIntent } from '../notification-journey-route-recovery';

export type NotificationJourneyRecoveryState = 'loading' | 'offline' | 'unavailable';
export type NotificationJourneyRecoveryPresentation = Readonly<{
  goHome: () => void;
  intent: NotificationJourneyIntent;
  retry: () => void;
  sessionKey: string;
  state: NotificationJourneyRecoveryState;
}>;

export type NotificationRoutePresentation = {
  busy: boolean;
  canMarkAllRead: boolean;
  content: ReactNode;
  invitationCount?: number;
  markAllRead: () => void;
  openInvitations: () => void;
  refresh: () => void;
  refreshing: boolean;
};

export type InvitationRoutePresentation = {
  content: ReactNode;
};

type PublishedRoute<T> = T & {
  dismiss: () => void;
  owner: object;
};

let notificationPresentation: PublishedRoute<NotificationRoutePresentation> | null | undefined;
let invitationPresentation: PublishedRoute<InvitationRoutePresentation> | null | undefined;
let notificationRecovery: (() => void) | undefined;
let invitationRecovery: (() => void) | undefined;
const notificationListeners = new Set<() => void>();
const invitationListeners = new Set<() => void>();
const journeyRecoveryListeners = new Set<() => void>();
let journeyRecoveryPresentation: (NotificationJourneyRecoveryPresentation & { owner: object }) | null = null;

function publish<T>(
  owner: object,
  value: T,
  dismiss: () => void,
): PublishedRoute<T> {
  return { ...value, dismiss, owner };
}

function emit(listeners: Set<() => void>) {
  for (const listener of listeners) listener();
}

export function NotificationJourneyRecoverySource({
  intent,
  onHome,
  onRetry,
  sessionKey,
  state,
}: {
  intent: NotificationJourneyIntent;
  onHome: () => void;
  onRetry: () => void;
  sessionKey: string;
  state: NotificationJourneyRecoveryState;
}) {
  const owner = useRef({});
  const homeRef = useRef(onHome);
  const retryRef = useRef(onRetry);
  homeRef.current = onHome;
  retryRef.current = onRetry;

  useEffect(() => {
    journeyRecoveryPresentation = {
      goHome: () => homeRef.current(),
      intent,
      owner: owner.current,
      retry: () => retryRef.current(),
      sessionKey,
      state,
    };
    emit(journeyRecoveryListeners);
    return () => {
      if (journeyRecoveryPresentation?.owner === owner.current) {
        journeyRecoveryPresentation = null;
        emit(journeyRecoveryListeners);
      }
    };
  }, [intent.routeKey, sessionKey]);

  useEffect(() => {
    if (journeyRecoveryPresentation?.owner !== owner.current) return;
    journeyRecoveryPresentation = {
      goHome: () => homeRef.current(),
      intent,
      owner: owner.current,
      retry: () => retryRef.current(),
      sessionKey,
      state,
    };
    emit(journeyRecoveryListeners);
  }, [intent, sessionKey, state]);

  return null;
}

export function NotificationRouteSource({
  busy,
  canMarkAllRead,
  children,
  invitationCount,
  markAllRead,
  onDismiss,
  onUnavailable,
  openInvitations,
  refresh,
  refreshing,
}: Omit<NotificationRoutePresentation, 'content'> & {
  children: ReactNode;
  onDismiss: () => void;
  onUnavailable: () => void;
}) {
  const owner = useRef({});
  const dismissRef = useRef(onDismiss);
  const unavailableRef = useRef(onUnavailable);
  dismissRef.current = onDismiss;
  unavailableRef.current = onUnavailable;
  const value = {
    busy,
    canMarkAllRead,
    content: children,
    invitationCount,
    markAllRead,
    openInvitations,
    refresh,
    refreshing,
  };

  useEffect(() => {
    notificationRecovery = () => unavailableRef.current();
    notificationPresentation = publish(
      owner.current,
      value,
      () => dismissRef.current(),
    );
    emit(notificationListeners);
    return () => {
      if (notificationPresentation?.owner === owner.current) {
        notificationPresentation = null;
        emit(notificationListeners);
      }
    };
  }, []);

  useEffect(() => {
    if (notificationPresentation?.owner !== owner.current) return;
    notificationPresentation = publish(
      owner.current,
      value,
      () => dismissRef.current(),
    );
    emit(notificationListeners);
  }, [busy, canMarkAllRead, children, invitationCount, markAllRead, openInvitations, refresh, refreshing]);

  return null;
}

export function InvitationRouteSource({
  children,
  onDismiss,
  onUnavailable,
}: {
  children: ReactNode;
  onDismiss: () => void;
  onUnavailable: () => void;
}) {
  const owner = useRef({});
  const dismissRef = useRef(onDismiss);
  const unavailableRef = useRef(onUnavailable);
  dismissRef.current = onDismiss;
  unavailableRef.current = onUnavailable;

  useEffect(() => {
    invitationRecovery = () => unavailableRef.current();
    invitationPresentation = publish(
      owner.current,
      { content: children },
      () => dismissRef.current(),
    );
    emit(invitationListeners);
    return () => {
      if (invitationPresentation?.owner === owner.current) {
        invitationPresentation = null;
        emit(invitationListeners);
      }
    };
  }, []);

  useEffect(() => {
    if (invitationPresentation?.owner !== owner.current) return;
    invitationPresentation = publish(
      owner.current,
      { content: children },
      () => dismissRef.current(),
    );
    emit(invitationListeners);
  }, [children]);

  return null;
}

function subscribe(listeners: Set<() => void>, listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useNotificationRoutePresentation(): NotificationRoutePresentation | null | undefined {
  return useSyncExternalStore(
    (listener) => subscribe(notificationListeners, listener),
    () => notificationPresentation,
    () => null,
  );
}

export function useInvitationRoutePresentation(): InvitationRoutePresentation | null | undefined {
  return useSyncExternalStore(
    (listener) => subscribe(invitationListeners, listener),
    () => invitationPresentation,
    () => null,
  );
}

export function useNotificationJourneyRecovery(
  routeKey: NotificationJourneyIntent['routeKey'],
): NotificationJourneyRecoveryPresentation | null {
  return useSyncExternalStore(
    (listener) => subscribe(journeyRecoveryListeners, listener),
    () => journeyRecoveryPresentation?.intent.routeKey === routeKey ? journeyRecoveryPresentation : null,
    () => null,
  );
}

export function dismissNotificationRoute() {
  const presentation = notificationPresentation;
  if (!presentation) return;
  notificationPresentation = null;
  emit(notificationListeners);
  presentation.dismiss();
}

export function dismissInvitationRoute() {
  const presentation = invitationPresentation;
  if (!presentation) return;
  invitationPresentation = null;
  emit(invitationListeners);
  presentation.dismiss();
}

export function prepareNotificationRoute() {
  notificationPresentation = undefined;
  emit(notificationListeners);
}

export function prepareInvitationRoute() {
  invitationPresentation = undefined;
  emit(invitationListeners);
}

export function recoverNotificationRoute() {
  const recover = notificationRecovery;
  notificationRecovery = undefined;
  recover?.();
}

export function recoverInvitationRoute() {
  const recover = invitationRecovery;
  invitationRecovery = undefined;
  recover?.();
}
