import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { SettingsJourneyIntent } from '../settings-journey-route-recovery';

export type SettingsJourneyRecoveryState = 'loading' | 'offline' | 'unavailable';
export type SettingsJourneyRecoveryPresentation = Readonly<{
  goHome: () => void;
  intent: SettingsJourneyIntent;
  retry: () => void;
  sessionKey: string;
  state: SettingsJourneyRecoveryState;
}>;

let current: (SettingsJourneyRecoveryPresentation & { owner: object }) | null = null;
const listeners = new Set<() => void>();
const emit = () => { for (const listener of listeners) listener(); };

export function SettingsJourneyRecoverySource({
  intent,
  onHome,
  onRetry,
  sessionKey,
  state,
}: {
  intent: SettingsJourneyIntent;
  onHome: () => void;
  onRetry: () => void;
  sessionKey: string;
  state: SettingsJourneyRecoveryState;
}) {
  const owner = useRef({});
  const homeRef = useRef(onHome);
  const retryRef = useRef(onRetry);
  homeRef.current = onHome;
  retryRef.current = onRetry;

  useEffect(() => {
    current = {
      goHome: () => homeRef.current(),
      intent,
      owner: owner.current,
      retry: () => retryRef.current(),
      sessionKey,
      state,
    };
    emit();
    return () => {
      if (current?.owner === owner.current) {
        current = null;
        emit();
      }
    };
  }, [intent.routeKey, sessionKey]);

  useEffect(() => {
    if (current?.owner !== owner.current) return;
    current = { ...current, intent, sessionKey, state };
    emit();
  }, [intent, sessionKey, state]);
  return null;
}

export function useSettingsJourneyRecovery(
  routeKey: SettingsJourneyIntent['routeKey'],
): SettingsJourneyRecoveryPresentation | null {
  return useSyncExternalStore(
    (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    () => current?.intent.routeKey === routeKey ? current : null,
    () => null,
  );
}
