import assert from 'node:assert/strict';
import test from 'node:test';
import type { ActivityDeletionMutationResult, NotificationHistoryState, PracticeCommentPage } from '@hourpaths/client-core';
import { applyLoadedActivityDeletionResult } from './activity-deletion-projection';
import type { ActivityDetail } from './activity-history';
import type { PathMemberSummary } from './ui/path-member-management-view';
import type { SocialFeedPage } from './ui/social-feed-presentation';

const activity = (id: string, participantId = 'owner-1'): ActivityDetail => ({
  activity: {
    createdAt: '2026-08-03T12:30:00Z', durationSeconds: 600, endedAt: '2026-08-03T12:30:00Z',
    id, occurrenceTimeZone: 'America/New_York', participantId, pathId: 'path-1',
    startedAt: '2026-08-03T12:20:00Z', updatedAt: '2026-08-03T12:30:00Z',
  },
  version: 1,
});

const member = (userId: string): PathMemberSummary => ({
  blockedByViewer: false, canChangeRole: false, canGrantAdministrator: false, canLeave: false,
  canRemove: false, canRevokeAdministrator: false, canStepDownAdministrator: false,
  displayName: userId, intervalProgress: { accumulatedSeconds: 900, targetSeconds: 1_800 },
  isViewer: userId === 'owner-1', overallProgress: { accumulatedSeconds: 4_200, targetSeconds: 7_200 },
  pathId: 'path-1', role: 'participant', sessionCount: userId === 'owner-1' ? 99 : 4, totalTrackedSeconds: 4_200,
  userId, username: userId,
});

const state = () => ({
  activities: [activity('activity-1'), activity('activity-2')],
  comments: {
    items: [
      { author: { displayName: 'actor-a', userId: 'a', username: 'a' }, authorUserId: 'a', createdAt: '2026-08-03T12:31:00Z', edited: false, eventId: 'practice:activity-1', heartCount: 1, heartedByViewer: false, heartPending: false, id: 'comment-1', pending: false, text: 'gone', updatedAt: '2026-08-03T12:31:00Z', version: 1 },
      { author: { displayName: 'actor-a', userId: 'a', username: 'a' }, authorUserId: 'a', createdAt: '2026-08-03T12:31:01Z', edited: false, eventId: 'achievement:interval-1', heartCount: 0, heartedByViewer: false, heartPending: false, id: 'comment-achievement', pending: false, text: 'also gone', updatedAt: '2026-08-03T12:31:01Z', version: 1 },
      { author: { displayName: 'actor-b', userId: 'b', username: 'b' }, authorUserId: 'b', createdAt: '2026-08-03T12:31:00Z', edited: false, eventId: 'practice:activity-2', heartCount: 0, heartedByViewer: false, heartPending: false, id: 'comment-2', pending: false, text: 'kept', updatedAt: '2026-08-03T12:31:00Z', version: 1 },
    ],
    nextCursor: 'comments-next',
  } satisfies PracticeCommentPage,
  feed: {
    items: [
      { activity: { durationSeconds: 600, edited: false, id: 'activity-1' }, commentsEnabled: true, id: 'practice:activity-1', participant: { displayName: 'owner-1', userId: 'owner-1', username: 'owner' }, path: { id: 'path-1', name: 'path-1' }, publishedAt: '2026-08-03T12:30:00Z', reactions: { applause: 0, celebrate: 0, fire: 0, heart: 1, strong: 0 }, reactionsEnabled: true, type: 'practice_session' as const, viewerReaction: null },
      { achievement: { kind: 'interval' as const, targetSeconds: 1_800, intervalStartedAt: '2026-08-03T12:00:00Z', intervalEndedAt: '2026-08-04T12:00:00Z' }, commentsEnabled: true, id: 'achievement:interval-1', participant: { displayName: 'owner-1', userId: 'owner-1', username: 'owner' }, path: { id: 'path-1', name: 'path-1' }, publishedAt: '2026-08-03T12:30:01Z', reactions: { applause: 0, celebrate: 1, fire: 0, heart: 0, strong: 0 }, reactionsEnabled: true, type: 'goal_achievement' as const, viewerReaction: null },
      { activity: { durationSeconds: 300, edited: false, id: 'activity-2' }, commentsEnabled: true, id: 'practice:activity-2', participant: { displayName: 'owner-1', userId: 'owner-1', username: 'owner' }, path: { id: 'path-1', name: 'path-1' }, publishedAt: '2026-08-03T11:30:00Z', reactions: { applause: 0, celebrate: 0, fire: 0, heart: 0, strong: 0 }, reactionsEnabled: true, type: 'practice_session' as const, viewerReaction: null },
    ],
    nextCursor: 'feed-next',
  } satisfies SocialFeedPage,
  members: [member('owner-1'), member('other-1')],
  notifications: {
    items: [
      { actor: { displayName: 'actor-a', userId: 'a', username: 'a' }, createdAt: '2026-08-03T12:32:00Z', id: 'notification-1', pathId: 'path-1', pathName: 'path-1', presentation: 'informational' as const, reaction: 'heart' as const, read: false, socialFeedEventId: 'practice:activity-1', type: 'practice_reaction' as const },
      { actor: { displayName: 'actor-a', userId: 'a', username: 'a' }, commentId: 'comment-achievement', createdAt: '2026-08-03T12:33:00Z', id: 'notification-achievement', pathId: 'path-1', pathName: 'path-1', presentation: 'informational' as const, read: false, socialFeedEventId: 'achievement:interval-1', type: 'practice_comment' as const },
      { actor: { displayName: 'actor-b', userId: 'b', username: 'b' }, createdAt: '2026-08-03T11:32:00Z', id: 'notification-2', pathId: 'path-1', pathName: 'path-1', presentation: 'informational' as const, reaction: 'heart' as const, read: false, socialFeedEventId: 'practice:activity-2', type: 'practice_reaction' as const },
    ],
    nextCursor: 'notifications-next', unreadCount: 11,
  } satisfies NotificationHistoryState,
  pathMemberActivities: [activity('activity-1'), activity('activity-2')],
  selectedMember: member('owner-1'),
  timer: { accumulatedSeconds: 4_200, intervalProgress: { accumulatedSeconds: 900, targetSeconds: 1_800 }, running: false },
});

