import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createPushRegistrationCoordinator,
  handleNotificationTap,
  interactionDisabledDestinationFromAPI,
  loadOrCreateInstallationID,
  nativeForegroundPresentation,
  nativeForegroundPresentationBeforeDeadline,
  parsePendingPushDeregistration,
  pushNotificationID,
  type PushPermission,
} from './push-notifications';

test('disabled interaction resolution keeps only privacy-safe event context', () => {
  assert.deepEqual(interactionDisabledDestinationFromAPI({
    actor: { displayName: 'actor-1' },
    interactionDisabled: 'reactions',
    pathId: 'path-1',
    reaction: 'heart',
    socialFeedEventId: 'event-1',
  }), {
    eventID: 'event-1',
    interaction: 'reactions',
    kind: 'interaction-disabled',
    pathID: 'path-1',
  });
  assert.equal(interactionDisabledDestinationFromAPI({
    interactionDisabled: 'comments',
    pathId: 'path-1',
    socialFeedEventId: '',
  }), null);
});

test('installation identity is stable once secure storage contains it', async () => {
  const values = new Map<string, string>();
  let generated = 0;
  const storage = {
    read: async (key: string) => values.get(key) ?? null,
    write: async (key: string, value: string) => { values.set(key, value); },
  };

  const first = await loadOrCreateInstallationID(storage, () => `installation-${++generated}`);
  const second = await loadOrCreateInstallationID(storage, () => `installation-${++generated}`);

  assert.equal(first, 'installation-1');
  assert.equal(second, first);
  assert.equal(generated, 1);
});

test('pending deregistration accepts only complete non-empty secure-store records', () => {
  assert.deepEqual(
    parsePendingPushDeregistration(JSON.stringify({
      accountID: 'account-1',
      credential: 'credential-1',
      installationID: 'installation-1',
    })),
    {
      accountID: 'account-1',
      credential: 'credential-1',
      installationID: 'installation-1',
    },
  );
  assert.equal(parsePendingPushDeregistration(null), null);
  assert.equal(parsePendingPushDeregistration('{'), null);
  assert.equal(parsePendingPushDeregistration(JSON.stringify({
    accountID: 'account-1',
    credential: '',
    installationID: 'installation-1',
  })), null);
});

test('registration requests and stores a token only after permission is granted', async () => {
  const events: string[] = [];
  let permission: PushPermission = { granted: false, canAskAgain: true };
  const coordinator = createPushRegistrationCoordinator({
    installationID: async () => 'installation-1',
    permission: async () => permission,
    requestPermission: async () => {
      events.push('permission:request');
      permission = { granted: true, canAskAgain: false };
      return permission;
    },
    pushToken: async () => {
      events.push('token:read');
      return 'push-token';
    },
    register: async (registration) => {
      events.push(`register:${registration.accountID}:${registration.installationID}:${registration.pushToken}`);
    },
    deregister: async (registration) => {
      events.push(`deregister:${registration.accountID}:${registration.installationID}`);
    },
  });

  assert.deepEqual(await coordinator.synchronize('account-1'), { granted: false, canAskAgain: true });
  assert.deepEqual(events, ['deregister:account-1:installation-1']);

  assert.deepEqual(await coordinator.synchronize('account-1', { requestPermission: true }), {
    granted: true,
    canAskAgain: false,
  });
  assert.deepEqual(events, [
    'deregister:account-1:installation-1',
    'permission:request',
    'token:read',
    'register:account-1:installation-1:push-token',
  ]);
});

test('registration deregisters on denial, sign-out, and account replacement', async () => {
  const events: string[] = [];
  let permission: PushPermission = { granted: true, canAskAgain: false };
  const coordinator = createPushRegistrationCoordinator({
    installationID: async () => 'installation-1',
    permission: async () => permission,
    requestPermission: async () => permission,
    pushToken: async () => 'push-token',
    register: async ({ accountID }) => { events.push(`register:${accountID}`); },
    deregister: async ({ accountID }) => { events.push(`deregister:${accountID}`); },
  });

  await coordinator.synchronize('account-1');
  await coordinator.synchronize('account-2');
  permission = { granted: false, canAskAgain: false };
  await coordinator.synchronize('account-2');
  await coordinator.signOut('account-2');

  assert.deepEqual(events, [
    'register:account-1',
    'deregister:account-1',
    'register:account-2',
    'deregister:account-2',
    'deregister:account-2',
  ]);
});

