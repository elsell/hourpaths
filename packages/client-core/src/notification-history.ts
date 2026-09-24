import { NUDGE_PRESETS, type NudgeContent, type NudgePreset } from './nudges';

export type PathInvitationNotificationType =
  | 'path_invitation_received'
  | 'path_invitation_accepted';

export type PathOwnershipTransferNotificationType =
  | 'path_ownership_transfer_received'
  | 'path_ownership_transfer_accepted'
  | 'path_ownership_transfer_declined'
  | 'path_ownership_transfer_canceled';

export type PathDeletionNotificationType = 'path_deleted';
export type PathMemberAccessNotificationType = 'path_member_role_changed' | 'path_member_removed';
export type PathVisibilityChangedNotificationType = 'path_visibility_changed';
export type NudgeNotificationType = 'nudge_received';
export type NotificationPracticeReaction = 'heart' | 'applause' | 'fire' | 'strong' | 'celebrate';
export type SocialNotificationType =
  | 'new_follower'
  | 'follow_request_received'
  | 'follow_request_accepted'
  | 'practice_reaction'
  | 'practice_comment'
  | 'comment_heart';

export type NotificationPresentation = 'actionable' | 'informational';
export type NotificationPathRole = 'participant' | 'supporter';
export type NotificationPathAccessRole = 'administrator' | NotificationPathRole;

export type NotificationPublicIdentity = Readonly<{
  userId: string;
  username: string;
  displayName: string;
}>;

type NotificationBase = Readonly<{
  id: string;
  presentation: NotificationPresentation;
  read: boolean;
  createdAt: string;
  actor: NotificationPublicIdentity;
}>;

type PathNotificationBase = NotificationBase & Readonly<{
  pathName: string;
}>;

type InvitationNotification = PathNotificationBase & Readonly<{
  type: PathInvitationNotificationType;
  pathId: string;
  invitationId: string;
  offeredRole: NotificationPathRole;
}>;

type OwnershipTransferNotification = PathNotificationBase & Readonly<{
  type: PathOwnershipTransferNotificationType;
  pathId: string;
  ownershipTransferId: string;
}>;

type PathDeletionNotification = PathNotificationBase & Readonly<{
  type: PathDeletionNotificationType;
  presentation: 'informational';
}>;

type PathMemberAccessNotification = PathNotificationBase & (
  | Readonly<{
    type: 'path_member_role_changed';
    presentation: 'informational';
    pathId: string;
    offeredRole: NotificationPathAccessRole;
  }>
  | Readonly<{
    type: 'path_member_removed';
    presentation: 'informational';
    pathId: string;
    offeredRole: NotificationPathRole;
  }>
);

type PathVisibilityChangedNotification = PathNotificationBase & Readonly<{
  type: PathVisibilityChangedNotificationType;
  presentation: 'informational';
  pathId: string;
  pathVisibility: 'private' | 'followers' | 'public';
}>;

type PracticeReactionNotification = PathNotificationBase & Readonly<{
  type: 'practice_reaction';
  presentation: 'informational';
  pathId: string;
  socialFeedEventId: string;
  reaction: NotificationPracticeReaction;
}>;

type PracticeCommentNotification = PathNotificationBase & Readonly<{
  type: 'practice_comment' | 'comment_heart';
  presentation: 'informational';
  pathId: string;
  socialFeedEventId: string;
  commentId: string;
}>;

export type NudgeNotification = PathNotificationBase & Readonly<{
  type: NudgeNotificationType;
  presentation: 'informational';
  pathId: string;
  content: NudgeContent;
}>;

type SocialNotification =
  | (NotificationBase & Readonly<{
      type: 'new_follower';
      presentation: 'informational';
    }>)
  | (NotificationBase & Readonly<{
      type: 'follow_request_received';
      presentation: 'actionable';
      followRequestId: string;
    }>)
  | (NotificationBase & Readonly<{
      type: 'follow_request_accepted';
      presentation: 'informational';
      followRequestId: string;
    }>);

