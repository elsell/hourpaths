import { useEffect, useRef, useSyncExternalStore, type ReactNode } from 'react';

export type NativeChildRoutePresentation = {
  content: ReactNode;
  dismissible: boolean;
  grouped: boolean;
  onRefresh?: () => void;
  refreshing: boolean;
  title: string;
};

type PublishedNativeChildRoute = NativeChildRoutePresentation & {
  dismiss: () => void;
  owner: object;
};

const presentations: Record<string, PublishedNativeChildRoute | undefined> = Object.create(null);
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function activityHistoryRouteKey(pathID: string) {
  return `path:${pathID}:history`;
}

export function activityDetailRouteKey(pathID: string, activityID: string) {
  return `path:${pathID}:activity:${activityID}`;
}

export function pathMembersRouteKey(pathID: string) {
  return `path:${pathID}:members`;
}

export function pathMemberRemovalRouteKey(pathID: string, userID: string) {
  return `path:${pathID}:member:${userID}:removal`;
}

export function pathNudgeSettingsRouteKey(pathID: string) {
  return `path:${pathID}:nudge-settings`;
}

export function NativeChildRouteSource({
  children,
  dismissible = true,
  grouped = false,
  onDismiss,
  onRefresh,
  refreshing = false,
  routeKey,
  title,
}: {
  children: ReactNode;
  dismissible?: boolean;
  grouped?: boolean;
  onDismiss: () => void;
  onRefresh?: () => void;
  refreshing?: boolean;
  routeKey: string;
  title: string;
}) {
  const owner = useRef({});
  const dismissRef = useRef(onDismiss);
  dismissRef.current = onDismiss;

  useEffect(() => {
    const presentation: PublishedNativeChildRoute = {
      content: children,
      dismiss: () => dismissRef.current(),
      dismissible,
      grouped,
      onRefresh,
      owner: owner.current,
      refreshing,
      title,
    };
    presentations[routeKey] = presentation;
    emitChange();
    return () => {
      if (presentations[routeKey]?.owner === owner.current) {
        delete presentations[routeKey];
        emitChange();
      }
    };
  }, [routeKey]);

  useEffect(() => {
    if (presentations[routeKey]?.owner !== owner.current) return;
    presentations[routeKey] = {
      content: children,
      dismiss: () => dismissRef.current(),
      dismissible,
      grouped,
      onRefresh,
      owner: owner.current,
      refreshing,
      title,
    };
    emitChange();
  }, [children, dismissible, grouped, onRefresh, refreshing, routeKey, title]);

  return null;
}

export function useNativeChildRoutePresentation(routeKey: string): NativeChildRoutePresentation | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => presentations[routeKey] ?? null,
    () => null,
  );
}

export function dismissNativeChildRoute(routeKey: string) {
  const presentation = presentations[routeKey];
  if (!presentation) return;
  delete presentations[routeKey];
  emitChange();
  presentation.dismiss();
}

export function withNativeChildRouteDismissalAllowed<T extends { dismissible: boolean }>(presentation: T): T {
  return { ...presentation, dismissible: true };
}

export function allowNativeChildRouteDismissal(routeKey: string) {
  const presentation = presentations[routeKey];
  if (!presentation) return;
  presentations[routeKey] = withNativeChildRouteDismissalAllowed(presentation);
  emitChange();
}
