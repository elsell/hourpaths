import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyNotificationMutation,
  mergeNotificationHistoryPage,
  notificationPresentationMessageKey,
  type PathInvitationNotification,
} from './index.js';

const received: PathInvitationNotification = {
  id: 'notification-received-1',
  type: 'path_invitation_received',
  presentation: 'actionable',
  read: false,
  createdAt: '2026-07-23T12:00:00Z',
  actor: {
    userId: 'creator-1',
    username: 'Book.Owner',
    displayName: 'Book Owner',
  },
  pathId: 'path-1',
  pathName: 'Morning Reading',
  invitationId: 'invitation-1',
  offeredRole: 'participant',
};

const accepted: PathInvitationNotification = {
  ...received,
  id: 'notification-accepted-1',
  type: 'path_invitation_accepted',
  presentation: 'informational',
  read: true,
  createdAt: '2026-07-23T12:01:00Z',
  actor: {
    userId: 'reader-1',
    username: 'Reader.One',
    displayName: 'Reader One',
  },
  offeredRole: 'supporter',
};

const transferReceived: PathInvitationNotification = {
  id: 'notification-transfer-received-1',
  type: 'path_ownership_transfer_received',
  presentation: 'actionable',
  read: false,
  createdAt: '2026-07-23T12:02:00Z',
  actor: {
    userId: 'creator-1',
    username: 'Book.Owner',
    displayName: 'Book Owner',
  },
  pathId: 'path-1',
  pathName: 'Morning Reading',
  ownershipTransferId: 'transfer-1',
};

const pathDeleted: PathInvitationNotification = {
  id: 'notification-path-deleted-1',
  type: 'path_deleted',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-23T12:03:00Z',
  actor: received.actor,
  pathName: 'Morning Reading',
};

const roleChanged: PathInvitationNotification = {
  id: 'notification-role-changed-1',
  type: 'path_member_role_changed',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-23T12:03:30Z',
  actor: received.actor,
  pathId: 'path-1',
  pathName: 'Morning Reading',
  offeredRole: 'supporter',
};

const memberRemoved: PathInvitationNotification = {
  ...roleChanged,
  id: 'notification-member-removed-1',
  type: 'path_member_removed',
  offeredRole: 'participant',
};

const visibilityChanged: PathInvitationNotification = {
  id: 'notification-visibility-changed-1',
  type: 'path_visibility_changed',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-23T12:03:45Z',
  actor: received.actor,
  pathId: 'path-1',
  pathName: 'Morning Reading',
  pathVisibility: 'followers',
};

const followRequestReceived: PathInvitationNotification = {
  id: 'notification-follow-request-1',
  type: 'follow_request_received',
  presentation: 'actionable',
  read: false,
  createdAt: '2026-07-23T12:04:00Z',
  actor: received.actor,
  followRequestId: 'follow-request-1',
};

const practiceReaction: PathInvitationNotification = {
  id: 'notification-practice-reaction-1',
  type: 'practice_reaction',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-23T12:05:00Z',
  actor: received.actor,
  pathId: 'path-1',
  pathName: 'Morning Reading',
  socialFeedEventId: 'feed-event-1',
  reaction: 'heart',
};

const practiceComment: PathInvitationNotification = {
  id: 'notification-practice-comment-1',
  type: 'practice_comment',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-23T12:06:00Z',
  actor: received.actor,
  pathId: 'path-1',
  pathName: 'Morning Reading',
  socialFeedEventId: 'feed-event-1',
  commentId: 'comment-1',
};

const commentHeart: PathInvitationNotification = {
  ...practiceComment,
  id: 'notification-comment-heart-1',
  type: 'comment_heart',
};

const nudgeReceived: PathInvitationNotification = {
  id: 'notification-nudge-1',
  type: 'nudge_received',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-23T12:07:00Z',
  actor: received.actor,
  pathId: 'path-1',
  pathName: 'Morning Reading',
  content: { kind: 'preset', preset: 'you_have_got_this' },
};

test('received nudges retain only resolved actor, Path, and preset content', () => {
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [nudgeReceived], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(page.items, [nudgeReceived]);
  assert.equal(
    notificationPresentationMessageKey(nudgeReceived),
    'notification.nudge.you_have_got_this',
  );
  for (const preset of [
    'you_have_got_this',
    'lets_go',
    'little_progress_counts',
    'keep_it_going',
    'time_to_work',
  ] as const) {
    assert.equal(
      notificationPresentationMessageKey({ ...nudgeReceived, content: { kind: 'preset', preset } }),
      `notification.nudge.${preset}`,
    );
  }
});