export type NotificationHistoryItem =
  | InvitationNotification
  | OwnershipTransferNotification
  | PathDeletionNotification
  | PathMemberAccessNotification
  | PathVisibilityChangedNotification
  | PracticeReactionNotification
  | PracticeCommentNotification
  | NudgeNotification
  | SocialNotification;

/** @deprecated Use NotificationHistoryItem. Retained for source compatibility. */
export type PathInvitationNotification = NotificationHistoryItem;

export type NotificationHistoryState = Readonly<{
  items: readonly PathInvitationNotification[];
  nextCursor: string;
  unreadCount: number;
}>;

export type NotificationHistoryPage = Readonly<{
  items: readonly PathInvitationNotification[];
  nextCursor: string;
  unreadCount: number;
}>;

export type NotificationMutation =
  | Readonly<{ kind: 'read'; notificationId: string }>
  | Readonly<{ kind: 'delete'; notificationId: string }>
  | Readonly<{ kind: 'read-all' }>;

export type NotificationMutationState = Readonly<{
  history: NotificationHistoryState;
  unreadCount: number;
}>;

export type NotificationPresentationMessageKey =
  | 'notification.pathInvitationReceived.participant'
  | 'notification.pathInvitationReceived.supporter'
  | 'notification.pathInvitationAccepted.participant'
  | 'notification.pathInvitationAccepted.supporter'
  | 'notification.pathOwnershipTransferReceived'
  | 'notification.pathOwnershipTransferAccepted'
  | 'notification.pathOwnershipTransferDeclined'
  | 'notification.pathOwnershipTransferCanceled'
  | 'notification.pathDeleted'
  | 'notification.pathMemberRoleChanged.participant'
  | 'notification.pathMemberRoleChanged.supporter'
  | 'notification.pathMemberRoleChanged.administrator'
  | 'notification.pathMemberRemoved.participant'
  | 'notification.pathMemberRemoved.supporter'
  | 'notification.pathVisibilityChanged'
  | 'notification.newFollower'
  | 'notification.followRequestReceived'
  | 'notification.followRequestAccepted'
  | 'notification.practiceReaction.heart'
  | 'notification.practiceReaction.applause'
  | 'notification.practiceReaction.fire'
  | 'notification.practiceReaction.strong'
  | 'notification.practiceReaction.celebrate'
  | 'notification.practiceComment'
  | 'notification.commentHeart'
  | `notification.nudge.${NudgePreset}`;

const notificationBaseKeys = [
  'actor',
  'createdAt',
  'id',
  'presentation',
  'read',
  'type',
] as const;

const invitationNotificationKeys = [...notificationBaseKeys, 'invitationId', 'offeredRole', 'pathId', 'pathName'].sort();
const ownershipTransferNotificationKeys = [...notificationBaseKeys, 'ownershipTransferId', 'pathId', 'pathName'].sort();
const pathDeletionNotificationKeys = [...notificationBaseKeys, 'pathName'].sort();
const pathMemberAccessNotificationKeys = [...notificationBaseKeys, 'offeredRole', 'pathId', 'pathName'].sort();
const pathVisibilityChangedNotificationKeys = [...notificationBaseKeys, 'pathId', 'pathName', 'pathVisibility'].sort();
const newFollowerNotificationKeys = [...notificationBaseKeys].sort();
const followRequestNotificationKeys = [...notificationBaseKeys, 'followRequestId'].sort();
const practiceReactionNotificationKeys = [
  ...notificationBaseKeys,
  'pathId',
  'pathName',
  'reaction',
  'socialFeedEventId',
].sort();
const practiceCommentNotificationKeys = [
  ...notificationBaseKeys,
  'commentId',
  'pathId',
  'pathName',
  'socialFeedEventId',
].sort();
const nudgeNotificationKeys = [...notificationBaseKeys, 'content', 'pathId', 'pathName'].sort();

function validPracticeReaction(value: unknown): value is NotificationPracticeReaction {
  return value === 'heart' || value === 'applause' || value === 'fire' ||
    value === 'strong' || value === 'celebrate';
}

