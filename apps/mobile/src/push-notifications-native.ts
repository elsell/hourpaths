import * as Crypto from 'expo-crypto';
import * as Linking from 'expo-linking';
import * as Notifications from 'expo-notifications';
import * as SecureStore from 'expo-secure-store';
import { AppState, Platform } from 'react-native';
import type { ForegroundNotificationOutcome } from '@hourpaths/client-core';
import {
  createPushRegistrationCoordinator,
  handleNotificationTap,
  loadOrCreateInstallationID,
  nativeForegroundPresentation,
  nativeForegroundPresentationBeforeDeadline,
  parsePendingPushDeregistration,
  pendingPushDeregistrationStorageKey,
  pushNotificationID,
  type NotificationDestination,
  type PendingPushDeregistration,
  type PushPermission,
  type PushRegistration,
  type PushRegistrationWithToken,
} from './push-notifications';

function normalizedPermission(
  permission: Notifications.NotificationPermissionsStatus,
): PushPermission {
  const iosStatus = permission.ios?.status;
  return {
    granted: permission.granted
      || iosStatus === Notifications.IosAuthorizationStatus.PROVISIONAL
      || iosStatus === Notifications.IosAuthorizationStatus.EPHEMERAL,
    canAskAgain: permission.canAskAgain,
  };
}

export async function getNativePushPermission(): Promise<PushPermission> {
  return normalizedPermission(await Notifications.getPermissionsAsync());
}

export function nativePushPlatform(): 'ios' | 'android' | null {
  return Platform.OS === 'ios' || Platform.OS === 'android' ? Platform.OS : null;
}

export function subscribeNativeAppActive(listener: () => void): () => void {
  const subscription = AppState.addEventListener('change', (state) => {
    if (state === 'active') listener();
  });
  return () => subscription.remove();
}

export async function ensureNativePushChannel(channelName: string): Promise<void> {
  if (Platform.OS !== 'android') return;
  await Notifications.setNotificationChannelAsync('hourpaths', {
    importance: Notifications.AndroidImportance.DEFAULT,
    name: channelName,
  });
}

export async function requestNativePushPermission(channelName: string): Promise<PushPermission> {
  await ensureNativePushChannel(channelName);
  return normalizedPermission(await Notifications.requestPermissionsAsync({
    ios: {
      allowAlert: true,
      allowBadge: true,
      allowSound: true,
    },
  }));
}

export async function openNativePushSettings(): Promise<void> {
  await Linking.openSettings();
}

export async function setNativeNotificationBadge(unreadCount: number): Promise<boolean> {
  if (!Number.isSafeInteger(unreadCount) || unreadCount < 0) {
    throw new Error('notification_badge_count_invalid');
  }
  return Notifications.setBadgeCountAsync(unreadCount);
}

export async function nativeInstallationID(): Promise<string> {
  return loadOrCreateInstallationID(
    {
      read: (key) => SecureStore.getItemAsync(key),
      write: (key, value) => SecureStore.setItemAsync(key, value),
    },
    () => Crypto.randomUUID(),
  );
}

export async function stageNativePushDeregistration(
  pending: PendingPushDeregistration,
): Promise<void> {
  await SecureStore.setItemAsync(
    pendingPushDeregistrationStorageKey,
    JSON.stringify(pending),
  );
}

export async function loadNativePushDeregistration(): Promise<PendingPushDeregistration | null> {
  return parsePendingPushDeregistration(
    await SecureStore.getItemAsync(pendingPushDeregistrationStorageKey),
  );
}

export async function clearNativePushDeregistration(): Promise<void> {
  await SecureStore.deleteItemAsync(pendingPushDeregistrationStorageKey);
}

export function createNativePushRegistrationCoordinator({
  projectId,
  channelName,
  register,
  deregister,
}: {
  projectId: string;
  channelName: string;
  register(registration: PushRegistrationWithToken): Promise<void>;
  deregister(registration: PushRegistration): Promise<void>;
}) {
  return createPushRegistrationCoordinator({
    installationID: nativeInstallationID,
    permission: getNativePushPermission,
    requestPermission: () => requestNativePushPermission(channelName),
    pushToken: async () => {
      await ensureNativePushChannel(channelName);
      return (await Notifications.getExpoPushTokenAsync({ projectId })).data;
    },
    register,
    deregister,
  });
}

export type NativeNotificationLifecyclePorts = {
  foreground(notificationID: string): Promise<ForegroundNotificationOutcome>;
  resolve(notificationID: string): Promise<NotificationDestination | null>;
  markRead(notificationID: string): Promise<void>;
  navigate(destination: NotificationDestination): Promise<void> | void;
  unavailable(): void;
  failure(cause: unknown): void;
};

export function installNativeNotificationLifecycle(
  ports: NativeNotificationLifecyclePorts,
): () => void {
  const observedEffects = new WeakSet<Promise<void>>();
  const handleForeground = async (data: Readonly<Record<string, unknown>>) => {
    const notificationID = pushNotificationID(data);
    if (!notificationID) return null;
    const outcome = await ports.foreground(notificationID);
    if (!observedEffects.has(outcome.effects)) {
      observedEffects.add(outcome.effects);
      void outcome.effects.catch(ports.failure);
    }
    return outcome;
  };
  Notifications.setNotificationHandler({
    handleNotification: async (notification) => {
      try {
        const presentation = handleForeground(notification.request.content.data ?? {})
          .then((outcome) => outcome?.presentation ?? 'quiet');
        return await nativeForegroundPresentationBeforeDeadline(
          presentation,
          new Promise<void>((resolve) => { setTimeout(resolve, 2_500); }),
        );
      } catch (cause) {
        ports.failure(cause);
        return nativeForegroundPresentation('quiet');
      }
    },
  });

  const handleResponse = async (response: Notifications.NotificationResponse) => {
    if (response.actionIdentifier !== Notifications.DEFAULT_ACTION_IDENTIFIER) return;
    await handleNotificationTap(response.notification.request.content.data ?? {}, ports);
  };
  const received = Notifications.addNotificationReceivedListener((notification) => {
    void handleForeground(notification.request.content.data ?? {}).catch(ports.failure);
  });
  const responded = Notifications.addNotificationResponseReceivedListener((response) => {
    void handleResponse(response).catch(ports.failure);
  });

  const lastResponse = Notifications.getLastNotificationResponse();
  if (lastResponse) {
    void handleResponse(lastResponse)
      .then(() => Notifications.clearLastNotificationResponse())
      .catch(ports.failure);
  }

  return () => {
    received.remove();
    responded.remove();
  };
}
