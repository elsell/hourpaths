import type { ReportingRepository } from '@hourpaths/client-core';
import type { GoalRemindersRepository } from '@hourpaths/client-core';
import type { WeekStartPreferenceRepository } from '@hourpaths/client-core';
import type { ProfilePictureRepository } from '@hourpaths/client-core';
import { providerSettingsLifetime } from '@hourpaths/client-core';
import type { ProfilePrivacyRepository } from '@hourpaths/client-core';
import type { ProfileEditingRepository } from '@hourpaths/client-core';
import type { TimerSubscriptionsRepository } from '@hourpaths/client-core';
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
  weekStart?: WeekStartPreferenceRepository;
  profilePrivacy?: ProfilePrivacyRepository;
  profileEditing?: ProfileEditingRepository;
  profilePicture?: ProfilePictureRepository;
  pickProfilePicture?: () => Promise<string | null>;
  profileOperationId?: () => string;
  timerSubscriptions: TimerSubscriptionsRepository;
  goalReminders: GoalRemindersRepository;
  reporting: ReportingRepository;
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
  weekStart,
  timerSubscriptions,
  goalReminders,
  reporting,
  profilePrivacy,
  profileEditing,
  profilePicture,
  pickProfilePicture,
  profileOperationId,
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
  const weekStartRef = useRef(weekStart); weekStartRef.current = weekStart;
  const profilePrivacyRef = useRef(profilePrivacy);
  profilePrivacyRef.current = profilePrivacy;
  const profilePictureRef = useRef(profilePicture);
  profilePictureRef.current = profilePicture;
  const profileEditingRef = useRef(profileEditing);
  profileEditingRef.current = profileEditing;
  const reportingRef = useRef(reporting); reportingRef.current = reporting;
  const goalRemindersRef = useRef(goalReminders);
  goalRemindersRef.current = goalReminders;
  const timerSubscriptionsRef = useRef(timerSubscriptions);
  timerSubscriptionsRef.current = timerSubscriptions;
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
    const assertProfileOwner = () => {
      // A timer count or name refresh republishes the same account presentation.
      // Reject only loss/replacement of its owning settings journey.
      if (currentPresentation?.sessionKey !== sessionKey || !currentPresentation.isCurrent()) throw new Error('settings_presentation_superseded');
    };
    const presentation: SettingsPresentation = {
      weekStart: weekStart ? {
        read: async signal => { assertActive(); const value = await weekStartRef.current!.read(signal); assertProfileOwner(); return value; },
        save: async (value, key, signal) => { assertActive(); const saved = await weekStartRef.current!.save(value, key, signal); assertProfileOwner(); return saved; },
      } : undefined,
      profileOperationId,
      pickProfilePicture,
      profilePicture: profilePicture ? {
        read: async () => { assertActive(); const value = await profilePictureRef.current!.read(); assertProfileOwner(); return value; },
        preview: async image => { assertActive(); const value = await profilePictureRef.current!.preview(image); assertProfileOwner(); return value; },
        save: async (value, key) => { assertActive(); const saved = await profilePictureRef.current!.save(value, key); assertProfileOwner(); return saved; },
      } : undefined,
      profilePrivacy: profilePrivacy ? {
        read: async () => { assertActive(); const value = await profilePrivacyRef.current!.read(); assertProfileOwner(); return value; },
        save: async (value, visibility, key) => { assertActive(); const saved = await profilePrivacyRef.current!.save(value, visibility, key); assertProfileOwner(); return saved; },
      } : undefined,
      profileEditing: profileEditing ? {
        read: async () => { assertActive(); const value = await profileEditingRef.current!.read(); assertProfileOwner(); return value; },
        save: async (value, key) => { assertActive(); const saved = await profileEditingRef.current!.save(value, key); assertProfileOwner(); return saved; },
      } : undefined,
      reporting: { submit: async (draft, key, signal) => { assertActive(); const receipt = await reportingRef.current.submit(draft, key, signal); assertActive(); return receipt; } },
      goalReminders: {
        get: subject => { assertActive(); return goalRemindersRef.current.get(subject); },
        update: (subject, value, key) => { assertActive(); return goalRemindersRef.current.update(subject, value, key); },
      },
      timerSubscriptions: {
        get: subject => { assertActive(); return timerSubscriptionsRef.current.get(subject); },
        update: (subject, value, key) => { assertActive(); return timerSubscriptionsRef.current.update(subject, value, key); },
      },
      providers: providers ? providerSettingsLifetime(
        () => providersRef.current!,
        () => active && isCurrentRef.current(),
      ) : undefined,
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
