import createClient from 'openapi-fetch';
import type { components, paths } from './schema';
import type { PathCreateBody } from './path-submission';
export { sessionExpiryAdvanced, sessionRefreshDelay, sessionRefreshLeadMs } from './session';
export {
  createPathSubmissionOwner,
  type CreatedPath,
  type PathCreateBody,
  type PathCreateDraft,
  type PathCreateRequest,
  type PathGoalAlignment,
  type PathIntervalGoalDraft,
  type PathOverallTargetDraft,
  type PathRecurrence,
  type PathSubmissionResult,
} from './path-submission';
export { createTimerOperationOwner, elapsedTimerSeconds, formatTimerDuration, timerMutationPresentation, type TimerMutationResult, type TimerState, type TimerStopResult } from './timer-control';

export type TokenProvider = () => Promise<string | null> | string | null;
export type OnboardingActivationInput = components['schemas']['OnboardingActivationInputBody'];
export type OnboardingProfile = components['schemas']['OnboardingProfileDTO'];
export type PathCapabilities = components['schemas']['PathCapabilities'];
export type PathProjection = components['schemas']['PathProjection'];
export type HomePreferences = components['schemas']['HomePreferences'];
export type HomePreferencesUpdate = Omit<components['schemas']['HomePreferencesUpdate'], '$schema'>;
export type PathRenameBody = Omit<components['schemas']['PathRename'], '$schema'>;
export type PathVisibilityUpdate = Omit<components['schemas']['PathVisibilityUpdate'], '$schema'>;
export type PathDeleteBody = Omit<components['schemas']['PathDelete'], '$schema'>;
export type PathDeletionReceipt = components['schemas']['PathDeletionReceipt'];
export type PathLeaveBody = Omit<components['schemas']['PathLeave'], '$schema'>;
export type PathLeaveReceipt = components['schemas']['PathLeaveReceipt'];
export type PathMember = components['schemas']['PathMember'];
export type MemberRemovalReview = components['schemas']['MemberRemovalReview'];
export type MemberRemovalBody = Omit<components['schemas']['MemberRemoval'], '$schema'>;
export type MemberRemovalReceipt = components['schemas']['MemberRemovalReceipt'];
export type MemberRoleChangeBody = Omit<components['schemas']['MemberRoleChange'], '$schema'>;
export type MemberRoleChangeReceipt = components['schemas']['MemberRoleChangeReceipt'];
export type SessionPath = PathProjection;
export type PathInvitation = components['schemas']['PathInvitation'];
export type PathInvitationAccept = Omit<components['schemas']['PathInvitationAccept'], '$schema'>;
export type PathInvitationRejection = components['schemas']['PathInvitationRejection'];
export type PathInvitationCreate = Omit<components['schemas']['PathInvitationCreate'], '$schema'>;
export type PathInvitationRecipient = components['schemas']['PathInvitationRecipient'];
export type PendingPathInvitation = components['schemas']['PendingPathInvitation'];
export type ManagedPathInvitation = components['schemas']['ManagedPathInvitation'];
export type PathInvitationCancellation = components['schemas']['PathInvitationCancellation'];
export type PathInvitationNotification = components['schemas']['PathInvitationNotification'];
export type NotificationMutationResult = components['schemas']['NotificationMutationResult'];
export type Nudge = components['schemas']['Nudge'];
export type NudgeAudiencePreference = components['schemas']['NudgeAudiencePreference'];
export type NudgeAudiencePreferenceInput = Omit<components['schemas']['NudgeAudiencePreferenceInput'], '$schema'>;
export type NudgeEligibility = components['schemas']['NudgeEligibility'];
export type NudgeInput = Omit<components['schemas']['NudgeInput'], '$schema'>;
export type NudgeNotificationChannelPreference = components['schemas']['NudgeNotificationChannelPreference'];
export type NudgeNotificationChannelPreferenceInput = Omit<components['schemas']['NudgeNotificationChannelPreferenceInput'], '$schema'>;
export type BlockReviewAcknowledgement = components['schemas']['BlockReviewAcknowledgement'];
export type PublicProfile = components['schemas']['PublicProfile'];
export type FollowRequest = components['schemas']['FollowRequest'];
export type PracticeFeedItem = components['schemas']['PracticeFeedItem'];
export type InteractionSettings = Omit<components['schemas']['InteractionSettings'], '$schema'>;
export type ConfiguredTimeZone = components['schemas']['TimeZonePreferenceDTO'];
export type ConfiguredTimeZoneUpdate =
  Omit<components['schemas']['TimeZonePreferenceInputBody'], '$schema' | 'confirmed'> & { confirmed: true };
