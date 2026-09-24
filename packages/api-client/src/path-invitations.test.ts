import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient, generatedResponse } from './index.js';
import type {
  PathCapabilities,
  PathInvitation,
  PathInvitationAccept,
  PathInvitationCreate,
  PathInvitationNotification,
  ManagedPathInvitation,
  PathInvitationRecipient,
  PathInvitationRejection,
  PendingPathInvitation,
  SessionPath,
} from './index.js';

const capabilities: PathCapabilities = {
  trackTime: false,
  renamePath: false,
  inviteMembers: false,
  manageMembers: false,
  manageGoals: false,
	manageLifecycle: false,
	manageVisibility: false,
  transferOwnership: false,
  leavePath: false,
};

const projectedPath: SessionPath = {
  id: 'path-1',
  name: 'Piano',
  visibility: 'followers',
  capabilities,
};

const recipient: PathInvitationRecipient = {
  userId: 'recipient',
  username: 'reader',
  displayName: 'Reader',
};

const invitation: PathInvitation = {
  id: 'invitation-1',
  pathId: 'path-1',
  inviterUserId: 'creator',
  recipientUserId: 'recipient',
  offeredRole: 'supporter',
  createdAt: '2026-07-23T12:00:00Z',
};

const pendingInvitation: PendingPathInvitation = {
  invitation,
  pathName: 'Morning Reading',
  inviter: {
    userId: 'creator',
    username: 'Book.Owner',
    displayName: 'Book Owner',
  },
  warning: {
    pathVisibility: 'followers',
    hasRetainedActivity: true,
  },
};

const notification: PathInvitationNotification = {
  id: 'notification-1',
  type: 'path_invitation_received',
  presentation: 'actionable',
  read: false,
  createdAt: '2026-07-23T12:00:00Z',
  actor: {
    userId: 'creator',
    username: 'Book.Owner',
    displayName: 'Book Owner',
  },
  pathId: 'path-1',
  pathName: 'Morning Reading',
  invitationId: 'invitation-1',
  offeredRole: 'supporter',
};

const reactionNotification: PathInvitationNotification = {
  id: 'reaction-notification-1',
  type: 'practice_reaction',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-28T12:00:00Z',
  actor: {
    userId: 'reactor',
    username: 'reader',
    displayName: 'Reader',
  },
  pathId: 'path-1',
  pathName: 'Piano',
  socialFeedEventId: 'practice:activity-1',
  reaction: 'fire',
};

test('practice reaction notifications expose only path-addressable curated context', () => {
  assert.deepEqual(
    {
      pathId: reactionNotification.pathId,
      pathName: reactionNotification.pathName,
      reaction: reactionNotification.reaction,
      socialFeedEventId: reactionNotification.socialFeedEventId,
      type: reactionNotification.type,
    },
    {
      pathId: 'path-1',
      pathName: 'Piano',
      reaction: 'fire',
      socialFeedEventId: 'practice:activity-1',
      type: 'practice_reaction',
    },
  );
});

const rejection: PathInvitationRejection = {
  invitationId: 'invitation-1',
  rejectedAt: '2026-07-23T12:01:00Z',
  unreadCount: 0,
};

