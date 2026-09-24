import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { SocialRouteIntent } from '../social-route-recovery';

export type SocialRouteRecoveryState = 'loading' | 'offline' | 'unavailable';

export type SocialRouteRecoveryPresentation = {
  onGoFollowing: () => void;
  onGoHome: () => void;
  onRetry?: () => void;
  state: SocialRouteRecoveryState;
  target: SocialRouteIntent;
};

type Published = SocialRouteRecoveryPresentation & { owner: object };
let current: Published | undefined;
const listeners = new Set<() => void>();
const emit = () => { for (const listener of listeners) listener(); };

export function socialRouteTargetKey(target: SocialRouteIntent) {
  return target.routeKey;
}

export function SocialRouteRecoverySource(props: SocialRouteRecoveryPresentation) {
  const owner = useRef({});
  useEffect(() => {
    current = { ...props, owner: owner.current };
    emit();
    return () => {
      if (current?.owner === owner.current) { current = undefined; emit(); }
    };
  }, []);
  useEffect(() => {
    if (current?.owner === owner.current) { current = { ...props, owner: owner.current }; emit(); }
  }, [props]);
  return null;
}

export function useSocialRouteRecovery(target: SocialRouteIntent) {
  const key = socialRouteTargetKey(target);
  return useSyncExternalStore(
    (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    () => current && socialRouteTargetKey(current.target) === key ? current : undefined,
    () => undefined,
  );
}
