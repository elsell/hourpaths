import type { ForegroundNotificationPresentation } from '@hourpaths/client-core';

export const pushInstallationStorageKey = 'push_installation_id';
export const pendingPushDeregistrationStorageKey = 'push_pending_deregistration';

export type PushPermission = {
  granted: boolean;
  canAskAgain: boolean;
};

export type PushRegistration = {
  accountID: string;
  installationID: string;
};

export type PushRegistrationWithToken = PushRegistration & {
  pushToken: string;
};

export type PendingPushDeregistration = PushRegistration & {
  credential: string;
};

export type NativeForegroundPresentation = Readonly<{
  shouldPlaySound: boolean;
  shouldSetBadge: false;
  shouldShowBanner: boolean;
  shouldShowList: boolean;
}>;

export function nativeForegroundPresentation(
  presentation: ForegroundNotificationPresentation,
): NativeForegroundPresentation {
  const actionable = presentation === 'actionable';
  return Object.freeze({
    shouldPlaySound: actionable,
    shouldSetBadge: false as const,
    shouldShowBanner: actionable,
    shouldShowList: actionable,
  });
}

export async function nativeForegroundPresentationBeforeDeadline(
  pending: Promise<ForegroundNotificationPresentation>,
  deadline: Promise<void>,
): Promise<NativeForegroundPresentation> {
  const presentation = await Promise.race([
    pending,
    deadline.then(() => 'quiet' as const),
  ]);
  return nativeForegroundPresentation(presentation);
}

export type NotificationDestination =
  | { kind: 'comments'; eventID: string; commentID?: string }
  | { kind: 'interaction-disabled'; eventID: string; interaction: 'comments' | 'reactions'; pathID: string }
  | { kind: 'invitation'; invitationID: string }
  | { kind: 'follow-request'; requestID: string }
  | { kind: 'profile'; username: string }
  | { kind: 'path'; pathID: string };

export function interactionDisabledDestinationFromAPI(value: unknown): NotificationDestination | null {
  if (!value || typeof value !== 'object') return null;
  const candidate = value as Record<string, unknown>;
  if (candidate.interactionDisabled !== 'comments' && candidate.interactionDisabled !== 'reactions') return null;
  if (typeof candidate.socialFeedEventId !== 'string' || !candidate.socialFeedEventId.trim() ||
      typeof candidate.pathId !== 'string' || !candidate.pathId.trim()) return null;
  return {
    eventID: candidate.socialFeedEventId,
    interaction: candidate.interactionDisabled,
    kind: 'interaction-disabled',
    pathID: candidate.pathId,
  };
}

type InstallationStorage = {
  read(key: string): Promise<string | null>;
  write(key: string, value: string): Promise<void>;
};

type PushRegistrationPorts = {
  installationID(): Promise<string>;
  permission(): Promise<PushPermission>;
  requestPermission(): Promise<PushPermission>;
  pushToken(): Promise<string>;
  register(registration: PushRegistrationWithToken): Promise<void>;
  deregister(registration: PushRegistration): Promise<void>;
};

type NotificationTapPorts = {
  resolve(notificationID: string): Promise<NotificationDestination | null>;
  markRead(notificationID: string): Promise<void>;
  navigate(destination: NotificationDestination): Promise<void> | void;
  unavailable(): void;
};

export function pushNotificationID(data: Readonly<Record<string, unknown>>): string | null {
  if (data.version !== 1 && data.version !== '1') return null;
  const notificationID = typeof data.notificationId === 'string'
    ? data.notificationId.trim()
    : '';
  return notificationID || null;
}

function requiredIdentifier(value: string, name: string): string {
  const identifier = value.trim();
  if (!identifier) throw new Error(`${name}_required`);
  return identifier;
}

export function parsePendingPushDeregistration(value: string | null): PendingPushDeregistration | null {
  if (!value) return null;
  try {
    const candidate: unknown = JSON.parse(value);
    if (!candidate || typeof candidate !== 'object') return null;
    const pending = candidate as Record<string, unknown>;
    if (
      typeof pending.accountID !== 'string'
      || typeof pending.installationID !== 'string'
      || typeof pending.credential !== 'string'
    ) return null;
    return {
      accountID: requiredIdentifier(pending.accountID, 'account_id'),
      installationID: requiredIdentifier(pending.installationID, 'installation_id'),
      credential: requiredIdentifier(pending.credential, 'credential'),
    };
  } catch {
    return null;
  }
}

export async function loadOrCreateInstallationID(
  storage: InstallationStorage,
  generate: () => string,
): Promise<string> {
  const stored = (await storage.read(pushInstallationStorageKey))?.trim();
  if (stored) return stored;
  const installationID = requiredIdentifier(generate(), 'installation_id');
  await storage.write(pushInstallationStorageKey, installationID);
  return installationID;
}

export function createPushRegistrationCoordinator(ports: PushRegistrationPorts) {
  let activeAccountID: string | null = null;

  async function deregister(accountID: string) {
    const installationID = await ports.installationID();
    await ports.deregister({
      accountID: requiredIdentifier(accountID, 'account_id'),
      installationID: requiredIdentifier(installationID, 'installation_id'),
    });
    if (activeAccountID === accountID) activeAccountID = null;
  }

  return {
    async synchronize(
      accountIDValue: string,
      options: { requestPermission?: boolean } = {},
    ): Promise<PushPermission> {
      const accountID = requiredIdentifier(accountIDValue, 'account_id');
      if (activeAccountID && activeAccountID !== accountID) {
        await deregister(activeAccountID);
      }

      let permission = await ports.permission();
      if (!permission.granted && permission.canAskAgain && options.requestPermission) {
        permission = await ports.requestPermission();
      }

      if (!permission.granted) {
        await deregister(accountID);
        return permission;
      }

      const installationID = requiredIdentifier(await ports.installationID(), 'installation_id');
      const pushToken = requiredIdentifier(await ports.pushToken(), 'push_token');
      await ports.register({ accountID, installationID, pushToken });
      activeAccountID = accountID;
      return permission;
    },

    async signOut(accountIDValue: string): Promise<void> {
      await deregister(requiredIdentifier(accountIDValue, 'account_id'));
    },
  };
}

export async function handleNotificationTap(
  data: Readonly<Record<string, unknown>>,
  ports: NotificationTapPorts,
): Promise<boolean> {
  const notificationID = pushNotificationID(data);
  if (!notificationID) return false;

  const destination = await ports.resolve(notificationID);
  if (!destination) {
    ports.unavailable();
    return true;
  }
  if (destination.kind !== 'interaction-disabled') {
    await ports.markRead(notificationID);
  }
  await ports.navigate(destination);
  return true;
}
