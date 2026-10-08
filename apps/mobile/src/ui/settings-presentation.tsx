import type { ProviderSettingsService } from '@hourpaths/client-core';
import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { PushPermission } from '../push-notifications';
import type { InteractionSettings } from '../interaction-settings';
import type { FrozenTimeZoneChangeIntent, TimeZonePreference } from '../time-zone-settings';
import type { NotificationChannelPreference } from '@hourpaths/client-core';

export type SignOutTimerChoice = 'confirmed_no_timers' | 'keep_running' | 'stop_and_save';
export type SignOutPresentationResult =
  | Readonly<{ kind: 'choice_required'; runningTimerCount: number }>
  | Readonly<{ kind: 'failed' | 'signed_out' | 'superseded' }>;

export type SettingsPresentation = {
  providers?: ProviderSettingsService;
  deleteAccount?: () => void;
  displayName: string;
  email: string;
  sessionKey: string;
  isCurrent: () => boolean;
  getInteractionSettings: () => Promise<InteractionSettings>;
  getNotificationChannels: () => Promise<NotificationChannelPreference[]>;
  getConfiguredTimeZone: () => Promise<TimeZonePreference>;
  runningTimerCount: number;
  synchronizePushPermission: (requestPermission: boolean) => Promise<PushPermission>;
  signOut: (resolution: SignOutTimerChoice) => Promise<SignOutPresentationResult>;
  updateInteractionSettings: (settings: InteractionSettings, idempotencyKey: string) => Promise<InteractionSettings>;
  updateNotificationChannel: (body: NotificationChannelPreference, idempotencyKey: string) => Promise<NotificationChannelPreference>;
  updateConfiguredTimeZone: (intent: FrozenTimeZoneChangeIntent) => Promise<TimeZonePreference>;
};

let currentPresentation: SettingsPresentation | null = null;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function SettingsPresentationSource({
  providers,
  deleteAccount,
  displayName,
  email,
  sessionKey,
  isCurrent,
  getInteractionSettings,
  getNotificationChannels,
  getConfiguredTimeZone,
  runningTimerCount,
  synchronizePushPermission,
  signOut,
  updateInteractionSettings,
  updateNotificationChannel,
  updateConfiguredTimeZone,
}: SettingsPresentation) {
  const providersRef = useRef(providers);
  providersRef.current = providers;
  const deleteAccountRef = useRef(deleteAccount);
  deleteAccountRef.current = deleteAccount;
  const getConfiguredTimeZoneRef = useRef(getConfiguredTimeZone);
  const isCurrentRef = useRef(isCurrent);
  const getInteractionSettingsRef = useRef(getInteractionSettings);
  const getNotificationChannelsRef = useRef(getNotificationChannels);
  const signOutRef = useRef(signOut);
  const synchronizePushPermissionRef = useRef(synchronizePushPermission);
  const updateInteractionSettingsRef = useRef(updateInteractionSettings);
  const updateNotificationChannelRef = useRef(updateNotificationChannel);
  const updateConfiguredTimeZoneRef = useRef(updateConfiguredTimeZone);
  getConfiguredTimeZoneRef.current = getConfiguredTimeZone;
  isCurrentRef.current = isCurrent;
  getInteractionSettingsRef.current = getInteractionSettings;
  getNotificationChannelsRef.current = getNotificationChannels;
  signOutRef.current = signOut;
  synchronizePushPermissionRef.current = synchronizePushPermission;
  updateInteractionSettingsRef.current = updateInteractionSettings;
  updateNotificationChannelRef.current = updateNotificationChannel;
  updateConfiguredTimeZoneRef.current = updateConfiguredTimeZone;
  useEffect(() => {
    let active = true;
    const assertActive = () => {
      if (!active || !isCurrentRef.current()) throw new Error('settings_presentation_superseded');
    };
    const presentation = {
      providers: providers ? {
        owner: () => { assertActive(); return providersRef.current!.owner(); },
        list: () => { assertActive(); return providersRef.current!.list(); },
        link: (provider: 'google' | 'apple') => { assertActive(); return providersRef.current!.link(provider); },
        unlink: (provider: 'google' | 'apple', owner: string) => { assertActive(); return providersRef.current!.unlink(provider, owner); },
      } : undefined,
      deleteAccount: () => { assertActive(); deleteAccountRef.current?.(); },
      displayName,
      email,
      sessionKey,
      isCurrent: () => active && isCurrentRef.current(),
      getInteractionSettings: () => { assertActive(); return getInteractionSettingsRef.current(); },
      getNotificationChannels: () => { assertActive(); return getNotificationChannelsRef.current(); },
      getConfiguredTimeZone: () => { assertActive(); return getConfiguredTimeZoneRef.current(); },
      runningTimerCount,
      signOut: (resolution: SignOutTimerChoice) => {
        if (!active || !isCurrentRef.current()) return Promise.resolve({ kind: 'superseded' } as const);
        return signOutRef.current(resolution);
      },
      synchronizePushPermission: (requestPermission: boolean) => {
        assertActive();
        return synchronizePushPermissionRef.current(requestPermission);
      },
      updateInteractionSettings: (settings: InteractionSettings, idempotencyKey: string) => {
        assertActive();
        return updateInteractionSettingsRef.current(settings, idempotencyKey);
      },
      updateNotificationChannel: (body: NotificationChannelPreference, idempotencyKey: string) => {
        assertActive();
        return updateNotificationChannelRef.current(body, idempotencyKey);
      },
      updateConfiguredTimeZone: (intent: FrozenTimeZoneChangeIntent) => {
        assertActive();
        return updateConfiguredTimeZoneRef.current(intent);
      },
    };
    currentPresentation = presentation;
    emitChange();
    return () => {
      active = false;
      if (currentPresentation === presentation) {
        currentPresentation = null;
        emitChange();
      }
    };
  }, [displayName, email, runningTimerCount, sessionKey]);
  return null;
}

export function useSettingsPresentation(): SettingsPresentation | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => currentPresentation,
    () => null,
  );
}