test('foreground presentation is intrusive only for unrelated actionable content', () => {
  assert.deepEqual(nativeForegroundPresentation('quiet'), {
    shouldPlaySound: false,
    shouldSetBadge: false,
    shouldShowBanner: false,
    shouldShowList: false,
  });
  assert.deepEqual(nativeForegroundPresentation('informational'), {
    shouldPlaySound: false,
    shouldSetBadge: false,
    shouldShowBanner: false,
    shouldShowList: false,
  });
  assert.deepEqual(nativeForegroundPresentation('actionable'), {
    shouldPlaySound: true,
    shouldSetBadge: false,
    shouldShowBanner: true,
    shouldShowList: true,
  });
});

test('foreground presentation returns quiet before Expo discards a slow notification', async () => {
  let release: ((presentation: 'actionable') => void) | undefined;
  const pending = new Promise<'actionable'>((resolve) => { release = resolve; });
  const deadline = Promise.resolve();

  assert.deepEqual(await nativeForegroundPresentationBeforeDeadline(pending, deadline), {
    shouldPlaySound: false,
    shouldSetBadge: false,
    shouldShowBanner: false,
    shouldShowList: false,
  });
  release?.('actionable');
});

test('push payload parsing accepts only a versioned opaque notification id', () => {
  assert.equal(pushNotificationID({ version: 1, notificationId: ' notification-1 ' }), 'notification-1');
  assert.equal(pushNotificationID({ version: '1', notificationId: 'notification-2', pathId: 'untrusted' }), 'notification-2');
  assert.equal(pushNotificationID({ version: 2, notificationId: 'notification-1' }), null);
  assert.equal(pushNotificationID({ version: 1, notificationId: '' }), null);
});

test('tap resolves only the opaque notification id before marking read and navigating', async () => {
  const events: string[] = [];
  const handled = await handleNotificationTap(
    { version: 1, notificationId: 'notification-1', pathId: 'untrusted-path' },
    {
      resolve: async (notificationID) => {
        events.push(`resolve:${notificationID}`);
        return { kind: 'path', pathID: 'authorized-path' };
      },
      markRead: async (notificationID) => { events.push(`read:${notificationID}`); },
      navigate: async (destination) => {
        if (destination.kind === 'path') events.push(`navigate:path:${destination.pathID}`);
        else if (destination.kind === 'invitation') events.push(`navigate:invitation:${destination.invitationID}`);
        else if (destination.kind === 'follow-request') events.push(`navigate:follow-request:${destination.requestID}`);
        else if (destination.kind === 'profile') events.push(`navigate:profile:${destination.username}`);
        else events.push(`navigate:comments:${destination.eventID}`);
      },
      unavailable: () => { events.push('unavailable'); },
    },
  );

  assert.equal(handled, true);
  assert.deepEqual(events, [
    'resolve:notification-1',
    'read:notification-1',
    'navigate:path:authorized-path',
  ]);
});

test('tap opens a disabled interaction after its notification has been tombstoned', async () => {
  const events: string[] = [];
  const handled = await handleNotificationTap(
    { version: 1, notificationId: 'notification-1' },
    {
      resolve: async () => ({
        eventID: 'event-1',
        interaction: 'comments',
        kind: 'interaction-disabled',
        pathID: 'path-1',
      }),
      markRead: async () => { throw new Error('notification_tombstoned'); },
      navigate: async (destination) => { events.push(`navigate:${destination.kind}`); },
      unavailable: () => { events.push('unavailable'); },
    },
  );

  assert.equal(handled, true);
  assert.deepEqual(events, ['navigate:interaction-disabled']);
});

test('tap ignores malformed payloads but explains an authoritatively unavailable item without exposing payload context', async () => {
  const events: string[] = [];
  const ports = {
    resolve: async (notificationID: string) => {
      events.push(`resolve:${notificationID}`);
      return null;
    },
    markRead: async (notificationID: string) => { events.push(`read:${notificationID}`); },
    navigate: async () => { events.push('navigate'); },
    unavailable: () => { events.push('unavailable'); },
  };

  assert.equal(await handleNotificationTap({ version: 1, pathId: 'path-1' }, ports), false);
  assert.equal(await handleNotificationTap({ version: 2, notificationId: 'notification-1' }, ports), false);
  assert.equal(await handleNotificationTap({ notificationId: 'notification-1' }, ports), false);
  assert.deepEqual(events, []);

  assert.equal(await handleNotificationTap({
    version: '1',
    notificationId: 'notification-1',
    pathId: 'private-path-name-must-not-be-used',
    actor: { displayName: 'private-actor-name-must-not-be-used' },
  }, ports), true);
  assert.deepEqual(events, ['resolve:notification-1', 'unavailable']);
});
