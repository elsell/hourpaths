import {
  applyActivityDeletionResult,
  type ActivityDeletionMutationResult,
  type NotificationHistoryState,
  type PracticeCommentPage,
} from '@hourpaths/client-core';
import type { ActivityDetail } from './activity-history';
import type { PathMemberSummary } from './ui/path-member-management-view';
import type { SocialFeedPage } from './ui/social-feed-presentation';

type TimerProjection = Readonly<{
  accumulatedSeconds: number;
  intervalProgress?: Readonly<{ accumulatedSeconds: number; targetSeconds: number }>;
  running: boolean;
}>;

export type LoadedActivityDeletionState = Readonly<{
  activities: readonly ActivityDetail[];
  comments: PracticeCommentPage;
  feed: SocialFeedPage;
  members: readonly PathMemberSummary[];
  notifications: NotificationHistoryState;
  pathMemberActivities: readonly ActivityDetail[];
  selectedMember: PathMemberSummary | null;
  timer: TimerProjection;
}>;

type ActivityDeletionTarget = Readonly<{
  activityId: string;
  ownerId: string;
  pathId: string;
}>;

export type LoadedActivityDeletionResult = LoadedActivityDeletionState & Readonly<{
  removedFeedEventIds: readonly string[];
  removedUnreadCount: number;
}>;

function reconcileMember(
  member: PathMemberSummary,
  target: ActivityDeletionTarget,
  mutation: Extract<ActivityDeletionMutationResult, { kind: 'applied' }>,
): PathMemberSummary {
  if (member.userId !== target.ownerId || member.pathId !== target.pathId) return member;
  return {
    ...member,
    intervalProgress: mutation.result.intervalProgress,
    overallProgress: member.overallProgress ? {
      accumulatedSeconds: mutation.result.accumulatedSeconds,
      targetSeconds: member.overallProgress.targetSeconds,
    } : undefined,
    sessionCount: mutation.result.sessionCount,
    totalTrackedSeconds: mutation.result.accumulatedSeconds,
  };
}

export function applyLoadedActivityDeletionResult(
  state: LoadedActivityDeletionState,
  mutation: Extract<ActivityDeletionMutationResult, { kind: 'applied' }>,
  target: ActivityDeletionTarget,
): LoadedActivityDeletionResult;
export function applyLoadedActivityDeletionResult(
  state: LoadedActivityDeletionState,
  mutation: ActivityDeletionMutationResult,
  target: ActivityDeletionTarget,
): LoadedActivityDeletionState | LoadedActivityDeletionResult;
export function applyLoadedActivityDeletionResult(
  state: LoadedActivityDeletionState,
  mutation: ActivityDeletionMutationResult,
  target: ActivityDeletionTarget,
): LoadedActivityDeletionState | LoadedActivityDeletionResult {
  if (mutation.kind !== 'applied') return state;

  const removedFeedEventIds = new Set(mutation.result.removedFeedEventIds);
  const base = applyActivityDeletionResult(
    { activities: state.activities, timer: state.timer },
    mutation,
    target.activityId,
  );
  const removedNotifications = state.notifications.items.filter((item) =>
    'socialFeedEventId' in item && removedFeedEventIds.has(item.socialFeedEventId));
  const removedUnreadCount = removedNotifications.filter(({ read }) => !read).length;
  const members = state.members.map((member) => reconcileMember(member, target, mutation));

  return {
    activities: base.activities,
    comments: {
      ...state.comments,
      items: state.comments.items.filter(({ eventId }) => !removedFeedEventIds.has(eventId)),
    },
    feed: {
      ...state.feed,
      items: state.feed.items.filter(({ id }) => !removedFeedEventIds.has(id)),
    },
    members,
    notifications: {
      ...state.notifications,
      items: state.notifications.items.filter((item) =>
        !('socialFeedEventId' in item) || !removedFeedEventIds.has(item.socialFeedEventId)),
      unreadCount: mutation.result.unreadNotificationCount,
    },
    pathMemberActivities: state.pathMemberActivities.filter(({ activity }) => activity.id !== target.activityId),
    removedFeedEventIds: mutation.result.removedFeedEventIds,
    removedUnreadCount,
    selectedMember: state.selectedMember
      ? reconcileMember(state.selectedMember, target, mutation)
      : null,
    timer: base.timer,
  };
}