function validatedNudgeContent(value: unknown): NudgeContent | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  if (
    !hasExactKeys(record, ['kind', 'preset']) ||
    record.kind !== 'preset' ||
    typeof record.preset !== 'string' ||
    !NUDGE_PRESETS.includes(record.preset as NudgePreset)
  ) return undefined;
  return Object.freeze({ kind: 'preset', preset: record.preset as NudgePreset });
}

function hasExactKeys(record: Record<string, unknown>, keys: readonly string[]): boolean {
  const actual = Object.keys(record).sort();
  return actual.length === keys.length && actual.every((key, index) => key === keys[index]);
}

function validText(value: unknown): value is string {
  return typeof value === 'string' &&
    value.length > 0 &&
    value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function validInstant(value: unknown): value is string {
  return validText(value) &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u.test(value) &&
    Number.isFinite(Date.parse(value));
}

function validCursor(value: unknown): value is string {
  return typeof value === 'string' &&
    value.length <= 4096 &&
    value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function validatedActor(value: unknown): NotificationPublicIdentity | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  if (
    !hasExactKeys(record, ['displayName', 'userId', 'username']) ||
    !validText(record.userId) ||
    !validText(record.username) ||
    !validText(record.displayName)
  ) {
    return undefined;
  }
  return Object.freeze({
    userId: record.userId,
    username: record.username,
    displayName: record.displayName,
  });
}

function validatedNotification(value: unknown): PathInvitationNotification | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  if (
    !validText(record.id) ||
    !validInstant(record.createdAt) ||
    typeof record.read !== 'boolean'
  ) {
    return undefined;
  }
  const actor = validatedActor(record.actor);
  if (!actor) return undefined;

  if (record.type === 'new_follower') {
    if (record.presentation !== 'informational' || !hasExactKeys(record, newFollowerNotificationKeys)) return undefined;
    return Object.freeze({ id: record.id, type: 'new_follower', presentation: 'informational', read: record.read, createdAt: record.createdAt, actor });
  }
  if (record.type === 'follow_request_received' || record.type === 'follow_request_accepted') {
    const expectedPresentation = record.type === 'follow_request_received' ? 'actionable' : 'informational';
    if (record.presentation !== expectedPresentation || !hasExactKeys(record, followRequestNotificationKeys) || !validText(record.followRequestId)) return undefined;
    if (record.type === 'follow_request_received') {
      return Object.freeze({ id: record.id, type: 'follow_request_received', presentation: 'actionable', read: record.read, createdAt: record.createdAt, actor, followRequestId: record.followRequestId });
    }
    return Object.freeze({ id: record.id, type: 'follow_request_accepted', presentation: 'informational', read: record.read, createdAt: record.createdAt, actor, followRequestId: record.followRequestId });
  }

  if (record.type === 'path_deleted') {
    if (record.presentation !== 'informational' || !hasExactKeys(record, pathDeletionNotificationKeys) || !validText(record.pathName)) {
      return undefined;
    }
    return Object.freeze({
      id: record.id,
      type: 'path_deleted',
      presentation: 'informational',
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathName: record.pathName,
    });
  }

  if (!validText(record.pathId) || !validText(record.pathName)) return undefined;

  if (record.type === 'nudge_received') {
    const content = validatedNudgeContent(record.content);
    if (
      record.presentation !== 'informational' ||
      !hasExactKeys(record, nudgeNotificationKeys) ||
      !content
    ) return undefined;
    return Object.freeze({
      id: record.id,
      type: 'nudge_received',
      presentation: 'informational',
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathId: record.pathId,
      pathName: record.pathName,
      content,
    });
  }

  if (record.type === 'path_member_role_changed' || record.type === 'path_member_removed') {
    const validRole = record.offeredRole === 'participant' || record.offeredRole === 'supporter'
      || (record.type === 'path_member_role_changed' && record.offeredRole === 'administrator');
    if (record.presentation !== 'informational' || !hasExactKeys(record, pathMemberAccessNotificationKeys)
      || !validRole) return undefined;
    return Object.freeze({
      id: record.id,
      type: record.type,
      presentation: 'informational',
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathId: record.pathId,
      pathName: record.pathName,
      offeredRole: record.offeredRole as NotificationPathAccessRole,
    }) as PathMemberAccessNotification;
  }

  if (record.type === 'path_visibility_changed') {
    if (
      record.presentation !== 'informational' ||
      !hasExactKeys(record, pathVisibilityChangedNotificationKeys) ||
      (record.pathVisibility !== 'private' && record.pathVisibility !== 'followers' && record.pathVisibility !== 'public')
    ) return undefined;
    return Object.freeze({
      id: record.id,
      type: 'path_visibility_changed',
      presentation: 'informational',
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathId: record.pathId,
      pathName: record.pathName,
      pathVisibility: record.pathVisibility,
    });
  }

  if (record.type === 'practice_reaction') {
    if (
      record.presentation !== 'informational' ||
      !hasExactKeys(record, practiceReactionNotificationKeys) ||
      !validText(record.socialFeedEventId) ||
      !validPracticeReaction(record.reaction)
    ) return undefined;
    return Object.freeze({
      id: record.id,
      type: 'practice_reaction',
      presentation: 'informational',
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathId: record.pathId,
      pathName: record.pathName,
      socialFeedEventId: record.socialFeedEventId,
      reaction: record.reaction,
    });
  }

  if (record.type === 'practice_comment' || record.type === 'comment_heart') {
    if (
      record.presentation !== 'informational' ||
      !hasExactKeys(record, practiceCommentNotificationKeys) ||
      !validText(record.socialFeedEventId) ||
      !validText(record.commentId)
    ) return undefined;
    return Object.freeze({
      id: record.id,
      type: record.type,
      presentation: 'informational',
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathId: record.pathId,
      pathName: record.pathName,
      socialFeedEventId: record.socialFeedEventId,
      commentId: record.commentId,
    });
  }

  const invitationReceived = record.type === 'path_invitation_received' &&
    record.presentation === 'actionable';
  const invitationAccepted = record.type === 'path_invitation_accepted' &&
    record.presentation === 'informational';
  if (invitationReceived || invitationAccepted) {
    if (
      !hasExactKeys(record, invitationNotificationKeys) ||
      !validText(record.invitationId) ||
      (record.offeredRole !== 'participant' && record.offeredRole !== 'supporter')
    ) return undefined;
    return Object.freeze({
      id: record.id,
      type: record.type as PathInvitationNotificationType,
      presentation: record.presentation as NotificationPresentation,
      read: record.read,
      createdAt: record.createdAt,
      actor,
      pathId: record.pathId,
      pathName: record.pathName,
      invitationId: record.invitationId,
      offeredRole: record.offeredRole,
    });
  }

  const transferReceived = record.type === 'path_ownership_transfer_received' &&
    record.presentation === 'actionable';
  const transferInformational = (
    record.type === 'path_ownership_transfer_accepted' ||
    record.type === 'path_ownership_transfer_declined' ||
    record.type === 'path_ownership_transfer_canceled'
  ) && record.presentation === 'informational';
  if (
    (!transferReceived && !transferInformational) ||
    !hasExactKeys(record, ownershipTransferNotificationKeys) ||
    !validText(record.ownershipTransferId)
  ) return undefined;

  return Object.freeze({
    id: record.id,
    type: record.type as PathOwnershipTransferNotificationType,
    presentation: record.presentation as NotificationPresentation,
    read: record.read,
    createdAt: record.createdAt,
    actor,
    pathId: record.pathId,
    pathName: record.pathName,
    ownershipTransferId: record.ownershipTransferId,
  });
}