export type PracticeReaction = components['schemas']['PracticeReactionInput']['reaction'];
export type PracticeReactionSummary = components['schemas']['PracticeReactionSummary'];
export type PracticeComment = components['schemas']['PracticeComment'];
export type PracticeCommentItem = components['schemas']['PracticeCommentItem'];
export type PracticeCommentInput = Omit<components['schemas']['PracticeCommentInput'], '$schema'>;
export type PracticeCommentEdit = Omit<components['schemas']['PracticeCommentEditInput'], '$schema'>;
export type PracticeCommentVersion = components['schemas']['CommentVersion'];
export type PracticeCommentHeartState = components['schemas']['CommentHeartSummary'];
export type PracticeCommentHeartRosterItem = components['schemas']['PublicProfile'];
export type ActiveFollowingItem = components['schemas']['ActiveFollowingItem'];
export type ActiveFollowingTimer = components['schemas']['ActiveFollowingTimer'];
export type BlockTarget = components['schemas']['BlockTarget'];
export type BlockSharedPath = components['schemas']['BlockSharedPath'];
export type BlockedAccount = components['schemas']['BlockedAccount'];
export type OwnershipTransfer = components['schemas']['OwnershipTransfer'];
export type OwnershipTransferCandidate = components['schemas']['OwnershipTransferCandidate'];
export type OwnershipTransferReview = components['schemas']['OwnershipTransferReview'];
export type OwnershipTransferReviewRequest = Omit<components['schemas']['OwnershipTransferReviewRequest'], '$schema'>;
export type OwnershipTransferResult = components['schemas']['OwnershipTransferResult'];
export type OwnershipTransferCreate = Omit<components['schemas']['OwnershipTransferCreate'], '$schema'>;
export type PushInstallationInput =
  Omit<components['schemas']['PushInstallationInputBody'], '$schema'>;
export type ManualActivityInput = components['schemas']['ManualActivityInputBody'];
export type ManualActivityDefaults = components['schemas']['ManualActivityDefaults'];
export type ActivityMutationResult = components['schemas']['ActivityMutationResult'];
export type ActivityDeletionResult = components['schemas']['ActivityDeletionResult'];
export type ActivityDetail = components['schemas']['ActivityDetail'];
export type ActivityRevision = components['schemas']['ActivityRevision'];
export type PathGoalUpdateDraft =
  Omit<components['schemas']['PathGoalsUpdate'], '$schema' | 'confirmed'> & { confirmed: true };
export type PathGoalMutationResult = components['schemas']['PathGoalMutationResult'];
export type PathArchiveStateDraft =
  Omit<components['schemas']['PathArchiveStateUpdate'], '$schema' | 'confirmed'> & { confirmed: true };
export function createApiClient(baseUrl: string, tokenProvider: TokenProvider) {
  return createClient<paths>({
    baseUrl,
    fetch: async (input, init = {}) => {
      const token = await tokenProvider();
      const requestInit = init as RequestInit;
      const headers = new Headers(input instanceof Request ? input.headers : undefined);
      new Headers(requestInit.headers).forEach((value, key) => headers.set(key, value));
      headers.delete('Authorization');
      if (token) headers.set('Authorization', `Bearer ${token}`);
      return fetch(input, { ...requestInit, headers });
    }
  });
}