test('path invitation operations use generated routes, current credentials, and caller-owned replay keys', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  const tokens = [
    'review-token', 'send-token', 'retry-token', 'list-token', 'notification-token',
    'read-token', 'delete-token', 'read-all-token', 'accept-token',
    'accept-confirm-token', 'accept-confirm-retry-token', 'reject-token',
  ] as const;
  let tokenIndex = 0;
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    const path = new URL(request.url).pathname;
    if (path.endsWith('/invitation-recipient')) {
      return Response.json({ data: recipient });
    }
    if (path === '/v1/path-invitations') {
      return Response.json({ data: [pendingInvitation], meta: { nextCursor: 'signed-next-page' } });
    }
    if (path === '/v1/notifications') {
      return Response.json({ data: [notification], meta: { nextCursor: 'signed-notification-page', unreadCount: 42 } });
    }
    if (path.startsWith('/v1/notifications/')) {
      return Response.json({ data: { unreadCount: 0 } });
    }
    if (path.endsWith('/reject')) return Response.json({ data: rejection });
    return Response.json({ data: invitation }, { status: path.endsWith('/invitations') ? 201 : 200 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const api = createSessionApiClient('https://api.example.test', () => tokens[tokenIndex++] ?? null);
  const body: PathInvitationCreate = {
    username: 'reader',
    expectedRecipientUserId: 'recipient',
    offeredRole: 'supporter',
  };
  const reviewed = await api.reviewPathInvitationRecipient('path-1', 'reader');
  const sent = await api.sendPathInvitation('path-1', body, 'send-invite-key-01');
  await api.sendPathInvitation('path-1', body, 'send-invite-key-01');
  const pending = await api.pendingPathInvitations('signed-current-page');
  const notifications = await api.notifications('signed-notification-current');
  const read = await api.markNotificationRead('notification-1');
  const deleted = await api.deleteNotification('notification-1');
  const allRead = await api.markAllNotificationsRead();
  const accepted = await api.acceptPathInvitation('invitation-1', 'accept-invite-key1');
  const acknowledgement: PathInvitationAccept = {
    visibilityWarningAcknowledgement: { pathVisibility: 'followers' },
  };
  await api.acceptPathInvitation('invitation-1', 'accept-confirm-key', acknowledgement);
  await api.acceptPathInvitation('invitation-1', 'accept-confirm-key', acknowledgement);
  const rejected = await api.rejectPathInvitation('invitation-1', 'reject-invite-key1');

  assert.deepEqual(requests.map((request) => [
    request.method,
    new URL(request.url).pathname,
    request.headers.get('authorization'),
  ]), [
    ['GET', '/v1/paths/path-1/invitation-recipient', 'Bearer review-token'],
    ['POST', '/v1/paths/path-1/invitations', 'Bearer send-token'],
    ['POST', '/v1/paths/path-1/invitations', 'Bearer retry-token'],
    ['GET', '/v1/path-invitations', 'Bearer list-token'],
    ['GET', '/v1/notifications', 'Bearer notification-token'],
    ['PATCH', '/v1/notifications/notification-1/read', 'Bearer read-token'],
    ['DELETE', '/v1/notifications/notification-1', 'Bearer delete-token'],
    ['POST', '/v1/notifications/read-all', 'Bearer read-all-token'],
    ['POST', '/v1/path-invitations/invitation-1/accept', 'Bearer accept-token'],
    ['POST', '/v1/path-invitations/invitation-1/accept', 'Bearer accept-confirm-token'],
    ['POST', '/v1/path-invitations/invitation-1/accept', 'Bearer accept-confirm-retry-token'],
    ['POST', '/v1/path-invitations/invitation-1/reject', 'Bearer reject-token'],
  ]);
  assert.equal(new URL(requests[0]!.url).searchParams.get('username'), 'reader');
  assert.deepEqual(await requests[1]!.json(), body);
  assert.deepEqual(await requests[2]!.json(), body);
  assert.equal(requests[1]!.headers.get('idempotency-key'), 'send-invite-key-01');
  assert.equal(requests[2]!.headers.get('idempotency-key'), 'send-invite-key-01');
  assert.equal(new URL(requests[3]!.url).search, '?cursor=signed-current-page&limit=25');
  assert.equal(new URL(requests[4]!.url).search, '?cursor=signed-notification-current&limit=25');
  assert.equal(requests[8]!.headers.get('idempotency-key'), 'accept-invite-key1');
  assert.equal(await requests[8]!.text(), '');
  assert.equal(requests[9]!.headers.get('idempotency-key'), 'accept-confirm-key');
  assert.equal(requests[10]!.headers.get('idempotency-key'), 'accept-confirm-key');
  assert.deepEqual(await requests[9]!.json(), acknowledgement);
  assert.deepEqual(await requests[10]!.json(), acknowledgement);
  assert.equal(requests[11]!.headers.get('idempotency-key'), 'reject-invite-key1');
  assert.equal(await requests[11]!.text(), '');
  assert.deepEqual(await generatedResponse(reviewed).json(), { data: recipient });
  assert.deepEqual(await generatedResponse(sent).json(), { data: invitation });
  assert.deepEqual(await generatedResponse(pending).json(), {
    data: [pendingInvitation],
    meta: { nextCursor: 'signed-next-page' },
  });
  assert.deepEqual(await generatedResponse(notifications).json(), {
    data: [notification],
    meta: { nextCursor: 'signed-notification-page', unreadCount: 42 },
  });
  assert.deepEqual(await generatedResponse(read).json(), { data: { unreadCount: 0 } });
  assert.deepEqual(await generatedResponse(deleted).json(), { data: { unreadCount: 0 } });
  assert.deepEqual(await generatedResponse(allRead).json(), { data: { unreadCount: 0 } });
  assert.deepEqual(await generatedResponse(accepted).json(), { data: invitation });
  assert.deepEqual(await generatedResponse(rejected).json(), { data: rejection });
});

test('projected paths expose server-authoritative typed capabilities', () => {
  assert.equal(projectedPath.capabilities.trackTime, false);
  assert.equal(projectedPath.capabilities.inviteMembers, false);
});

test('manager invitation list and cancellation use generated Path-bound contracts', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  const managed: ManagedPathInvitation = {
    invitation,
    inviter: { userId: 'creator', username: 'Book.Owner', displayName: 'Book Owner' },
    recipient: { userId: 'recipient', username: 'reader', displayName: 'Reader' },
  };
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    if (request.method === 'DELETE') return Response.json({ data: { invitationId: 'invitation-1', canceledAt: '2026-08-08T12:00:00Z' } });
    return Response.json({ data: [managed], meta: { nextCursor: 'managed-next' } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });
  const tokens = ['managed-list-token', 'managed-cancel-token']; let index = 0;
  const api = createSessionApiClient('https://api.example.test', () => tokens[index++] ?? null);
  const listed = await api.managedPathInvitations('path-1', 'managed-current', 10);
  const canceled = await api.cancelPathInvitation('path-1', 'invitation-1', 'cancel-invite-key1');
  assert.deepEqual(requests.map((request) => [request.method, new URL(request.url).pathname, request.headers.get('authorization')]), [
    ['GET', '/v1/paths/path-1/invitations', 'Bearer managed-list-token'],
    ['DELETE', '/v1/paths/path-1/invitations/invitation-1', 'Bearer managed-cancel-token'],
  ]);
  assert.equal(new URL(requests[0]!.url).search, '?cursor=managed-current&limit=10');
  assert.equal(requests[1]!.headers.get('idempotency-key'), 'cancel-invite-key1');
  assert.deepEqual(await generatedResponse(listed).json(), { data: [managed], meta: { nextCursor: 'managed-next' } });
  assert.deepEqual(await generatedResponse(canceled).json(), { data: { invitationId: 'invitation-1', canceledAt: '2026-08-08T12:00:00Z' } });
});