export function mergeNotificationHistoryPage(
  state: NotificationHistoryState,
  value: unknown,
  requestedCursor: unknown,
): NotificationHistoryState {
  if (!validCursor(requestedCursor)) throw new Error('invalid notification cursor');
  if (requestedCursor !== state.nextCursor) throw new Error('stale notification cursor');
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid notification page');
  }
  const page = value as Record<string, unknown>;
  if (
    !hasExactKeys(page, ['items', 'nextCursor', 'unreadCount']) ||
    !Array.isArray(page.items) ||
    !validCursor(page.nextCursor) ||
    !Number.isSafeInteger(page.unreadCount) ||
    (page.unreadCount as number) < 0
  ) {
    throw new Error('invalid notification page');
  }
  const validated = page.items.map((item) => {
    const notification = validatedNotification(item);
    if (!notification) throw new Error('invalid notification');
    return notification;
  });
  const seenPageIDs = new Set<string>();
  if (validated.some((item) => seenPageIDs.size === seenPageIDs.add(item.id).size)) {
    throw new Error('invalid notification');
  }
  for (let index = 1; index < validated.length; index += 1) {
    if (Date.parse(validated[index - 1]!.createdAt) < Date.parse(validated[index]!.createdAt)) {
      throw new Error('invalid notification');
    }
  }

  const byID = new Map(validated.map((item) => [item.id, item]));
  const items = state.items
    .map((item) => byID.get(item.id) ?? item)
    .concat(validated.filter((item) => !state.items.some((current) => current.id === item.id)));
  return Object.freeze({
    items: Object.freeze(items),
    nextCursor: page.nextCursor,
    unreadCount: requestedCursor ? state.unreadCount : page.unreadCount as number,
  });
}