test('received nudges reject custom text, actionability, identifiers, and history metadata', () => {
  for (const malformed of [
    { ...nudgeReceived, presentation: 'actionable' },
    { ...nudgeReceived, content: { kind: 'custom', text: 'Keep going' } },
    { ...nudgeReceived, content: { kind: 'preset', preset: 'unknown' } },
    { ...nudgeReceived, content: { kind: 'preset', preset: 'lets_go', text: 'custom' } },
    { ...nudgeReceived, content: undefined },
    { ...nudgeReceived, nudgeId: 'nudge-1' },
    { ...nudgeReceived, sentHistory: ['nudge-1'] },
    { ...nudgeReceived, message: 'custom' },
  ]) assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [malformed], nextCursor: '', unreadCount: 0 },
    '',
  ), /invalid notification/);
});

test('practice-comment notifications retain only server-resolved comment navigation context', () => {
  const state = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [practiceComment], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(state.items, [practiceComment]);
  assert.equal(notificationPresentationMessageKey(state.items[0]!), 'notification.practiceComment');
  assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [{ ...practiceComment, commentId: undefined }], nextCursor: '', unreadCount: 1 },
    '',
  ), /invalid notification/);
});

test('comment-heart notifications retain only public actor and focused comment context', () => {
  const state = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [commentHeart], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(state.items, [commentHeart]);
  assert.equal(notificationPresentationMessageKey(state.items[0]!), 'notification.commentHeart');
  for (const malformed of [
    { ...commentHeart, presentation: 'actionable' },
    { ...commentHeart, commentId: '' },
    { ...commentHeart, socialFeedEventId: '' },
    { ...commentHeart, actor: { ...commentHeart.actor, email: 'private@example.test' } },
    { ...commentHeart, heartCount: 1 },
  ]) assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [malformed], nextCursor: '', unreadCount: 0 },
    '',
  ), /invalid notification/);
});

test('notification pages retain signed cursors and merge each notification once', () => {
  const signed = 'eyJ1c2VyIjoicmVhZGVyLTEifQ.signature+/=';
  const first = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [accepted, received], nextCursor: signed, unreadCount: 17 },
    '',
  );
  assert.deepEqual(first, { items: [accepted, received], nextCursor: signed, unreadCount: 17 });

  const readReceived = { ...received, read: true };
  const second = mergeNotificationHistoryPage(
    first,
    { items: [readReceived], nextCursor: '', unreadCount: 0 },
    signed,
  );
  assert.deepEqual(second, { items: [accepted, readReceived], nextCursor: '', unreadCount: 17 });
  assert.throws(
    () => mergeNotificationHistoryPage(first, { items: [], nextCursor: '', unreadCount: 17 }, 'wrong.cursor'),
    /stale notification cursor/,
  );
});

test('notification page replacement converges read, deletion, cursor, and authoritative count', () => {
  const stale = {
    items: [accepted, received],
    nextCursor: 'stale-signed-cursor',
    unreadCount: 11,
  };
  const replacement = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: stale.unreadCount },
    {
      items: [{ ...received, read: true }],
      nextCursor: 'fresh-signed-cursor',
      unreadCount: 3,
    },
    '',
  );
  assert.deepEqual(replacement, {
    items: [{ ...received, read: true }],
    nextCursor: 'fresh-signed-cursor',
    unreadCount: 3,
  });
});

test('notification history rejects malformed outer data and cursors', () => {
  for (const page of [
    null,
    {},
    { items: null, nextCursor: '' },
    { items: [], nextCursor: null },
    { items: [], nextCursor: ' signed' },
    { items: [], nextCursor: '', unreadCount: -1 },
    { items: [], nextCursor: '', unreadCount: 1.5 },
    { items: [], nextCursor: '', unreadCount: 0, extra: true },
  ]) {
    assert.throws(
      () => mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        page as never,
        '',
      ),
      /invalid notification page/,
    );
  }
  assert.throws(
    () => mergeNotificationHistoryPage(
      { items: [], nextCursor: '', unreadCount: 0 },
      { items: [], nextCursor: '', unreadCount: 0 },
      null as never,
    ),
    /invalid notification cursor/,
  );
});

test('notification history rejects malformed type and presentation semantics', () => {
  for (const malformed of [
    { ...received, type: 'unknown' },
    { ...received, presentation: 'informational' },
    { ...accepted, presentation: 'actionable' },
    { ...accepted, type: 'path_invitation_received' },
    { ...transferReceived, presentation: 'informational' },
    { ...transferReceived, type: 'path_ownership_transfer_accepted', presentation: 'actionable' },
    { ...received, read: 'false' },
  ]) {
    assert.throws(
      () => mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        { items: [malformed], nextCursor: '', unreadCount: 0 },
        '',
      ),
      /invalid notification/,
    );
  }
});

