import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { PushPermission } from '../push-notifications';
import type { InteractionSettings } from '../interaction-settings';
import type { FrozenTimeZoneChangeIntent, TimeZonePreference } from '../time-zone-settings';
import type { NudgeChannelPreference, NudgeChannelUpdateBody } from '@hourpaths/client-core';

export type SignOutTimerChoice = 'confirmed_no_timers' | 'keep_running' | 'stop_and_save';
export type SignOutPresentationResult =
  | Readonly<{ kind: 'choice_required'; runningTimerCount: number }>
  | Readonly<{ kind: 'failed' | 'signed_out' | 'superseded' }>;

export type SettingsPresentation = {
  displayName: string;
  email: string;
  sessionKey: string;
  isCurrent: () => boolean;
  getInteractionSettings: () => Promise<InteractionSettings>;
  getNudgeChannelPreference: () => Promise<NudgeChannelPreference>;
  getConfiguredTimeZone: () => Promise<TimeZonePreference>;
  runningTimerCount: number;
  synchronizePushPermission: (requestPermission: boolean) => Promise<PushPermission>;
  signOut: (resolution: SignOutTimerChoice) => Promise<SignOutPresentationResult>;
  updateInteractionSettings: (settings: InteractionSettings, idempotencyKey: string) => Promise<InteractionSettings>;
  updateNudgeChannelPreference: (body: NudgeChannelUpdateBody, idempotencyKey: string) => Promise<NudgeChannelPreference>;
  updateConfiguredTimeZone: (intent: FrozenTimeZoneChangeIntent) => Promise<TimeZonePreference>;
};

let currentPresentation: SettingsPresentation | null = null;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function SettingsPresentationSource({
  displayName,
  email,
  sessionKey,
  isCurrent,
  getInteractionSettings,
  getNudgeChannelPreference,
  getConfiguredTimeZone,
  runningTimerCount,
  synchronizePushPermission,
  signOut,
  updateInteractionSettings,
  updateNudgeChannelPreference,
  updateConfiguredTimeZone,
}: SettingsPresentation) {
  const getConfiguredTimeZoneRef = useRef(getConfiguredTimeZone);
  const isCurrentRef = useRef(isCurrent);
  const getInteractionSettingsRef = useRef(getInteractionSettings);
  const getNudgeChannelPreferenceRef = useRef(getNudgeChannelPreference);
  const signOutRef = useRef(signOut);
  const synchronizePushPermissionRef = useRef(synchronizePushPermission);
  const updateInteractionSettingsRef = useRef(updateInteractionSettings);
  const updateNudgeChannelPreferenceRef = useRef(updateNudgeChannelPreference);
  const updateConfiguredTimeZoneRef = useRef(updateConfiguredTimeZone);
  getConfiguredTimeZoneRef.current = getConfiguredTimeZone;
  isCurrentRef.current = isCurrent;
  getInteractionSettingsRef.current = getInteractionSettings;
  getNudgeChannelPreferenceRef.current = getNudgeChannelPreference;
  signOutRef.current = signOut;
  synchronizePushPermissionRef.current = synchronizePushPermission;
  updateInteractionSettingsRef.current = updateInteractionSettings;
  updateNudgeChannelPreferenceRef.current = updateNudgeChannelPreference;
  updateConfiguredTimeZoneRef.current = updateConfiguredTimeZone;
  useEffect(() => {
    let active = true;
    const assertActive = () => {
      if (!active || !isCurrentRef.current()) throw new Error('settings_presentation_superseded');
    };
    const presentation = {
      displayName,
      email,
      sessionKey,
      isCurrent: () => active && isCurrentRef.current(),
      getInteractionSettings: () => { assertActive(); return getInteractionSettingsRef.current(); },
      getNudgeChannelPreference: () => { assertActive(); return getNudgeChannelPreferenceRef.current(); },
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
      updateNudgeChannelPreference: (body: NudgeChannelUpdateBody, idempotencyKey: string) => {
        assertActive();
        return updateNudgeChannelPreferenceRef.current(body, idempotencyKey);
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