export function notificationPresentationMessageKey(
  notification: PathInvitationNotification,
): NotificationPresentationMessageKey {
  switch (notification.type) {
  case 'new_follower':
    return 'notification.newFollower';
  case 'follow_request_received':
    return 'notification.followRequestReceived';
  case 'follow_request_accepted':
    return 'notification.followRequestAccepted';
  case 'practice_reaction':
    return `notification.practiceReaction.${notification.reaction}`;
  case 'practice_comment':
    return 'notification.practiceComment';
  case 'comment_heart':
    return 'notification.commentHeart';
  case 'nudge_received':
    return `notification.nudge.${notification.content.preset}`;
  case 'path_deleted':
    return 'notification.pathDeleted';
  case 'path_member_role_changed':
    return `notification.pathMemberRoleChanged.${notification.offeredRole}`;
  case 'path_member_removed':
    return `notification.pathMemberRemoved.${notification.offeredRole}`;
  case 'path_visibility_changed':
    return 'notification.pathVisibilityChanged';
  case 'path_ownership_transfer_received':
    return 'notification.pathOwnershipTransferReceived';
  case 'path_ownership_transfer_accepted':
    return 'notification.pathOwnershipTransferAccepted';
  case 'path_ownership_transfer_declined':
    return 'notification.pathOwnershipTransferDeclined';
  case 'path_ownership_transfer_canceled':
    return 'notification.pathOwnershipTransferCanceled';
  }
  const event = notification.type === 'path_invitation_received'
    ? 'Received'
    : 'Accepted';
  return `notification.pathInvitation${event}.${notification.offeredRole}`;
}

export function applyNotificationMutation(
  history: NotificationHistoryState,
  mutation: NotificationMutation,
  value: unknown,
): NotificationMutationState {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid notification mutation result');
  }
  const result = value as Record<string, unknown>;
  if (
    !hasExactKeys(result, ['unreadCount']) ||
    !Number.isSafeInteger(result.unreadCount) ||
    (result.unreadCount as number) < 0
  ) {
    throw new Error('invalid notification mutation result');
  }
  if (
    mutation.kind !== 'read-all' &&
    !history.items.some(({ id }) => id === mutation.notificationId)
  ) {
    throw new Error('unknown notification');
  }

  const items = mutation.kind === 'read-all'
    ? history.items.map((notification) =>
        notification.read ? notification : Object.freeze({ ...notification, read: true }))
    : mutation.kind === 'delete'
      ? history.items.filter(({ id }) => id !== mutation.notificationId)
      : history.items.map((notification) =>
          notification.id === mutation.notificationId && !notification.read
            ? Object.freeze({ ...notification, read: true })
            : notification);
  return Object.freeze({
    history: Object.freeze({
      items: Object.freeze(items),
      nextCursor: history.nextCursor,
      unreadCount: result.unreadCount as number,
    }),
    unreadCount: result.unreadCount as number,
  });
}