test('social notifications retain only actor and optional follow-request context', () => {
  const newFollower: PathInvitationNotification = {
    id: 'notification-follower-1',
    type: 'new_follower',
    presentation: 'informational',
    read: false,
    createdAt: '2026-07-23T12:05:00Z',
    actor: received.actor,
  };
  const acceptedRequest: PathInvitationNotification = {
    ...followRequestReceived,
    id: 'notification-follow-accepted-1',
    type: 'follow_request_accepted',
    presentation: 'informational',
    createdAt: '2026-07-23T12:04:30Z',
  };
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [newFollower, acceptedRequest, followRequestReceived], nextCursor: '', unreadCount: 2 },
    '',
  );
  assert.equal(page.items.length, 3);
  assert.equal(notificationPresentationMessageKey(newFollower), 'notification.newFollower');
  assert.equal(notificationPresentationMessageKey(followRequestReceived), 'notification.followRequestReceived');
  assert.equal(notificationPresentationMessageKey(acceptedRequest), 'notification.followRequestAccepted');
  for (const malformed of [
    { ...followRequestReceived, followRequestId: undefined },
    { ...followRequestReceived, pathId: 'invented' },
    { ...newFollower, followRequestId: 'invented' },
  ]) assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [malformed], nextCursor: '', unreadCount: 0 },
    '',
  ), /invalid notification/);
});

test('practice-reaction notifications retain only curated informational Path context', () => {
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [practiceReaction], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(page.items, [practiceReaction]);
  assert.equal(notificationPresentationMessageKey(practiceReaction), 'notification.practiceReaction.heart');

  for (const reaction of ['heart', 'applause', 'fire', 'strong', 'celebrate'] as const) {
    assert.equal(
      notificationPresentationMessageKey({ ...practiceReaction, reaction }),
      `notification.practiceReaction.${reaction}`,
    );
  }

  for (const malformed of [
    { ...practiceReaction, presentation: 'actionable' },
    { ...practiceReaction, pathId: '' },
    { ...practiceReaction, pathName: '' },
    { ...practiceReaction, socialFeedEventId: '' },
    { ...practiceReaction, reaction: 'thumbs_down' },
    { ...practiceReaction, privateRoster: ['user-1'] },
  ]) assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [malformed], nextCursor: '', unreadCount: 0 },
    '',
  ), /invalid notification/);
});

test('ownership-transfer notifications require only their transfer subject', () => {
  const acceptedTransfer = {
    ...transferReceived,
    id: 'notification-transfer-accepted-1',
    type: 'path_ownership_transfer_accepted' as const,
    presentation: 'informational' as const,
  };
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [acceptedTransfer, transferReceived], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(page.items, [acceptedTransfer, transferReceived]);

  for (const malformed of [
    { ...transferReceived, ownershipTransferId: '' },
    { ...transferReceived, invitationId: 'invitation-1' },
    { ...transferReceived, offeredRole: 'participant' },
    { ...received, ownershipTransferId: 'transfer-1' },
  ]) {
    assert.throws(
      () => mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        { items: [malformed], nextCursor: '', unreadCount: 0 },
        '',
      ),
      /invalid notification/,
    );
  }
});

test('standalone Path-deletion notices retain explanation snapshots without a Path target', () => {
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [pathDeleted], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(page.items, [pathDeleted]);
  assert.equal(notificationPresentationMessageKey(pathDeleted), 'notification.pathDeleted');
  for (const malformed of [
    { ...pathDeleted, presentation: 'actionable' },
    { ...pathDeleted, pathId: 'deleted-path' },
    { ...pathDeleted, invitationId: 'deleted-invitation' },
    { ...pathDeleted, ownershipTransferId: 'deleted-transfer' },
  ]) {
    assert.throws(
      () => mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        { items: [malformed], nextCursor: '', unreadCount: 0 },
        '',
      ),
      /invalid notification/,
    );
  }
});

test('Path access changes are quiet role-specific informational notices', () => {
  const administratorChanged = { ...roleChanged, id: 'notification-admin', offeredRole: 'administrator' as const };
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [memberRemoved, roleChanged, administratorChanged], nextCursor: '', unreadCount: 3 },
    '',
  );
  assert.deepEqual(page.items, [memberRemoved, roleChanged, administratorChanged]);
  assert.equal(notificationPresentationMessageKey(roleChanged), 'notification.pathMemberRoleChanged.supporter');
  assert.equal(notificationPresentationMessageKey(administratorChanged), 'notification.pathMemberRoleChanged.administrator');
  assert.equal(notificationPresentationMessageKey(memberRemoved), 'notification.pathMemberRemoved.participant');
  for (const malformed of [
    { ...roleChanged, presentation: 'actionable' },
    { ...memberRemoved, offeredRole: 'administrator' },
    { ...roleChanged, invitationId: 'invitation-1' },
    { ...memberRemoved, pathId: undefined },
  ]) assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [malformed], nextCursor: '', unreadCount: 0 },
    '',
  ), /invalid notification/);
});