test('authoritative deletion purges exact loaded derivatives and reconciles owner progress', () => {
  const current = state();
  const mutation: ActivityDeletionMutationResult = {
    kind: 'applied', result: {
      accumulatedSeconds: 3_600,
      intervalProgress: { accumulatedSeconds: 300, targetSeconds: 1_800 },
      removedFeedEventIds: ['practice:activity-1', 'achievement:interval-1'],
      sessionCount: 3,
      unreadNotificationCount: 7,
    },
  };
  const next = applyLoadedActivityDeletionResult(current, mutation, {
    activityId: 'activity-1', ownerId: 'owner-1', pathId: 'path-1',
  });

  assert.deepEqual(next.activities.map(({ activity }) => activity.id), ['activity-2']);
  assert.deepEqual(next.pathMemberActivities.map(({ activity }) => activity.id), ['activity-2']);
  assert.deepEqual(next.feed.items.map(({ id }) => id), ['practice:activity-2']);
  assert.deepEqual(next.comments.items.map(({ id }) => id), ['comment-2']);
  assert.deepEqual(next.notifications.items.map(({ id }) => id), ['notification-2']);
  assert.equal(next.notifications.unreadCount, 7);
  assert.equal(next.removedUnreadCount, 2);
  assert.deepEqual(next.removedFeedEventIds, ['practice:activity-1', 'achievement:interval-1']);
  assert.strictEqual(next.feed.items[0], current.feed.items[2]);
  assert.strictEqual(next.comments.items[0], current.comments.items[2]);
  assert.strictEqual(next.notifications.items[0], current.notifications.items[2]);
  assert.strictEqual(next.pathMemberActivities[0], current.pathMemberActivities[1]);
  assert.deepEqual(next.timer, { accumulatedSeconds: 3_600, intervalProgress: { accumulatedSeconds: 300, targetSeconds: 1_800 }, running: false });
  assert.deepEqual(next.members[0], {
    ...current.members[0], intervalProgress: { accumulatedSeconds: 300, targetSeconds: 1_800 },
    overallProgress: { accumulatedSeconds: 3_600, targetSeconds: 7_200 }, sessionCount: 3, totalTrackedSeconds: 3_600,
  });
  assert.deepEqual(next.selectedMember, next.members[0]);
  assert.strictEqual(next.members[1], current.members[1]);
  assert.deepEqual(current.activities.map(({ activity }) => activity.id), ['activity-1', 'activity-2']);
});

test('failed deletion preserves every loaded projection by identity', () => {
  const current = state();
  const next = applyLoadedActivityDeletionResult(current, { kind: 'failed', cause: new Error('offline') }, {
    activityId: 'activity-1', ownerId: 'owner-1', pathId: 'path-1',
  });
  assert.strictEqual(next, current);
});