export type GeneratedOperationResult<T> = {
  data?: T;
  error?: unknown;
  response: Response;
};

export function generatedResponse<T>(result: GeneratedOperationResult<T>) {
  return {
    ok: result.response.ok,
    status: result.response.status,
    problem: result.error,
    json: async (): Promise<T | undefined> => result.data,
  };
}

export function createSessionApiClient(baseUrl: string, tokenProvider: TokenProvider) {
  const publicClient = createApiClient(baseUrl, () => null);
  const authenticatedClient = createApiClient(baseUrl, tokenProvider);
  return {
    exchange: (identityToken: string) => publicClient.POST('/v1/sessions', {
      body: { identityToken },
    }),
    refresh: () => authenticatedClient.POST('/v1/session/refresh'),
    profile: () => authenticatedClient.GET('/v1/me'),
    configuredTimeZone: () => authenticatedClient.GET('/v1/me/time-zone'),
    updateConfiguredTimeZone: (body: ConfiguredTimeZoneUpdate, idempotencyKey: string) => authenticatedClient.PUT('/v1/me/time-zone', {
      params: { header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    onboarding: () => authenticatedClient.GET('/v1/onboarding'),
    activateOnboarding: (body: OnboardingActivationInput) => authenticatedClient.POST('/v1/onboarding/activation', { body }),
    paths: (cursor?: string) => authenticatedClient.GET('/v1/paths', { params: { query: { cursor, limit: 25 } } }),
    updateHomePreferences: (body: HomePreferencesUpdate, idempotencyKey: string) => authenticatedClient.PUT('/v1/me/home-preferences', {
      params: { header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    archivedPaths: (cursor?: string) => authenticatedClient.GET('/v1/paths', {
      params: { query: { archived: true, cursor, limit: 25 } },
    }),
    path: (pathId: string) => authenticatedClient.GET('/v1/paths/{id}', {
      params: { path: { id: pathId } },
    }),
    pathMembers: (pathId: string, cursor?: string) => authenticatedClient.GET('/v1/paths/{pathId}/members', {
      params: { path: { pathId }, query: { cursor, limit: 25 } },
    }),
    reviewPathMemberRemoval: (pathId: string, userId: string) => authenticatedClient.GET('/v1/paths/{pathId}/members/{userId}/removal-review', {
      params: { path: { pathId, userId } },
    }),
    removePathMember: (pathId: string, userId: string, body: MemberRemovalBody, idempotencyKey: string) => authenticatedClient.DELETE('/v1/paths/{pathId}/members/{userId}', {
      params: { path: { pathId, userId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    changePathMemberRole: (pathId: string, userId: string, body: MemberRoleChangeBody, idempotencyKey: string) => authenticatedClient.PATCH('/v1/paths/{pathId}/members/{userId}', {
      params: { path: { pathId, userId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    reviewPathInvitationRecipient: (pathId: string, username: string) => authenticatedClient.GET('/v1/paths/{pathId}/invitation-recipient', {
      params: { path: { pathId }, query: { username } },
    }),
    sendPathInvitation: (pathId: string, body: PathInvitationCreate, idempotencyKey: string) => authenticatedClient.POST('/v1/paths/{pathId}/invitations', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    managedPathInvitations: (pathId: string, cursor?: string, limit = 25) => authenticatedClient.GET('/v1/paths/{pathId}/invitations', {
      params: { path: { pathId }, query: { cursor, limit } },
    }),
    cancelPathInvitation: (pathId: string, invitationId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/paths/{pathId}/invitations/{invitationId}', {
      params: { path: { pathId, invitationId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    pendingPathInvitations: (cursor?: string) => authenticatedClient.GET('/v1/path-invitations', {
      params: { query: { cursor, limit: 25 } },
    }),
    notifications: (cursor?: string) => authenticatedClient.GET('/v1/notifications', {
      params: { query: { cursor, limit: 25 } },
    }),
    getNudgeNotificationChannel: () => authenticatedClient.GET('/v1/me/notification-channels/nudges'),
    updateNudgeNotificationChannel: (body: NudgeNotificationChannelPreferenceInput, idempotencyKey: string) => authenticatedClient.PUT('/v1/me/notification-channels/nudges', {
      params: { header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    getPathNudgePreference: (pathId: string) => authenticatedClient.GET('/v1/paths/{pathId}/nudge-preference', {
      params: { path: { pathId } },
    }),
    updatePathNudgePreference: (pathId: string, body: NudgeAudiencePreferenceInput, idempotencyKey: string) => authenticatedClient.PUT('/v1/paths/{pathId}/nudge-preference', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    getPathMemberNudgeEligibility: (pathId: string, userId: string) => authenticatedClient.GET('/v1/paths/{pathId}/members/{userId}/nudge-eligibility', {
      params: { path: { pathId, userId } },
    }),
    sendPathMemberNudge: (pathId: string, userId: string, body: NudgeInput, idempotencyKey: string) => authenticatedClient.POST('/v1/paths/{pathId}/members/{userId}/nudges', {
      params: { path: { pathId, userId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    searchProfiles: (query: string, cursor?: string) => authenticatedClient.GET('/v1/profiles', {
      params: { query: { query, cursor, limit: 25 } },
    }),
    profileByUsername: (username: string) => authenticatedClient.GET('/v1/profiles/{username}', {
      params: { path: { username } },
    }),
    reviewProfileBlock: (username: string) => authenticatedClient.GET('/v1/profiles/{username}/block-review', {
      params: { path: { username } },
    }),
    blockProfile: (username: string, idempotencyKey: string, acknowledgement: BlockReviewAcknowledgement) => authenticatedClient.POST('/v1/profiles/{username}/block', {
      params: { path: { username }, header: { 'Idempotency-Key': idempotencyKey } },
      body: { acknowledgement },
    }),
    blockedAccounts: (cursor?: string) => authenticatedClient.GET('/v1/blocked-accounts', {
      params: { query: { cursor, limit: 25 } },
    }),
    unblockAccount: (userId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/blocked-accounts/{userId}', {
      params: { path: { userId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    followProfile: (username: string, idempotencyKey: string) => authenticatedClient.POST('/v1/profiles/{username}/follow', {
      params: { path: { username }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    cancelFollowRequest: (username: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/profiles/{username}/follow-request', {
      params: { path: { username }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    unfollowProfile: (username: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/profiles/{username}/follow', {
      params: { path: { username }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    followRequests: (cursor?: string) => authenticatedClient.GET('/v1/follow-requests', {
      params: { query: { cursor, limit: 25 } },
    }),
	 socialFeed: (cursor?: string) => authenticatedClient.GET('/v1/social/feed', {
      params: { query: { cursor, limit: 25 } },
    }),
    getPracticeFeedEvent: (eventId: string) => authenticatedClient.GET('/v1/social/feed/{eventId}', {
      params: { path: { eventId } },
    }),
    getInteractionSettings: () => authenticatedClient.GET('/v1/me/interaction-settings'),
    updateInteractionSettings: (body: InteractionSettings, idempotencyKey: string) => authenticatedClient.PUT('/v1/me/interaction-settings', {
      params: { header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    socialFeedComments: (eventId: string, cursor?: string) => authenticatedClient.GET('/v1/social/feed/{eventId}/comments', {
      params: { path: { eventId }, query: { cursor, limit: 25 } },
    }),
    createSocialFeedComment: (eventId: string, body: PracticeCommentInput, idempotencyKey: string) => authenticatedClient.POST('/v1/social/feed/{eventId}/comments', {
      params: { path: { eventId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    editSocialFeedComment: (eventId: string, commentId: string, body: PracticeCommentEdit, idempotencyKey: string) => authenticatedClient.PATCH('/v1/social/feed/{eventId}/comments/{commentId}', {
      params: { path: { eventId, commentId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    deleteSocialFeedComment: (eventId: string, commentId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/social/feed/{eventId}/comments/{commentId}', {
      params: { path: { eventId, commentId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    socialFeedCommentHistory: (eventId: string, commentId: string, cursor?: string) => authenticatedClient.GET('/v1/social/feed/{eventId}/comments/{commentId}/history', {
      params: { path: { eventId, commentId }, query: { cursor, limit: 25 } },
    }),
    setPracticeCommentHeart: (eventId: string, commentId: string, idempotencyKey: string) => authenticatedClient.PUT('/v1/social/feed/{eventId}/comments/{commentId}/heart', {
      params: { path: { eventId, commentId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    removePracticeCommentHeart: (eventId: string, commentId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/social/feed/{eventId}/comments/{commentId}/heart', {
      params: { path: { eventId, commentId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    practiceCommentHearts: (eventId: string, commentId: string, cursor?: string) => authenticatedClient.GET('/v1/social/feed/{eventId}/comments/{commentId}/hearts', {
      params: { path: { eventId, commentId }, query: { cursor, limit: 25 } },
    }),
    setSocialFeedReaction: (eventId: string, reaction: PracticeReaction, idempotencyKey: string) => authenticatedClient.PUT('/v1/social/feed/{eventId}/reaction', {
      params: { path: { eventId }, header: { 'Idempotency-Key': idempotencyKey } },
      body: { reaction },
    }),
    removeSocialFeedReaction: (eventId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/social/feed/{eventId}/reaction', {
      params: { path: { eventId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    socialActiveFollowing: (cursor?: string) => authenticatedClient.GET('/v1/social/feed/active', {
      params: { query: { cursor, limit: 25 } },
    }),
    acceptFollowRequest: (requestId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/follow-requests/{requestId}/accept', {
      params: { path: { requestId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    rejectFollowRequest: (requestId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/follow-requests/{requestId}/reject', {
      params: { path: { requestId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    getNotification: (notificationId: string) => authenticatedClient.GET('/v1/notifications/{notificationId}', {
      params: { path: { notificationId } },
    }),
    markNotificationRead: (notificationId: string) => authenticatedClient.PATCH('/v1/notifications/{notificationId}/read', {
      params: { path: { notificationId } },
    }),
    deleteNotification: (notificationId: string) => authenticatedClient.DELETE('/v1/notifications/{notificationId}', {
      params: { path: { notificationId } },
    }),
    markAllNotificationsRead: () => authenticatedClient.POST('/v1/notifications/read-all'),
    ownershipTransferCandidates: (pathId: string, cursor?: string) => authenticatedClient.GET('/v1/paths/{pathId}/ownership-transfer-candidates', {
      params: { path: { pathId }, query: { cursor, limit: 25 } },
    }),
    pendingOwnershipTransfer: (pathId: string) => authenticatedClient.GET('/v1/paths/{pathId}/ownership-transfer', {
      params: { path: { pathId } },
    }),
    reviewOwnershipTransfer: (pathId: string, body: OwnershipTransferReviewRequest) => authenticatedClient.POST('/v1/paths/{pathId}/ownership-transfer/review', {
      params: { path: { pathId } },
      body,
    }),
    initiateOwnershipTransfer: (pathId: string, body: OwnershipTransferCreate, idempotencyKey: string) => authenticatedClient.POST('/v1/paths/{pathId}/ownership-transfers', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    acceptOwnershipTransfer: (transferId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/path-ownership-transfers/{transferId}/accept', {
      params: { path: { transferId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    declineOwnershipTransfer: (transferId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/path-ownership-transfers/{transferId}/decline', {
      params: { path: { transferId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    cancelOwnershipTransfer: (transferId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/path-ownership-transfers/{transferId}/cancel', {
      params: { path: { transferId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    upsertPushInstallation: (installationId: string, body: PushInstallationInput) =>
      authenticatedClient.PUT('/v1/push-installations/{installationId}', {
        params: { path: { installationId } },
        body,
      }),
    deletePushInstallation: (installationId: string) =>
      authenticatedClient.DELETE('/v1/push-installations/{installationId}', {
        params: { path: { installationId } },
      }),
    acceptPathInvitation: (invitationId: string, idempotencyKey: string, body?: PathInvitationAccept) => authenticatedClient.POST('/v1/path-invitations/{invitationId}/accept', {
      params: { path: { invitationId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    rejectPathInvitation: (invitationId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/path-invitations/{invitationId}/reject', {
      params: { path: { invitationId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    createPath: (body: PathCreateBody, idempotencyKey: string) => authenticatedClient.POST('/v1/paths', {
      params: { header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    renamePath: (pathId: string, body: PathRenameBody, idempotencyKey: string) => authenticatedClient.PUT('/v1/paths/{pathId}/name', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    setPathVisibility: (pathId: string, body: PathVisibilityUpdate, idempotencyKey: string) => authenticatedClient.PUT('/v1/paths/{pathId}/visibility', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    deletePath: (pathId: string, body: PathDeleteBody, idempotencyKey: string) => authenticatedClient.DELETE('/v1/paths/{id}', {
      params: { path: { id: pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    leavePath: (pathId: string, body: PathLeaveBody, idempotencyKey: string) => authenticatedClient.DELETE('/v1/paths/{pathId}/membership', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    updatePathGoals: (pathId: string, body: PathGoalUpdateDraft, idempotencyKey: string) => authenticatedClient.PUT('/v1/paths/{pathId}/goals', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    setPathArchiveState: (pathId: string, body: PathArchiveStateDraft, idempotencyKey: string) => authenticatedClient.PUT('/v1/paths/{pathId}/archive-state', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    currentTimer: (pathId: string) => authenticatedClient.GET('/v1/paths/{pathId}/timer', {
      params: { path: { pathId } },
    }),
    startTimer: (pathId: string, idempotencyKey: string) => authenticatedClient.POST('/v1/paths/{pathId}/timer', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    stopTimer: (pathId: string, timerId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/paths/{pathId}/timer/{timerId}', {
      params: { path: { pathId, timerId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    createManualActivity: (pathId: string, body: ManualActivityInput, idempotencyKey: string) => authenticatedClient.POST('/v1/paths/{pathId}/activities', {
      params: { path: { pathId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    activities: (pathId: string, cursor?: string, participantId?: string) => authenticatedClient.GET('/v1/paths/{pathId}/activities', {
      params: { path: { pathId }, query: { participantId, cursor, limit: 25 } },
    }),
    manualActivityDefaults: (pathId: string) => authenticatedClient.GET('/v1/paths/{pathId}/activities/manual-defaults', {
      params: { path: { pathId } },
    }),
    updateActivity: (pathId: string, activityId: string, body: ManualActivityInput, idempotencyKey: string) => authenticatedClient.PUT('/v1/paths/{pathId}/activities/{activityId}', {
      params: { path: { pathId, activityId }, header: { 'Idempotency-Key': idempotencyKey } },
      body,
    }),
    deleteActivity: (pathId: string, activityId: string, idempotencyKey: string) => authenticatedClient.DELETE('/v1/paths/{pathId}/activities/{activityId}', {
      params: { path: { pathId, activityId }, header: { 'Idempotency-Key': idempotencyKey } },
    }),
    activity: (pathId: string, activityId: string) => authenticatedClient.GET('/v1/paths/{pathId}/activities/{activityId}', {
      params: { path: { pathId, activityId } },
    }),
    activityRevisions: (pathId: string, activityId: string, cursor?: string) => authenticatedClient.GET('/v1/paths/{pathId}/activities/{activityId}/revisions', {
      params: { path: { pathId, activityId }, query: { cursor, limit: 25 } },
    }),
    declineDuplicateEmailRecovery: () => authenticatedClient.POST('/v1/onboarding/duplicate-email-recovery/decline'),
    revoke: () => authenticatedClient.DELETE('/v1/session'),
  };
}