test('Path visibility changes retain the exact audience needed for quiet Path-scoped explanation copy', () => {
  const page = mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [visibilityChanged], nextCursor: '', unreadCount: 1 },
    '',
  );
  assert.deepEqual(page.items, [visibilityChanged]);
  assert.equal(page.items[0]?.pathVisibility, 'followers');
  assert.equal(notificationPresentationMessageKey(visibilityChanged), 'notification.pathVisibilityChanged');
  for (const malformed of [
    { ...visibilityChanged, presentation: 'actionable' },
    { ...visibilityChanged, pathVisibility: 'members' },
    { ...visibilityChanged, profileVisibility: 'public' },
    { ...visibilityChanged, recipient: received.actor },
  ]) assert.throws(() => mergeNotificationHistoryPage(
    { items: [], nextCursor: '', unreadCount: 0 },
    { items: [malformed], nextCursor: '', unreadCount: 0 },
    '',
  ), /invalid notification/);
});

test('notification history rejects malformed or over-broad public identity', () => {
  for (const actor of [
    { ...received.actor, userId: '' },
    { ...received.actor, username: ' Book.Owner' },
    { ...received.actor, displayName: '' },
    { ...received.actor, email: 'owner@example.test' },
    { ...received.actor, profileVisibility: 'private' },
  ]) {
    assert.throws(
      () => mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        { items: [{ ...received, actor }], nextCursor: '', unreadCount: 0 },
        '',
      ),
      /invalid notification/,
    );
  }
});

test('notification history rejects malformed Path, invitation, role, timestamp, and extra data', () => {
  for (const malformed of [
    { ...received, id: '' },
    { ...received, pathId: '' },
    { ...received, pathName: ' Morning Reading' },
    { ...received, invitationId: '' },
    { ...received, offeredRole: 'administrator' },
    { ...received, createdAt: 'not-an-instant' },
    { ...received, createdAt: ' 2026-07-23T12:00:00Z' },
    { ...received, providerEmail: 'owner@example.test' },
  ]) {
    assert.throws(
      () => mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        { items: [malformed], nextCursor: '', unreadCount: 0 },
        '',
      ),
      /invalid notification/,
    );
  }
});

test('notification presentation selects a localized type-and-role message key', () => {
  assert.equal(
    notificationPresentationMessageKey(received),
    'notification.pathInvitationReceived.participant',
  );
  assert.equal(
    notificationPresentationMessageKey(accepted),
    'notification.pathInvitationAccepted.supporter',
  );
  assert.equal(
    notificationPresentationMessageKey(transferReceived),
    'notification.pathOwnershipTransferReceived',
  );
  for (const type of ['accepted', 'declined', 'canceled'] as const) {
    assert.equal(
      notificationPresentationMessageKey({
        ...transferReceived,
        type: `path_ownership_transfer_${type}`,
        presentation: 'informational',
      }),
      `notification.pathOwnershipTransfer${type[0]!.toUpperCase()}${type.slice(1)}`,
    );
  }
});

test('notification mutations apply only the acknowledged local state and authoritative unread count', () => {
  const history = { items: [accepted, received], nextCursor: 'signed-next', unreadCount: 1 };
  assert.deepEqual(
    applyNotificationMutation(history, { kind: 'read', notificationId: received.id }, { unreadCount: 1 }),
    {
      history: { items: [accepted, { ...received, read: true }], nextCursor: 'signed-next', unreadCount: 1 },
      unreadCount: 1,
    },
  );
  assert.deepEqual(
    applyNotificationMutation(history, { kind: 'delete', notificationId: accepted.id }, { unreadCount: 1 }),
    {
      history: { items: [received], nextCursor: 'signed-next', unreadCount: 1 },
      unreadCount: 1,
    },
  );
  assert.deepEqual(
    applyNotificationMutation(history, { kind: 'read-all' }, { unreadCount: 0 }),
    {
      history: {
        items: [{ ...accepted, read: true }, { ...received, read: true }],
        nextCursor: 'signed-next',
        unreadCount: 0,
      },
      unreadCount: 0,
    },
  );
});

test('notification mutations fail closed on malformed results and unknown local targets', () => {
  const history = { items: [received], nextCursor: '', unreadCount: 1 };
  for (const result of [
    null,
    {},
    { unreadCount: -1 },
    { unreadCount: 1.5 },
    { unreadCount: 1, extra: true },
  ]) {
    assert.throws(
      () => applyNotificationMutation(history, { kind: 'read-all' }, result as never),
      /invalid notification mutation result/,
    );
  }
  assert.throws(
    () => applyNotificationMutation(
      history,
      { kind: 'read', notificationId: 'missing' },
      { unreadCount: 0 },
    ),
    /unknown notification/,
  );
});
