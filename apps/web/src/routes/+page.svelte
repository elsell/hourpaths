<script lang="ts">
  import { onMount } from 'svelte';
  import { createPathSubmissionOwner, createSessionApiClient, createTimerOperationOwner, formatTimerDuration, generatedResponse, sessionExpiryAdvanced, sessionRefreshDelay, sessionRefreshLeadMs, timerMutationPresentation, type ActivityDeletionResult, type ActivityDetail, type ActivityMutationResult, type ActivityRevision, type GeneratedOperationResult, type ManualActivityDefaults, type MemberRemovalReceipt, type MemberRemovalReview as GeneratedMemberRemovalReview, type OwnershipTransfer, type OwnershipTransferCandidate, type OwnershipTransferResult, type PathArchiveStateDraft, type PathCreateDraft, type PathGoalMutationResult, type PathGoalUpdateDraft, type PathMember, type PathRecurrence, type SessionPath, type TimerState, type TimerStopResult } from '@hourpaths/api-client';
  import { activeTimerSeconds, applyGoalMutationResult, applyNotificationMutation, applyPathArchiveResult, applyPathDeletionResult, applyPathLeaveResult, applyPathMemberRemovalResult, applyPathMemberRoleChangeResult, applyPathRenameResult, applyPathVisibilityResult, authenticatedProfileFromAPI, compareGoalConfigurations, createAsyncMutationBarrier, createManualActivityFormState, createNotificationRefreshLatch, createPathArchiveOperationOwner, createPathDeletionOperationOwner, createPathInvitationAcceptOwner, createPathInvitationCancelOwner, createPathInvitationRecipientReviewOwner, createPathInvitationSendOwner, createPathLeaveOperationOwner, createPathMemberRemovalOperationOwner, createPathMemberRoleChangeOperationOwner, createPathRenameOperationOwner, createPathVisibilityOperationOwner, createProfileSearchOwner, createSessionOperationOwner, createSignOutTimerResolutionCoordinator, effectivePathCapabilities, followRequestPageFromAPI, followRequestReviewResultFromAPI, intervalProgress, isSessionFailure, manualActivityParticipantNow, mergeFollowRequestPage, mergeManagedPendingInvitationPage, mergeNotificationHistoryPage, mergePendingInvitationPage, notificationPresentationMessageKey, overallProgress, overrideManualActivityOccurrence, pathInvitationFailureFromProblem, pathInvitationFailureMessageKey, pathInvitationOutputData, pathsRequiringTimerRestore, pathVisibilityFromAPI, pathVisibilityOptions, profileSearchPageFromAPI, profileSearchQuery, publicProfileFromAPI, relationshipMutationResultFromAPI, removeResolvedFollowRequest, retainedSessionExpiry, reviewPathArchiveChange, reviewPathDeletion, reviewPathLeave, reviewPathMemberRemoval, reviewPathMemberRoleChange, reviewPathRename, reviewPathVisibilityChange, reviewPendingPathInvitationAcceptance, serializeManualActivityForm, sessionFailureFromResponse, sessionRetryDelay, updateManualActivityDuration, validateSessionCredential, type AuthenticatedProfile, type ClientRuntimeConfig, type FollowRequestState, type GoalConfiguration, type GoalConfigurationComparison, type IntervalProgress, type ManagedPendingPathInvitation, type ManagedPendingPathInvitationState, type ManualActivityFormState, type ManualActivityLocalDateTime, type ManualActivityParticipantNow, type NotificationHistoryState, type NotificationMutation, type OverallProgress, type PathArchiveReview, type PathDeletionReview, type PathInvitation, type PathInvitationAcceptanceReview, type PathInvitationFailure, type PathInvitationNotification, type PathInvitationRecipientReview, type PathInvitationRole, type PathLeaveReceipt, type PathLeaveReview, type PathMemberAccessRole, type PathMemberRemovalReview, type PathMemberRoleChangeReceipt, type PathVisibility, type PathVisibilityChangeReview, type PendingPathInvitation, type PendingPathInvitationState, type ProfileSearchState, type PublicProfile, type RunningTimerSnapshot, type SessionAccessState, type SessionFailure, type SessionOperationTicket, type SignOutTimerResolution } from '@hourpaths/client-core';
  import { blockedAccountPageFromAPI, blockReviewFromAPI, blockResultFromAPI, mergeBlockedAccountPage, unblockResultFromAPI, type BlockedAccount, type BlockedAccountPage, type BlockReview } from '@hourpaths/client-core';
  import { createTranslator, type MessageKey, type SupportedLocale, type Translator } from '@hourpaths/i18n';
  import { applicationDestination, applicationSession, applicationSessionOperations, beginApplicationSignIn, clearApplicationSession, refreshApplicationSession, revokeApplicationSession, revokeSupersededApplicationSession, webSessionFailure, type ApplicationSession } from '$lib/auth';
  import { replaceApplicationLocation } from '$lib/provider-auth';
  import { buildGoalDraft, goalFormState, type GoalFormState } from '$lib/path-goals';
  import { activityEditForm, activityValidationNow, groupActivitiesNewestFirst, mergeActivityHistory, mergeRevisionHistory, removeActivity, type ActivityDay } from '$lib/path-activity-history';
  import { openNotificationConvergenceBrowser, type NotificationConvergenceBrowser } from '$lib/notification-convergence-browser';
  import { notificationConvergenceOwner, notificationConvergenceSignal, notificationSections, notificationTarget } from '$lib/notification-page';
  import { focusAccessibleElement } from '$lib/accessibility-focus';
  import { currentPendingOwnershipTransfers, decodeOwnershipTransferReview, mergeOwnershipTransferCandidates, ownershipTransferExpiration, validViewerTimeZone, type OwnershipTransferExpiration, type OwnershipTransferReview } from '$lib/path-ownership-transfer';
  import SocialProfileDiscovery from '$lib/social-profile-discovery.svelte';
  import PathMemberAccess from '$lib/path-member-access.svelte';
  import PathMemberComparison from '$lib/path-member-comparison.svelte';

  const intervalProgressPresentation = intervalProgress;

  type CursorPage<T> = { items: T[]; nextCursor?: string };
  type CursorEnvelope<T> = { data: T[]; meta: { nextCursor?: string } };
  type VisibilitySessionPath = Omit<SessionPath, 'visibility'> & { visibility: PathVisibility };
  type PrimarySurface = 'home' | 'following';
  type PresentedPathMember = PathMember & { canChangeRole: boolean; canGrantAdministrator: boolean; canRevokeAdministrator: boolean; canStepDownAdministrator: boolean; isViewer: boolean };
  type PresentedOwnershipTransfer = OwnershipTransfer & {
    counterpart: { displayName: string; userId: string; username: string };
    counterpartRole: 'creator' | 'recipient';
    viewerTimeZone: string;
  };

  function ownershipTransferFromResult(result: OwnershipTransferResult): PresentedOwnershipTransfer {
    if (!validViewerTimeZone(result.viewerTimeZone)) throw sessionFailureFromResponse(502);
    return {
      ...result.transfer,
      counterpart: result.counterpart,
      counterpartRole: result.counterpartRole,
      viewerTimeZone: result.viewerTimeZone,
    };
  }

  function visibilitySessionPath(path: SessionPath): VisibilitySessionPath {
    return { ...path, visibility: pathVisibilityFromAPI(path.visibility) };
  }

  function pathVisibilityLabel(value: unknown): string {
    return i18n.t(`pathVisibility.option.${pathVisibilityFromAPI(value)}`);
  }

  function cursorGeneratedResponse<T>(result: GeneratedOperationResult<CursorEnvelope<T>>) {
    const response = generatedResponse(result);
    return {
      ...response,
      json: async () => {
        const envelope = await response.json();
        return envelope ? { data: { items: envelope.data, nextCursor: envelope.meta.nextCursor } } : undefined;
      },
    };
  }

  async function invitationResponse<T>(result: GeneratedOperationResult<T>): Promise<T> {
    const response = generatedResponse(result);
    if (!response.ok) throw pathInvitationFailureFromProblem(response.status, response.problem);
    const value = await response.json();
    if (value === undefined) throw { kind: 'invalid_response' } as const;
    return value;
  }

  export let data: { locale: SupportedLocale; config: ClientRuntimeConfig };
  let profile: AuthenticatedProfile | null = null;
  let paths: SessionPath[] | null = null;
  let archivedPaths: SessionPath[] = [];
  let showingArchived = false;
  let primarySurface: PrimarySurface = 'home';
  let socialQuery = '';
  let socialSearch: ProfileSearchState = { query: '', items: [], nextCursor: '' };
  let socialSearchState: 'hint' | 'loading' | 'empty' | 'error' | 'results' = 'hint';
  let socialLoadingMore = false;
  let selectedSocialProfile: PublicProfile | null = null;
  let selectedSocialUsername = '';
  let socialProfileState: 'idle' | 'loading' | 'error' | 'ready' = 'idle';
  let socialRelationshipBusy = false;
  let socialRelationshipError = false;
  let socialFollowRequestsOpen = false;
  let socialFollowRequests: FollowRequestState = { items: [], nextCursor: '' };
  let socialFollowRequestsState: 'idle' | 'loading' | 'empty' | 'error' | 'ready' = 'idle';
  let socialFollowRequestBusyID = '';
  let socialBlockReview: BlockReview | null = null;
  let socialBlockBusy = false;
  let socialBlockError = false;
  let socialBlockedAccountsOpen = false;
  let socialBlockedAccountsState: 'idle' | 'loading' | 'empty' | 'error' | 'ready' = 'idle';
  let socialBlockedAccounts: BlockedAccountPage = { items: [], nextCursor: '' };
  let socialUnblockReview: BlockedAccount | null = null;
  $: presentedSocialBlockedAccounts = socialBlockedAccounts.items.map(({ identity, blockedAt }) => ({ ...identity, blockedAt }));
  $: presentedSocialUnblockReview = socialUnblockReview ? { ...socialUnblockReview.identity, blockedAt: socialUnblockReview.blockedAt } : null;
  const socialProfileSearchOwner = createProfileSearchOwner();
  const socialProfileSearchOperations = createSessionOperationOwner();
  const socialProfileDetailOperations = createSessionOperationOwner();
  const socialFollowRequestOperations = createSessionOperationOwner();
  const socialBlockOperations = createSessionOperationOwner();
  const socialBlockedAccountOperations = createSessionOperationOwner();
  let session: ApplicationSession | null = null;
  let ready = false;
  let errorKey: MessageKey | null = null;
  let accessState: SessionAccessState = 'authentication_required';
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let refreshAttempt = 0;
  let sessionOperationBusy = false;
  let retryOperation: 'refresh' | 'profile' = 'refresh';
  let sessionRenewable = true;
  let creatingPath = false;
  let pathName = '';
  let pathSubmitting = false;
  let pathErrorKey: MessageKey | null = null;
  let pathCreated = false;
  let intervalGoalEnabled = false;
  let intervalHours = '0';
  let intervalMinutes = '0';
  let intervalSeconds = '0';
  let intervalRecurrence: PathRecurrence = 'daily';
  let customAlignment = false;
  let alignmentValue = '';
  let yearlyMonth = '1';
  let yearlyDay = '1';
  let overallTargetEnabled = false;
  let overallHours = '0';
  let overallMinutes = '0';
  let overallSeconds = '0';
  const calendarMonths = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12] as const;
  const isoWeekdays = [1, 2, 3, 4, 5, 6, 7] as const;
  const pathCreation = createPathSubmissionOwner(() => crypto.randomUUID());
  const timerOperations = createTimerOperationOwner(() => crypto.randomUUID());
  const timerMutationBarrier = createAsyncMutationBarrier();
  const signOutTimerResolutions = createSignOutTimerResolutionCoordinator();
  let timerStates: Record<string, TimerState> = {};
  let timerBusy: Record<string, boolean> = {};
  let timerErrorKeys: Record<string, MessageKey | undefined> = {};
  let timerNoticeKey: MessageKey | null = null;
  let timerMutationLocked = false;
  let signOutDialogOpen = false;
  let signOutBusy = false;
  let signOutErrorKey: MessageKey | null = null;
  let signOutRunningTimerCount = 0;
  type SignOutResolutionContext = {
    session: ApplicationSession;
    ownerID: string;
    resolution: SignOutTimerResolution;
    timerIDs: ReadonlyMap<string, string>;
  };
  let signOutResolutionContext: SignOutResolutionContext | null = null;
  let now = Date.now();
  let manualPathID: string | null = null;
  let manualForm: ManualActivityFormState | null = null;
  let manualNote = '';
  let manualBusy = false;
  let manualErrorKey: MessageKey | null = null;
  let manualActivity: { id: string; version: number } | null = null;
  let manualDefaults: ManualActivityDefaults | null = null;
  let manualDefaultsLoadedAt = 0;
  let manualIdempotencyKey = '';
  let manualOwnerID: string | null = null;
  let manualOccurrenceTimeZone: string | null = null;
  const manualOperations = createSessionOperationOwner();
  let selectedPath: SessionPath | null = null;
  let selectedTimerState: TimerState | undefined;
  $: selectedTimerState = selectedPath ? timerStates[selectedPath.id] : undefined;
  let activityDays: ActivityDay[] = [];
  let activityHistoryItems: ActivityDetail[] = [];
  let selectedActivity: ActivityDetail | null = null;
  let activityRevisions: ActivityRevision[] = [];
  let pathDetailBusy = false;
  let pathDetailErrorKey: MessageKey | null = null;
  let detailOwnerID: string | null = null;
  let historyOpen = false;
  let activityNextCursor: string | null = null;
  let activityPageBusy = false;
  let activityPageFailed = false;
  let activityRetryCursor: string | null = null;
  let activityRetryReplace = true;
  let revisionNextCursor: string | null = null;
  let revisionPageBusy = false;
  let revisionPageFailed = false;
  let revisionRetryCursor: string | null = null;
  let revisionRetryReplace = true;
  let revisionRetryActivityID: string | null = null;
  const pathDetailOperations = createSessionOperationOwner();
  let deleteConfirmActivityID: string | null = null;
  let deleteBusy = false;
  let deleteFailed = false;
  let deleteIdempotencyKey = '';
  let deleteOwnerID: string | null = null;
  const deleteOperations = createSessionOperationOwner();
  let managingPathGoals = false;
  let goalForm: GoalFormState | null = null;
  let goalReview: GoalConfigurationComparison | null = null;
  let goalUpdateBusy = false;
  let goalUpdateErrorKey: MessageKey | null = null;
  let goalUpdateSaved = false;
  let goalUpdateIdempotencyKey = '';
  let goalUpdateOwnerID: string | null = null;
  const goalManagementOperations = createSessionOperationOwner();
  const pathVisibilityOperations = createPathVisibilityOperationOwner(() => crypto.randomUUID());
  let pathVisibilityDraft: PathVisibility = 'private';
  let pathVisibilityReview: Extract<PathVisibilityChangeReview, { kind: 'ready' }> | null = null;
  let pathVisibilityBusy = false;
  let pathVisibilityErrorKey: MessageKey | null = null;
  let pathVisibilitySaved = false;
  const archiveOperations = createPathArchiveOperationOwner(() => crypto.randomUUID());
  let archiveReview: PathArchiveReview | null = null;
  let archiveBusy = false;
  let archiveErrorKey: MessageKey | null = null;
  let archiveStatusKey: MessageKey | null = null;
  const pathDeletionOperations = createPathDeletionOperationOwner(() => crypto.randomUUID());
  let pathDeletionReview: PathDeletionReview | null = null;
  let pathDeletionBusy = false;
  let pathDeletionErrorKey: MessageKey | null = null;
  const pathLeaveOperations = createPathLeaveOperationOwner(() => crypto.randomUUID());
  let pathLeaveReview: PathLeaveReview | null = null;
  let pathLeaveBusy = false;
  let pathLeaveErrorKey: MessageKey | null = null;
  let pathLeaveStatus: { pathName: string; retained: boolean } | null = null;
  let pathLeaveReturnFocus: HTMLElement | null = null;
  $: pathLeaveInteractionBlocked = pathLeaveReview !== null || pathLeaveBusy;
  const pathMemberListOperations = createSessionOperationOwner();
  const pathMemberReviewOperations = createSessionOperationOwner();
  const pathMemberActivityOperations = createSessionOperationOwner();
  const pathMemberUnblockOperations = createSessionOperationOwner();
  const pathMemberRemovalOperations = createPathMemberRemovalOperationOwner(() => crypto.randomUUID());
  const pathMemberRoleChangeOperations = createPathMemberRoleChangeOperationOwner(() => crypto.randomUUID());
  let pathMembersOpen = false;
  let pathMembers: PresentedPathMember[] = [];
  let pathMembersNextCursor = '';
  let pathMembersLoading = false;
  let pathMembersLoadingMore = false;
  let pathMembersFailed = false;
  let selectedPathMember: PresentedPathMember | null = null;
  let pathMemberRemovalReview: PathMemberRemovalReview | null = null;
  let pathMemberReviewLoading = false;
  let pathMemberRemovalBusy = false;
  let pathMemberRemovalError = false;
  let pathMemberPendingRole: PathMemberAccessRole | null = null;
  let pathMemberRoleChangeBusy = false;
  let pathMemberRoleChangeError = false;
  let pathMemberUnblockBusy = false;
  let pathMemberUnblockError = false;
  let pathMemberActivities: ActivityDetail[] = [];
  let pathMemberActivitiesNextCursor = '';
  let pathMemberActivitiesLoading = false;
  let pathMemberActivitiesFailed = false;
  const pathRenameOperations = createPathRenameOperationOwner(() => crypto.randomUUID());
  let renamingPath = false;
  let pathRenameDraft = '';
  let pathRenameBusy = false;
  let pathRenameErrorKey: MessageKey | null = null;
  let pathRenameStatusName: string | null = null;
  let pendingInvitations: PendingPathInvitationState = { items: [], nextCursor: '' };
  let pendingInvitationsBusy = false;
  let pendingInvitationsErrorKey: MessageKey | null = null;
  let pendingInvitationBusy: Record<string, boolean | undefined> = {};
  let pendingInvitationErrors: Record<string, MessageKey | undefined> = {};
  let pendingInvitationReview:
    | Extract<PathInvitationAcceptanceReview, { kind: 'confirmation-required' }>
    | null = null;
  let pendingInvitationFocusTarget:
    | { kind: 'accept' | 'confirm'; invitationID: string }
    | null = null;
  let acceptedInvitation: { role: PathInvitationRole } | null = null;
  let sharingPath = false;
  let invitationUsername = '';
  let invitationRole: PathInvitationRole = 'participant';
  let invitationReview: PathInvitationRecipientReview | null = null;
  let invitationReviewBusy = false;
  let invitationSendBusy = false;
  let invitationErrorKey: MessageKey | null = null;
  let invitationSent = false;
  let invitationOwnerID: string | null = null;
  const invitationReviewOwner = createPathInvitationRecipientReviewOwner();
  const invitationSendOwner = createPathInvitationSendOwner(() => crypto.randomUUID());
  const managedInvitationListOperations = createSessionOperationOwner();
  const managedInvitationCancelOwner = createPathInvitationCancelOwner(() => crypto.randomUUID());
  let managedInvitations: ManagedPendingPathInvitationState = { items: [], nextCursor: '' };
  let managedInvitationsLoading = false;
  let managedInvitationsLoadingMore = false;
  let managedInvitationsFailed = false;
  let managedInvitationRetryCursor = '';
  let managedInvitationCancelReview: ManagedPendingPathInvitation | null = null;
  let managedInvitationCancelBusy = false;
  let managedInvitationCancelFailed = false;
  const invitationAcceptOwner = createPathInvitationAcceptOwner(() => crypto.randomUUID());
  const notificationOperations = createSessionOperationOwner();
  const notificationMutationOperations = createSessionOperationOwner();
  let notificationHistory: NotificationHistoryState = { items: [], nextCursor: '', unreadCount: 0 };
  let notificationsOpen = false;
  let notificationsBusy = false;
  let notificationErrorKey: MessageKey | null = null;
  let notificationMutationBusy = false;
  let notificationMutationErrorKey: MessageKey | null = null;
  let notificationRetry: NotificationMutation | null = null;
  let notificationUnreadCount = 0;
  let notificationUnreadCountAuthoritative = false;
  let notificationConvergenceBrowser: NotificationConvergenceBrowser | null = null;
  let notificationRefreshWaitingForLoadOwnerID = '';
  const notificationRefreshLatch = createNotificationRefreshLatch(async (ownerID) => {
    if (!session || profile?.id !== ownerID) return;
    if (notificationsBusy) {
      notificationRefreshWaitingForLoadOwnerID = ownerID;
      return;
    }
    await loadNotifications('', true, ownerID);
  });
  $: notificationGroups = notificationSections(notificationHistory.items);
  let pendingOwnershipTransfers: PresentedOwnershipTransfer[] = [];
  let visiblePendingOwnershipTransfers: PresentedOwnershipTransfer[] = [];
  $: visiblePendingOwnershipTransfers = currentPendingOwnershipTransfers(pendingOwnershipTransfers, now);
  let ownershipTransferFocusTarget: string | null = null;
  let ownershipOpen = false;
  let ownershipCandidates: OwnershipTransferCandidate[] = [];
  let ownershipCandidateNextCursor = '';
  let ownershipCandidatesBusy = false;
  let ownershipErrorKey: MessageKey | null = null;
  let ownershipSelectedCandidate: OwnershipTransferCandidate | null = null;
  let ownershipReview: OwnershipTransferReview | null = null;
  let ownershipReviewing = false;
  let ownershipReviewExpiration: OwnershipTransferExpiration | null = null;
  let ownershipMutationBusy = false;
  let ownershipMutationAction: 'initiate' | 'accept' | 'decline' | 'cancel' | null = null;
  let ownershipMutationTargetID: string | null = null;
  let ownershipMutationKeys: Record<string, string | undefined> = {};
  const ownershipOperations = createSessionOperationOwner();
  let i18n: Translator;
  $: i18n = createTranslator([data.locale]);

  function overallProgressMessage(progress: OverallProgress): string {
    return i18n.t(progress.completed ? 'path.progress.overallComplete' : 'path.progress.overall', {
      accumulated: i18n.number(progress.accumulatedSeconds),
      target: i18n.number(progress.targetSeconds),
    });
  }

  function intervalProgressMessage(progress: IntervalProgress): string {
    return i18n.t(progress.completed ? 'path.progress.intervalComplete' : 'path.progress.interval', {
      accumulated: i18n.number(progress.accumulatedSeconds),
      target: i18n.number(progress.targetSeconds),
    });
  }

  function resetManualActivity() {
    manualOperations.invalidate();
    manualPathID = null;
    manualForm = null;
    manualDefaults = null;
    manualDefaultsLoadedAt = 0;
    manualNote = '';
    manualBusy = false;
    manualErrorKey = null;
    manualActivity = null;
    manualIdempotencyKey = '';
    manualOwnerID = null;
    manualOccurrenceTimeZone = null;
  }

  function resetDeleteActivity() {
    deleteOperations.invalidate();
    deleteConfirmActivityID = null;
    deleteBusy = false;
    deleteFailed = false;
    deleteIdempotencyKey = '';
    deleteOwnerID = null;
  }

  function resetGoalManagement() {
    pathDeletionOperations.cancel(pathDeletionReview?.pathId);
    pathDeletionReview = null;
    pathDeletionBusy = false;
    pathDeletionErrorKey = null;
    goalManagementOperations.invalidate();
    managingPathGoals = false;
    goalForm = null;
    goalReview = null;
    goalUpdateBusy = false;
    goalUpdateErrorKey = null;
    goalUpdateSaved = false;
    goalUpdateIdempotencyKey = '';
    goalUpdateOwnerID = null;
    pathVisibilityOperations.cancel(selectedPath?.id);
    pathVisibilityDraft = 'private';
    pathVisibilityReview = null;
    pathVisibilityBusy = false;
    pathVisibilityErrorKey = null;
    pathVisibilitySaved = false;
  }

  function resetInvitationShare() {
    invitationReviewOwner.cancel();
    invitationSendOwner.cancel(selectedPath?.id);
    managedInvitationListOperations.invalidate();
    managedInvitationCancelOwner.cancel();
    sharingPath = false;
    invitationUsername = '';
    invitationRole = 'participant';
    invitationReview = null;
    invitationReviewBusy = false;
    invitationSendBusy = false;
    invitationErrorKey = null;
    invitationSent = false;
    invitationOwnerID = null;
    managedInvitations = { items: [], nextCursor: '' };
    managedInvitationsLoading = false;
    managedInvitationsLoadingMore = false;
    managedInvitationsFailed = false;
    managedInvitationRetryCursor = '';
    managedInvitationCancelReview = null;
    managedInvitationCancelBusy = false;
    managedInvitationCancelFailed = false;
  }

  function resetInvitations() {
    resetInvitationShare();
    invitationAcceptOwner.cancel();
    pendingInvitations = { items: [], nextCursor: '' };
    pendingInvitationsBusy = false;
    pendingInvitationsErrorKey = null;
    pendingInvitationBusy = {};
    pendingInvitationErrors = {};
    pendingInvitationReview = null;
    acceptedInvitation = null;
    pendingOwnershipTransfers = [];
    resetNotifications();
    resetSocialProfileDiscovery();
  }

  function clearPendingInvitationAcceptance() {
    invitationAcceptOwner.cancel();
    pendingInvitationReview = null;
    pendingInvitationBusy = {};
  }

  function resetNotifications() {
    notificationOperations.invalidate();
    notificationMutationOperations.invalidate();
    notificationHistory = { items: [], nextCursor: '', unreadCount: 0 };
    notificationsOpen = false;
    notificationsBusy = false;
    notificationErrorKey = null;
    notificationMutationBusy = false;
    notificationMutationErrorKey = null;
    notificationRetry = null;
    notificationUnreadCount = 0;
    notificationUnreadCountAuthoritative = false;
    notificationRefreshWaitingForLoadOwnerID = '';
  }

  function resetSocialProfileDiscovery() {
    socialProfileSearchOwner.cancel();
    socialProfileSearchOperations.invalidate();
    socialProfileDetailOperations.invalidate();
    socialFollowRequestOperations.invalidate();
    socialBlockOperations.invalidate();
    socialBlockedAccountOperations.invalidate();
    primarySurface = 'home';
    socialQuery = '';
    socialSearch = { query: '', items: [], nextCursor: '' };
    socialSearchState = 'hint';
    socialLoadingMore = false;
    selectedSocialProfile = null;
    selectedSocialUsername = '';
    socialProfileState = 'idle';
    socialRelationshipBusy = false;
    socialRelationshipError = false;
    socialFollowRequestsOpen = false;
    socialFollowRequests = { items: [], nextCursor: '' };
    socialFollowRequestsState = 'idle';
    socialFollowRequestBusyID = '';
    socialBlockReview = null;
    socialBlockBusy = false;
    socialBlockError = false;
    socialBlockedAccountsOpen = false;
    socialBlockedAccountsState = 'idle';
    socialBlockedAccounts = { items: [], nextCursor: '' };
    socialUnblockReview = null;
  }

  function resetOwnershipTransfer() {
    ownershipOperations.invalidate();
    ownershipOpen = false;
    ownershipCandidates = [];
    ownershipCandidateNextCursor = '';
    ownershipCandidatesBusy = false;
    ownershipErrorKey = null;
    ownershipSelectedCandidate = null;
    ownershipReview = null;
    ownershipReviewing = false;
    ownershipReviewExpiration = null;
    ownershipMutationBusy = false;
    ownershipMutationAction = null;
    ownershipMutationTargetID = null;
    ownershipMutationKeys = {};
  }

  function focusPendingOwnershipTransfer(node: HTMLElement, transferID: string) {
    if (ownershipTransferFocusTarget !== transferID) return;
    focusAccessibleElement(node);
    ownershipTransferFocusTarget = null;
  }

  function ownershipTransferViewerRole(transfer: PresentedOwnershipTransfer): 'creator' | 'recipient' {
    if (transfer.counterpartRole) return transfer.counterpartRole === 'recipient' ? 'creator' : 'recipient';
    return transfer.creatorUserId === profile?.id ? 'creator' : 'recipient';
  }

  function resetPathRename() {
    pathRenameOperations.cancel();
    renamingPath = false;
    pathRenameDraft = '';
    pathRenameBusy = false;
    pathRenameErrorKey = null;
    pathRenameStatusName = null;
  }

  function closePathMemberReview() {
    pathMemberReviewOperations.invalidate();
    pathMemberActivityOperations.invalidate();
    pathMemberUnblockOperations.invalidate();
    if (!pathMemberRemovalBusy && !pathMemberRoleChangeBusy && selectedPath && selectedPathMember) {
      pathMemberRemovalOperations.cancel(selectedPath.id, selectedPathMember.userId);
      pathMemberRoleChangeOperations.cancel(selectedPath.id, selectedPathMember.userId);
    }
    selectedPathMember = null;
    pathMemberRemovalReview = null;
    pathMemberReviewLoading = false;
    pathMemberRemovalBusy = false;
    pathMemberRemovalError = false;
    pathMemberPendingRole = null;
    pathMemberRoleChangeBusy = false;
    pathMemberRoleChangeError = false;
    pathMemberUnblockBusy = false;
    pathMemberUnblockError = false;
    pathMemberActivities = [];
    pathMemberActivitiesNextCursor = '';
    pathMemberActivitiesLoading = false;
    pathMemberActivitiesFailed = false;
  }

  function closePathMembers(preserveComparison = false) {
    closePathMemberReview();
    pathMembersOpen = false;
    if (preserveComparison) return;
    pathMemberListOperations.invalidate();
    pathMembers = [];
    pathMembersNextCursor = '';
    pathMembersLoading = false;
    pathMembersLoadingMore = false;
    pathMembersFailed = false;
  }

  function resetPathDetails() {
    closePathMembers();
    pathDetailOperations.invalidate();
    resetDeleteActivity();
    resetGoalManagement();
    resetInvitationShare();
    resetOwnershipTransfer();
    resetPathRename();
    pathLeaveOperations.cancel(pathLeaveReview?.pathId);
    pathLeaveReview = null;
    pathLeaveBusy = false;
    pathLeaveErrorKey = null;
    if (archiveReview) archiveOperations.cancel(archiveReview.pathId);
    archiveReview = null;
    archiveBusy = false;
    archiveErrorKey = null;
    selectedPath = null;
    activityDays = [];
    activityHistoryItems = [];
    selectedActivity = null;
    activityRevisions = [];
    pathDetailBusy = false;
    pathDetailErrorKey = null;
    detailOwnerID = null;
    historyOpen = false;
    activityNextCursor = null;
    activityPageBusy = false;
    activityPageFailed = false;
    activityRetryCursor = null;
    activityRetryReplace = true;
    revisionNextCursor = null;
    revisionPageBusy = false;
    revisionPageFailed = false;
    revisionRetryCursor = null;
    revisionRetryReplace = true;
    revisionRetryActivityID = null;
  }

  function scheduleExpiration(expiresAt: string) {
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      applicationSessionOperations.invalidate();
      clearApplicationSession();
      session = null;
      profile = null;
      paths = null;
      archivedPaths = [];
      creatingPath = false;
      pathSubmitting = false;
      pathCreation.cancel();
      timerOperations.cancel();
      timerStates = {};
      resetManualActivity();
      resetPathDetails();
      resetInvitations();
      errorKey = 'errors.sessionExpired';
    }, Math.max(0, Date.parse(expiresAt) - Date.now()));
  }

  function scheduleRetry(expiresAt: string, operation: 'refresh' | 'profile') {
    retryOperation = operation;
    const delay = sessionRetryDelay(refreshAttempt, expiresAt);
    refreshAttempt += 1;
    if (delay <= 0) { scheduleExpiration(expiresAt); return; }
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      if (retryOperation === 'profile' && session) void attemptProfile(session, expiresAt);
      else void attemptRefresh(expiresAt);
    }, delay);
  }

  async function loadProfile(session: ApplicationSession) {
    const [nextProfileData, nextPaths, nextArchived, nextPendingInvitations] = await Promise.all([
      validateSessionCredential<unknown>(session, async (current) =>
        generatedResponse(await createSessionApiClient(data.config.apiURL, () => current.token).profile()),
      ),
      validateSessionCredential<SessionPath[]>(session, async (current) =>
        generatedResponse(await createSessionApiClient(data.config.apiURL, () => current.token).paths()),
      ),
      validateSessionCredential<CursorPage<SessionPath>>(session, async (current) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).archivedPaths(),
      )),
      validateSessionCredential<CursorPage<PendingPathInvitation>>(session, async (current) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).pendingPathInvitations(),
      )),
    ]);
    const nextProfile = authenticatedProfileFromAPI(nextProfileData);
    const trackablePaths = pathsRequiringTimerRestore(nextPaths);
    const [nextTimers, ownershipResults] = await Promise.all([
      Promise.all(trackablePaths.map((path) =>
      validateSessionCredential<TimerState>(session, async (current) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).currentTimer(path.id),
      )),
      )),
      Promise.all([...nextPaths, ...nextArchived.items].map(async (path) => {
        const result = await createSessionApiClient(data.config.apiURL, () => session.token).pendingOwnershipTransfer(path.id);
        const response = generatedResponse(result);
        if (response.status === 404) return null;
        if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
        const envelope = await response.json();
        if (!envelope?.data) throw sessionFailureFromResponse(502);
        return ownershipTransferFromResult(envelope.data);
      })),
    ]);
    return {
      profile: nextProfile,
      paths: nextPaths,
      archivedPaths: nextArchived.items,
      pendingInvitations: nextPendingInvitations,
      pendingOwnershipTransfers: currentPendingOwnershipTransfers(
        ownershipResults.flatMap((transfer) => transfer ? [transfer] : []),
      ),
      timers: Object.fromEntries(trackablePaths.map((path, index) => [path.id, nextTimers[index]!])),
    };
  }

  function handleFailure(
    cause: unknown,
    current: ApplicationSession | null,
    expiresAt: string,
    operation: 'refresh' | 'profile',
    ticket: SessionOperationTicket,
  ) {
    if (!ticket.current()) return;
    const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
    const presentation = webSessionFailure(failure);
    if (presentation.discardCredential) {
      applicationSessionOperations.invalidate();
      clearApplicationSession(); session = null; profile = null; paths = null; archivedPaths = []; creatingPath = false;
      pathCreation.cancel();
      timerOperations.cancel();
      timerStates = {};
      resetManualActivity();
      resetPathDetails();
      resetInvitations();
      resetSocialProfileDiscovery();
      pathSubmitting = false;
    }
    else {
      const retainedExpiry = retainedSessionExpiry(current, expiresAt);
      accessState = presentation.accessState;
      if (presentation.retryable) scheduleRetry(retainedExpiry, operation);
      else scheduleExpiration(retainedExpiry);
    }
    errorKey = presentation.message;
  }

  function scheduleOwnedSession() {
    if (!session) return;
    if (sessionRenewable) scheduleRefresh(session.expiresAt);
    else scheduleExpiration(session.expiresAt);
  }

  async function attemptProfile(
    current: ApplicationSession,
    expiresAt: string,
    ticket = applicationSessionOperations.issue(),
    nested = false,
  ) {
    if (goalUpdateBusy || (sessionOperationBusy && !nested)) return;
    if (!nested) sessionOperationBusy = true;
    try {
      const home = await loadProfile(current);
      if (!ticket.current() || goalUpdateBusy) return;
      if (profile && profile.id !== home.profile.id) { resetManualActivity(); resetPathDetails(); resetInvitations(); }
      profile = home.profile;
      paths = home.paths;
      archivedPaths = home.archivedPaths;
      pendingInvitations = mergePendingInvitationPage(
        { items: [], nextCursor: '' },
        { items: home.pendingInvitations.items, nextCursor: home.pendingInvitations.nextCursor ?? '' },
        '',
      );
      pendingOwnershipTransfers = home.pendingOwnershipTransfers;
      if (pendingInvitationReview && !pendingInvitations.items.some(
        ({ invitation, warning }) =>
          invitation.id === pendingInvitationReview?.invitationId &&
          warning?.pathVisibility === pendingInvitationReview.warning.pathVisibility &&
          warning?.hasRetainedActivity === pendingInvitationReview.warning.hasRetainedActivity,
      )) {
        invitationAcceptOwner.cancel(pendingInvitationReview.invitationId);
        pendingInvitationReview = null;
      }
      timerStates = home.timers;
      accessState = 'authenticated_online';
      refreshAttempt = 0;
      errorKey = null;
      scheduleOwnedSession();
    } catch (cause) {
      handleFailure(cause, current, expiresAt, 'profile', ticket);
    } finally {
      if (!nested) sessionOperationBusy = false;
    }
  }

  async function attemptRefresh(expiresAt: string) {
    if (!session || goalUpdateBusy || sessionOperationBusy) return;
    sessionOperationBusy = true;
    const current = session;
    const ticket = applicationSessionOperations.issue();
    try {
      let next: ApplicationSession;
      try { next = await refreshApplicationSession(data.config, current, ticket); }
      catch (cause) { handleFailure(cause, current, expiresAt, 'refresh', ticket); return; }
      if (!ticket.current()) {
        revokeSupersededApplicationSession(data.config, next);
        return;
      }
      clearPendingInvitationAcceptance();
      session = next;
      sessionRenewable = sessionExpiryAdvanced(expiresAt, next.expiresAt);
      await attemptProfile(next, next.expiresAt, ticket, true);
    } finally {
      sessionOperationBusy = false;
    }
  }

  function scheduleRefresh(expiresAt: string) {
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => void attemptRefresh(expiresAt), sessionRefreshDelay(expiresAt));
  }

  onMount(async () => {
    try {
      session = applicationSession();
      if (session) {
        if (session.nextAction && session.nextAction !== 'home') {
          replaceApplicationLocation(applicationDestination(session.nextAction));
          return;
        }
        if (Date.parse(session.expiresAt) - Date.now() < sessionRefreshLeadMs) {
          await attemptRefresh(session.expiresAt);
        } else {
          sessionRenewable = true;
          await attemptProfile(session, session.expiresAt);
        }
      }
    } catch (cause) {
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      accessState = presentation.accessState;
      if (presentation.discardCredential) { applicationSessionOperations.invalidate(); timerOperations.cancel(); resetManualActivity(); resetPathDetails(); resetInvitations(); clearApplicationSession(); session = null; timerStates = {}; }
      else {
        if (session) presentation.retryable ? scheduleRetry(session.expiresAt, 'profile') : scheduleExpiration(session.expiresAt);
      }
      errorKey = presentation.message;
    } finally { ready = true; }
  });

  onMount(() => {
    let active = true;
    let liveElapsed: ReturnType<typeof setTimeout>;
    const tick = () => {
      liveElapsed = setTimeout(() => {
        if (!active) return;
        now = Date.now();
        tick();
      }, 1000);
    };
    tick();
    return () => { active = false; clearTimeout(liveElapsed); };
  });

  function requestCurrentNotificationRefresh() {
    if (profile) void notificationRefreshLatch.request(profile.id);
  }

  onMount(() => {
    const handleNotificationSignal = (message: unknown) => {
      const ownerID = notificationConvergenceOwner(message, profile?.id ?? '');
      if (ownerID) void notificationRefreshLatch.request(ownerID);
    };
    notificationConvergenceBrowser = openNotificationConvergenceBrowser(
      handleNotificationSignal,
      requestCurrentNotificationRefresh,
    );
    return () => {
      notificationConvergenceBrowser?.close();
      notificationConvergenceBrowser = null;
      notificationRefreshLatch.dispose();
    };
  });

  async function signIn() {
    errorKey = null;
    try { await beginApplicationSignIn(data.config); }
    catch { errorKey = 'errors.signInFailed'; }
  }

  async function signOut() {
    if (refreshTimer) clearTimeout(refreshTimer);
    applicationSessionOperations.invalidate();
    resetManualActivity();
    resetPathDetails();
    resetInvitations();
    const revocation = revokeApplicationSession(data.config, session);
    session = null;
    accessState = 'authentication_required';
    profile = null;
    paths = null;
    archivedPaths = [];
    creatingPath = false;
    pathCreation.cancel();
    timerOperations.cancel();
    timerMutationLocked = false;
    timerMutationBarrier.unblock();
    signOutTimerResolutions.invalidate();
    pathSubmitting = false;
    pathName = '';
    pathErrorKey = null;
    pathCreated = false;
    resetGoalFields();
    timerStates = {};
    timerBusy = {};
    timerErrorKeys = {};
    timerNoticeKey = null;
    await revocation;
  }

  function ownsSignOutResolution(context: Pick<SignOutResolutionContext, 'session' | 'ownerID'>): boolean {
    return session === context.session && profile?.id === context.ownerID;
  }

  function presentSignOutDialog() {
    signOutDialogOpen = true;
  }

  async function openSignOutDialog() {
    if (!session || timerMutationLocked) return;
    const currentSession = session;
    const ownerID = profile?.id ?? '';
    timerMutationLocked = true;
    await timerMutationBarrier.blockAndDrain();
    if (session !== currentSession || profile?.id !== ownerID) {
      timerMutationLocked = false;
      timerMutationBarrier.unblock();
      return;
    }
    signOutTimerResolutions.invalidate();
    signOutResolutionContext = null;
    signOutErrorKey = null;
    signOutRunningTimerCount = 0;
    const runningEntries = Object.entries(timerStates).filter(([, state]) => state.running);
    if (runningEntries.length === 0) {
      presentSignOutDialog();
      return;
    }

    const snapshots: RunningTimerSnapshot[] = [];
    const timerIDs = new Map<string, string>();
    for (const [pathID, state] of runningEntries) {
      if (!state.timer || state.timer.pathId !== pathID || !Number.isFinite(Date.parse(state.timer.startedAt))) {
        signOutErrorKey = 'settings.account.activeTimers.stopFailed';
        timerMutationLocked = false;
        timerMutationBarrier.unblock();
        return;
      }
      snapshots.push({
        accumulatedSeconds: state.accumulatedSeconds,
        pathId: pathID,
        startedAt: state.timer.startedAt,
      });
      timerIDs.set(pathID, state.timer.id);
    }
    signOutRunningTimerCount = snapshots.length;
    signOutResolutionContext = {
      ownerID,
      resolution: signOutTimerResolutions.begin(snapshots),
      session: currentSession,
      timerIDs,
    };
    presentSignOutDialog();
  }

  function cancelSignOut() {
    signOutResolutionContext?.resolution.cancel();
    signOutTimerResolutions.invalidate();
    signOutResolutionContext = null;
    signOutBusy = false;
    signOutErrorKey = null;
    signOutDialogOpen = false;
    timerMutationLocked = false;
    timerMutationBarrier.unblock();
  }

  async function completeOwnedSignOut(context: Pick<SignOutResolutionContext, 'session' | 'ownerID'>) {
    if (!ownsSignOutResolution(context)) {
      signOutTimerResolutions.invalidate();
      return;
    }
    signOutResolutionContext = null;
    signOutDialogOpen = false;
    await signOut();
  }

  async function completeOrdinarySignOut() {
    if (!session) return;
    const runningEntries = Object.entries(timerStates).filter(([, state]) => state.running);
    if (runningEntries.length > 0) {
      signOutDialogOpen = false;
      openSignOutDialog();
      return;
    }
    const context = { ownerID: profile?.id ?? '', session };
    signOutBusy = true;
    try {
      await completeOwnedSignOut(context);
    } finally {
      signOutBusy = false;
    }
  }

  async function keepTimersRunningAndSignOut() {
    const context = signOutResolutionContext;
    if (!context) return;
    const decision = context.resolution.keepRunning();
    if (!decision.authorizeSignOut || !ownsSignOutResolution(context)) {
      signOutTimerResolutions.invalidate();
      return;
    }
    signOutBusy = true;
    try {
      await completeOwnedSignOut(context);
    } finally {
      signOutBusy = false;
    }
  }

  async function stopTimersAndSignOut() {
    const context = signOutResolutionContext;
    if (!context || signOutBusy) return;
    signOutBusy = true;
    signOutErrorKey = null;
    try {
      const decision = await context.resolution.stopAndSave(async (timer) => {
        if (!ownsSignOutResolution(context)) {
          signOutTimerResolutions.invalidate();
          throw new Error('sign_out_resolution_superseded');
        }
        const timerID = context.timerIDs.get(timer.pathId);
        if (!timerID) throw new Error('sign_out_timer_missing');
        const result = await timerOperations.stop(timer.pathId, timerID, (idempotencyKey) =>
          validateSessionCredential<TimerStopResult>(context.session, async (credential) => generatedResponse(
            await createSessionApiClient(data.config.apiURL, () => credential.token).stopTimer(
              timer.pathId,
              timerID,
              idempotencyKey,
            ),
          )),
        );
        if (result.kind === 'failed') throw result.cause;
        if (result.kind === 'superseded' || !ownsSignOutResolution(context)) {
          signOutTimerResolutions.invalidate();
          throw new Error('sign_out_resolution_superseded');
        }
        const presentation = timerMutationPresentation(result.state);
        timerStates = { ...timerStates, [timer.pathId]: presentation.state };
        return { pathId: timer.pathId, running: presentation.state.running };
      });
      if (!decision.authorizeSignOut) {
        if (decision.kind === 'stop_failed') signOutErrorKey = 'settings.account.activeTimers.stopFailed';
        return;
      }
      await completeOwnedSignOut(context);
    } finally {
      signOutBusy = false;
    }
  }

  function openPathCreation() {
    creatingPath = true;
    pathErrorKey = null;
    pathCreated = false;
  }

  function resetGoalFields() {
    intervalGoalEnabled = false;
    intervalHours = '0';
    intervalMinutes = '0';
    intervalSeconds = '0';
    intervalRecurrence = 'daily';
    customAlignment = false;
    alignmentValue = '';
    yearlyMonth = '1';
    yearlyDay = '1';
    overallTargetEnabled = false;
    overallHours = '0';
    overallMinutes = '0';
    overallSeconds = '0';
  }

  function cancelPathCreation() {
    pathCreation.cancel();
    creatingPath = false;
    pathSubmitting = false;
    pathName = '';
    pathErrorKey = null;
    resetGoalFields();
  }

  async function submitPath() {
    if (!session || !paths || pathSubmitting) return;
    pathErrorKey = null;
    pathCreated = false;
    const goalDraft = buildGoalDraft({
      intervalEnabled: intervalGoalEnabled,
      intervalDuration: { hours: intervalHours, minutes: intervalMinutes, seconds: intervalSeconds },
      recurrence: intervalRecurrence,
      customAlignment,
      alignmentValue,
      yearlyMonth,
      yearlyDay,
      overallEnabled: overallTargetEnabled,
      overallDuration: { hours: overallHours, minutes: overallMinutes, seconds: overallSeconds },
    });
    if (!goalDraft.ok) {
      pathErrorKey = goalDraft.reason === 'duration' ? 'pathCreate.durationInvalid' : 'pathCreate.alignmentInvalid';
      return;
    }
    pathSubmitting = true;
    const current = session;
    const draft: PathCreateDraft = { name: pathName, ...goalDraft.goals };
    const result = await pathCreation.submit(draft, (body, idempotencyKey) =>
      validateSessionCredential<SessionPath>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).createPath(body, idempotencyKey),
      )),
    );
    if (result.kind === 'superseded') return;
    pathSubmitting = false;
    if (result.kind === 'invalid_name') { pathErrorKey = 'pathCreate.nameRequired'; return; }
    if (result.kind === 'invalid_goal') { pathErrorKey = 'pathCreate.durationInvalid'; return; }
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      if (presentation.discardCredential) {
        pathCreation.cancel();
        timerOperations.cancel();
        applicationSessionOperations.invalidate();
        resetManualActivity(); resetPathDetails(); clearApplicationSession(); session = null; profile = null; paths = null; archivedPaths = []; timerStates = {}; creatingPath = false;
      }
      return;
    }
    paths = [...paths, result.path];
    timerStates = {
      ...timerStates,
      [result.path.id]: {
        running: false,
        accumulatedSeconds: 0,
        intervalProgress: result.path.intervalGoal
          ? { accumulatedSeconds: 0, targetSeconds: result.path.intervalGoal.targetSeconds }
          : undefined,
      },
    };
    pathName = '';
    resetGoalFields();
    creatingPath = false;
    pathCreated = true;
  }

  function recurrenceLabel(recurrence: PathRecurrence): string {
    return i18n.t(`path.goal.recurrence.${recurrence}` as MessageKey);
  }

  function weekdayName(isoWeekday: number): string {
    return i18n.date(Date.UTC(2024, 0, isoWeekday), { weekday: 'long', timeZone: 'UTC' });
  }

  function monthName(month: number): string {
    return i18n.date(Date.UTC(2024, month - 1, 1), { month: 'long', timeZone: 'UTC' });
  }

  function intervalGoalSummary(goal: NonNullable<GoalConfiguration['intervalGoal']>): string {
    const base = i18n.t('path.goal.intervalSummary', {
      seconds: i18n.number(goal.targetSeconds),
      recurrence: recurrenceLabel(goal.recurrence),
    });
    const alignment = goal.alignment;
    if (!alignment) return base;
    const boundary = goal.recurrence === 'hourly' ? i18n.number(alignment.minute ?? 0)
      : goal.recurrence === 'daily' ? i18n.number(alignment.hour ?? 0)
        : goal.recurrence === 'weekly' ? weekdayName(alignment.isoWeekday ?? 1)
          : goal.recurrence === 'monthly' ? i18n.number(alignment.day ?? 1)
            : `${monthName(alignment.month ?? 1)} ${i18n.number(alignment.day ?? 1)}`;
    return `${base}; ${i18n.t('pathCreate.alignment')}: ${boundary}`;
  }

  function activityDayDate(localDate: string): Date {
    const [year, month, day] = localDate.split('-').map(Number);
    return new Date(year!, month! - 1, day!);
  }

  function alignmentInputLabel(recurrence: Exclude<PathRecurrence, 'weekly' | 'yearly'>): string {
    const keys = {
      hourly: 'pathCreate.alignment.minute',
      daily: 'pathCreate.alignment.hour',
      monthly: 'pathCreate.alignment.day',
    } as const satisfies Record<typeof recurrence, MessageKey>;
    return i18n.t(keys[recurrence]);
  }

  async function toggleTimer(pathID: string) {
    if (!session || timerBusy[pathID]) return;
    const mutationLease = timerMutationBarrier.enter();
    if (!mutationLease) return;
    try {
    const path = paths?.find((candidate) => candidate.id === pathID);
    if (!path || !effectivePathCapabilities(path).trackTime) return;
    const state = timerStates[pathID];
    if (!state) return;
    const timerID = state.timer?.id;
    timerBusy = { ...timerBusy, [pathID]: true };
    timerErrorKeys = { ...timerErrorKeys, [pathID]: undefined };
    timerNoticeKey = null;
    const current = session;
    const result = state.running && timerID
      ? await timerOperations.stop(pathID, timerID, (idempotencyKey) =>
          validateSessionCredential<TimerStopResult>(current, async (credential) => generatedResponse(
            await createSessionApiClient(data.config.apiURL, () => credential.token).stopTimer(pathID, timerID, idempotencyKey),
          )),
        )
      : await timerOperations.start(pathID, (idempotencyKey) =>
          validateSessionCredential<TimerState>(current, async (credential) => generatedResponse(
            await createSessionApiClient(data.config.apiURL, () => credential.token).startTimer(pathID, idempotencyKey),
          )),
        );
    if (result.kind === 'superseded') return;
    timerBusy = { ...timerBusy, [pathID]: false };
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      timerErrorKeys = { ...timerErrorKeys, [pathID]: presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message };
      if (presentation.discardCredential) {
        timerOperations.cancel();
        applicationSessionOperations.invalidate();
        resetManualActivity(); resetPathDetails(); clearApplicationSession(); session = null; profile = null; paths = null; archivedPaths = []; timerStates = {}; creatingPath = false;
      }
      return;
    }
    const presentation = timerMutationPresentation(result.state);
    timerStates = { ...timerStates, [pathID]: presentation.state };
    if (presentation.notice === 'subsecond') timerNoticeKey = 'timer.subsecondNotice';
    } finally {
      mutationLease.release();
    }
  }

  function manualLocalNow(): ManualActivityParticipantNow {
    if (!manualDefaults) throw new Error('manual defaults unavailable');
    return activityValidationNow(manualDefaults.currentInstant, manualDefaults.timeZone, Math.max(0, Date.now() - manualDefaultsLoadedAt), manualOccurrenceTimeZone ?? undefined);
  }

  function handleManualFailure(cause: unknown) {
    const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
    const presentation = webSessionFailure(failure);
    if (!presentation.discardCredential) {
      manualErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      return;
    }
    applicationSessionOperations.invalidate();
    pathCreation.cancel();
    timerOperations.cancel();
    resetManualActivity();
    resetPathDetails();
    resetInvitations();
    clearApplicationSession();
    session = null;
    profile = null;
    paths = null;
    archivedPaths = [];
    creatingPath = false;
    pathSubmitting = false;
    timerStates = {};
    accessState = presentation.accessState;
    errorKey = presentation.message;
  }

  function handlePathDetailFailure(cause: unknown) {
    const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
    const presentation = webSessionFailure(failure);
    if (!presentation.discardCredential) {
      pathDetailErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      return;
    }
    applicationSessionOperations.invalidate();
    pathCreation.cancel();
    timerOperations.cancel();
    resetManualActivity();
    resetPathDetails();
    resetInvitations();
    clearApplicationSession();
    session = null;
    profile = null;
    paths = null;
    archivedPaths = [];
    creatingPath = false;
    pathSubmitting = false;
    timerStates = {};
    accessState = presentation.accessState;
    errorKey = presentation.message;
  }

  function openPathDetails(path: SessionPath) {
    if (!session || !profile) return;
    resetManualActivity();
    resetPathDetails();
    pathLeaveStatus = null;
    selectedPath = path;
    void openPathMembers('', true, false);
  }

  function invitationFailureKey(cause: unknown): MessageKey {
    if (!cause || typeof cause !== 'object' || typeof (cause as { kind?: unknown }).kind !== 'string') {
      return 'pathInvitation.retry';
    }
    return pathInvitationFailureMessageKey(cause as PathInvitationFailure);
  }

  async function loadNotifications(cursor = '', replace = false, expectedOwnerID = '') {
    if (!session || !profile || notificationsBusy) return;
    if (expectedOwnerID && profile.id !== expectedOwnerID) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = notificationOperations.issue();
    notificationsBusy = true;
    notificationErrorKey = null;
    try {
      const result = await createSessionApiClient(data.config.apiURL, () => current.token)
        .notifications(cursor || undefined);
      const response = generatedResponse(result);
      if (!response.ok) throw new Error('notification_history_unavailable');
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_notification_history');
      const page = {
        items: envelope.data as PathInvitationNotification[],
        nextCursor: envelope.meta.nextCursor ?? '',
        unreadCount: envelope.meta.unreadCount,
      };
      if (session !== current || profile?.id !== ownerID || !ticket.current()) return;
      notificationHistory = mergeNotificationHistoryPage(
        replace ? { items: [], nextCursor: '', unreadCount: notificationHistory.unreadCount } : notificationHistory,
        page,
        cursor,
      );
      notificationUnreadCount = notificationHistory.unreadCount;
      notificationUnreadCountAuthoritative = true;
    } catch {
      if (session === current && profile?.id === ownerID && ticket.current()) {
        notificationErrorKey = 'notification.error';
      }
    } finally {
      if (session === current && profile?.id === ownerID && ticket.current()) {
        notificationsBusy = false;
        const waitingOwnerID = notificationRefreshWaitingForLoadOwnerID;
        notificationRefreshWaitingForLoadOwnerID = '';
        if (waitingOwnerID && ownerID === waitingOwnerID) {
          void notificationRefreshLatch.request(waitingOwnerID);
        }
      }
    }
  }

  function toggleNotifications() {
    if (!notificationsOpen) {
      notificationsOpen = true;
      if (profile) void notificationRefreshLatch.request(profile.id);
      return;
    }
    notificationOperations.invalidate();
    notificationsOpen = false;
    notificationsBusy = false;
    notificationErrorKey = null;
  }

  function retryNotificationHistory() {
    requestCurrentNotificationRefresh();
  }

  async function mutateNotification(mutation: NotificationMutation): Promise<boolean> {
    if (!session || !profile || notificationMutationBusy) return false;
    const current = session;
    const ownerID = profile.id;
    const ticket = notificationMutationOperations.issue();
    notificationMutationBusy = true;
    notificationMutationErrorKey = null;
    notificationRetry = null;
    try {
      const api = createSessionApiClient(data.config.apiURL, () => current.token);
      const result = mutation.kind === 'read'
        ? await api.markNotificationRead(mutation.notificationId)
        : mutation.kind === 'delete'
          ? await api.deleteNotification(mutation.notificationId)
          : await api.markAllNotificationsRead();
      const response = generatedResponse(result);
      if (!response.ok) throw new Error('notification_mutation_unavailable');
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_notification_mutation');
      const applied = applyNotificationMutation(
        notificationHistory,
        mutation,
        envelope.data,
      );
      if (session !== current || profile?.id !== ownerID || !ticket.current()) return false;
      notificationHistory = applied.history;
      notificationUnreadCount = applied.unreadCount;
      notificationUnreadCountAuthoritative = true;
      notificationConvergenceBrowser?.publish(notificationConvergenceSignal(ownerID));
      void notificationRefreshLatch.request(ownerID);
      return true;
    } catch {
      if (session === current && profile?.id === ownerID && ticket.current()) {
        notificationMutationErrorKey = 'notification.mutationError';
        notificationRetry = mutation;
      }
      return false;
    } finally {
      if (session === current && profile?.id === ownerID && ticket.current()) {
        notificationMutationBusy = false;
      }
    }
  }

  async function openNotification(notification: PathInvitationNotification) {
    if (!session || !profile) return;
    notificationOperations.invalidate();
    notificationsBusy = false;
    if (!notification.read && !await mutateNotification({ kind: 'read', notificationId: notification.id })) return;
    const accessiblePaths = [...(paths ?? []), ...archivedPaths];
    const target = notificationTarget(notification, pendingInvitations.items, visiblePendingOwnershipTransfers, accessiblePaths);
    if (!target) return;

    notificationsOpen = false;
    notificationsBusy = false;
    if (target.kind === 'invitation') {
      return;
    }
    if (target.kind === 'follow-request') {
      primarySurface = 'following';
      await loadSocialFollowRequests();
      return;
    }
    if (target.kind === 'profile') {
      primarySurface = 'following';
      await openSocialProfileByUsername(target.username);
      return;
    }
    if (target.kind === 'ownership-transfer') {
      resetPathDetails();
      ownershipTransferFocusTarget = target.transferId;
      return;
    }

    const path = accessiblePaths.find((candidate) => candidate.id === target.pathId);
    if (!path) return;
    openPathDetails(path);
  }

  async function deleteNotification(notification: PathInvitationNotification) {
    await mutateNotification({ kind: 'delete', notificationId: notification.id });
  }

  async function markAllNotificationsRead() {
    await mutateNotification({ kind: 'read-all' });
  }

  async function retryNotificationMutation() {
    const mutation = notificationRetry;
    if (!mutation) return;
    if (mutation.kind === 'read') {
      const notification = notificationHistory.items.find(({ id }) => id === mutation.notificationId);
      if (notification) await openNotification(notification);
      return;
    }
    await mutateNotification(mutation);
  }

  function pendingInvitationAcceptanceInProgress(): boolean {
    return Object.values(pendingInvitationBusy).some(Boolean);
  }

  async function loadPendingInvitations(cursor = '', acceptanceRefresh = false) {
    if (
      !session ||
      !profile ||
      pendingInvitationsBusy ||
      (!acceptanceRefresh && pendingInvitationAcceptanceInProgress())
    ) return;
    const current = session;
    const ownerID = profile.id;
    pendingInvitationsBusy = true;
    pendingInvitationsErrorKey = null;
    try {
      const result = await createSessionApiClient(data.config.apiURL, () => current.token)
        .pendingPathInvitations(cursor || undefined);
      const response = generatedResponse(result);
      if (!response.ok) throw pathInvitationFailureFromProblem(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw { kind: 'invalid_response' } as const;
      const page = { items: envelope.data as PendingPathInvitation[], nextCursor: envelope.meta.nextCursor ?? '' };
      if (session !== current || profile?.id !== ownerID) return;
      pendingInvitations = mergePendingInvitationPage(pendingInvitations, page, cursor);
    } catch (cause) {
      if (session === current && profile?.id === ownerID) pendingInvitationsErrorKey = invitationFailureKey(cause);
    } finally {
      if (session === current && profile?.id === ownerID) pendingInvitationsBusy = false;
    }
  }

  async function loadOwnershipCandidates(cursor = '') {
    if (!session || !profile || !selectedPath || ownershipCandidatesBusy || ownershipMutationBusy) return;
    const path = paths?.find(({ id }) => id === selectedPath?.id);
    if (!path || !effectivePathCapabilities(path).transferOwnership) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = path.id;
    const ticket = ownershipOperations.issue();
    ownershipCandidatesBusy = true;
    ownershipErrorKey = null;
    try {
      const result = await createSessionApiClient(data.config.apiURL, () => current.token)
        .ownershipTransferCandidates(pathID, cursor || undefined);
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw sessionFailureFromResponse(502);
      if (!ticket.current() || session !== current || profile?.id !== ownerID || selectedPath?.id !== pathID) return;
      ownershipCandidates = mergeOwnershipTransferCandidates(ownershipCandidates, envelope.data, !cursor);
      ownershipCandidateNextCursor = envelope.meta.nextCursor ?? '';
    } catch {
      if (ticket.current() && session === current && profile?.id === ownerID && selectedPath?.id === pathID) {
        ownershipErrorKey = 'pathOwnership.unavailable';
      }
    } finally {
      if (ticket.current() && session === current && profile?.id === ownerID && selectedPath?.id === pathID) {
        ownershipCandidatesBusy = false;
      }
    }
  }

  function openOwnershipTransfer() {
    if (!session || !profile || !selectedPath || !effectivePathCapabilities(selectedPath).transferOwnership) return;
    resetOwnershipTransfer();
    ownershipOpen = true;
    void loadOwnershipCandidates();
  }

  function closeOwnershipTransfer() {
    resetOwnershipTransfer();
  }

  async function requestOwnershipTransferReview(
    current: ApplicationSession,
    pathID: string,
    recipientID: string,
  ): Promise<OwnershipTransferReview> {
    const response = generatedResponse(await createSessionApiClient(data.config.apiURL, () => current.token)
      .reviewOwnershipTransfer(pathID, { recipientUserId: recipientID }));
    if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
    const envelope = await response.json();
    const review = decodeOwnershipTransferReview(envelope?.data, pathID, recipientID);
    if (!review) throw sessionFailureFromResponse(502);
    return review;
  }

  async function selectOwnershipCandidate(candidate: OwnershipTransferCandidate) {
    if (!session || !profile || !selectedPath || ownershipMutationBusy || ownershipCandidatesBusy) return;
    if (!effectivePathCapabilities(selectedPath).transferOwnership) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    const ticket = ownershipOperations.issue();
    ownershipCandidatesBusy = true;
    ownershipErrorKey = null;
    try {
      const review = await requestOwnershipTransferReview(current, pathID, candidate.userId);
      if (!ticket.current() || session !== current || profile?.id !== ownerID || selectedPath?.id !== pathID) return;
      const expiration = ownershipTransferExpiration(review.reviewedAt, review.expiresAt, review.viewerTimeZone, i18n);
      if (!expiration) throw sessionFailureFromResponse(502);
      ownershipReview = review;
      ownershipSelectedCandidate = { ...review.recipient, administrator: candidate.administrator };
      ownershipReviewExpiration = expiration;
      ownershipReviewing = true;
      ownershipMutationKeys = {};
    } catch {
      if (ticket.current() && session === current && profile?.id === ownerID && selectedPath?.id === pathID) ownershipErrorKey = 'pathOwnership.unavailable';
    } finally {
      if (ticket.current() && session === current && profile?.id === ownerID && selectedPath?.id === pathID) ownershipCandidatesBusy = false;
    }
  }

  async function refreshOwnershipState(current: ApplicationSession, ownerID: string) {
    const [nextPaths, nextArchived] = await Promise.all([
      validateSessionCredential<SessionPath[]>(current, async (credential) =>
        generatedResponse(await createSessionApiClient(data.config.apiURL, () => credential.token).paths()),
      ),
      validateSessionCredential<CursorPage<SessionPath>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).archivedPaths(),
      )),
    ]);
    const ownershipResults = await Promise.all([...nextPaths, ...nextArchived.items].map(async (path) => {
      const result = await createSessionApiClient(data.config.apiURL, () => current.token).pendingOwnershipTransfer(path.id);
      const response = generatedResponse(result);
      if (response.status === 404) return null;
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope?.data) throw sessionFailureFromResponse(502);
      return ownershipTransferFromResult(envelope.data);
    }));
    if (session !== current || profile?.id !== ownerID) return;
    paths = nextPaths;
    archivedPaths = nextArchived.items;
    pendingOwnershipTransfers = currentPendingOwnershipTransfers(
      ownershipResults.flatMap((transfer) => transfer ? [transfer] : []),
    );
    if (selectedPath) selectedPath = nextPaths.find(({ id }) => id === selectedPath?.id) ?? null;
  }

  async function initiateOwnershipTransfer() {
    if (!session || !profile || !selectedPath || !ownershipSelectedCandidate || !ownershipReview || !ownershipReviewExpiration || ownershipMutationBusy) return;
    if (Date.parse(ownershipReview.expiresAt) <= now) {
      ownershipErrorKey = 'pathOwnership.unavailable';
      return;
    }
    if (!effectivePathCapabilities(selectedPath).transferOwnership) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    const recipient = ownershipSelectedCandidate;
    const review = ownershipReview;
    const key = `initiate:${pathID}:${recipient.userId}`;
    const idempotencyKey = ownershipMutationKeys[key] ?? crypto.randomUUID();
    ownershipMutationKeys = { ...ownershipMutationKeys, [key]: idempotencyKey };
    ownershipMutationBusy = true;
    ownershipMutationAction = 'initiate';
    ownershipMutationTargetID = recipient.userId;
    ownershipErrorKey = null;
    try {
      const result = await validateSessionCredential<OwnershipTransferResult>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).initiateOwnershipTransfer(
          pathID,
          { reservationToken: review.reservationToken },
          idempotencyKey,
        ),
      ));
      if (session !== current || profile?.id !== ownerID || selectedPath?.id !== pathID) return;
      pendingOwnershipTransfers = currentPendingOwnershipTransfers([
        ...pendingOwnershipTransfers,
        ownershipTransferFromResult(result),
      ]);
      await refreshOwnershipState(current, ownerID);
      delete ownershipMutationKeys[key];
      ownershipReviewing = false;
    } catch {
      if (session === current && profile?.id === ownerID && selectedPath?.id === pathID) ownershipErrorKey = 'pathOwnership.unavailable';
    } finally {
      if (session === current && profile?.id === ownerID) {
        ownershipMutationBusy = false;
        ownershipMutationAction = null;
        ownershipMutationTargetID = null;
      }
    }
  }

  async function mutateOwnershipTransfer(transfer: PresentedOwnershipTransfer, action: 'accept' | 'decline' | 'cancel') {
    if (!session || !profile || ownershipMutationBusy || transfer.state !== 'pending') return;
    const current = session;
    const ownerID = profile.id;
    const key = `${action}:${transfer.id}`;
    const idempotencyKey = ownershipMutationKeys[key] ?? crypto.randomUUID();
    ownershipMutationKeys = { ...ownershipMutationKeys, [key]: idempotencyKey };
    ownershipMutationBusy = true;
    ownershipMutationAction = action;
    ownershipMutationTargetID = transfer.id;
    ownershipErrorKey = null;
    try {
      const api = createSessionApiClient(data.config.apiURL, () => current.token);
      const operation = action === 'accept'
        ? await api.acceptOwnershipTransfer(transfer.id, idempotencyKey)
        : action === 'decline'
          ? await api.declineOwnershipTransfer(transfer.id, idempotencyKey)
          : await api.cancelOwnershipTransfer(transfer.id, idempotencyKey);
      await validateSessionCredential<OwnershipTransferResult>(current, async () => generatedResponse(operation));
      if (session !== current || profile?.id !== ownerID) return;
      await refreshOwnershipState(current, ownerID);
      delete ownershipMutationKeys[key];
    } catch {
      if (session === current && profile?.id === ownerID) ownershipErrorKey = 'pathOwnership.unavailable';
    } finally {
      if (session === current && profile?.id === ownerID) {
        ownershipMutationBusy = false;
        ownershipMutationAction = null;
        ownershipMutationTargetID = null;
      }
    }
  }

  async function loadManagedPathInvitations(
    cursor: string,
    replace: boolean,
    expectedSession: ApplicationSession,
    expectedOwnerID: string,
    pathID: string,
  ) {
    if (
      session !== expectedSession ||
      profile?.id !== expectedOwnerID ||
      selectedPath?.id !== pathID ||
      !sharingPath ||
      managedInvitationsLoading ||
      managedInvitationsLoadingMore
    ) return;
    const currentPath = paths?.find(({ id }) => id === pathID);
    if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) return;
    const ticket = managedInvitationListOperations.issue();
    if (cursor) managedInvitationsLoadingMore = true;
    else managedInvitationsLoading = true;
    managedInvitationsFailed = false;
    managedInvitationRetryCursor = cursor;
    try {
      const result = await createSessionApiClient(data.config.apiURL, () => expectedSession.token)
        .managedPathInvitations(pathID, cursor || undefined);
      const response = generatedResponse(result);
      if (!response.ok) throw pathInvitationFailureFromProblem(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw { kind: 'invalid_response' } as const;
      if (
        !ticket.current() ||
        session !== expectedSession ||
        profile?.id !== expectedOwnerID ||
        selectedPath?.id !== pathID ||
        !sharingPath
      ) return;
      const currentPath = paths?.find(({ id }) => id === pathID);
      if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) {
        resetInvitationShare();
        return;
      }
      const page = {
        items: envelope.data as ManagedPendingPathInvitation[],
        nextCursor: envelope.meta.nextCursor ?? '',
      };
      managedInvitations = mergeManagedPendingInvitationPage(managedInvitations, page, cursor);
      managedInvitationRetryCursor = '';
    } catch {
      if (
        ticket.current() &&
        session === expectedSession &&
        profile?.id === expectedOwnerID &&
        selectedPath?.id === pathID &&
        sharingPath
      ) managedInvitationsFailed = true;
    } finally {
      if (
        ticket.current() &&
        session === expectedSession &&
        profile?.id === expectedOwnerID &&
        selectedPath?.id === pathID &&
        sharingPath
      ) {
        managedInvitationsLoading = false;
        managedInvitationsLoadingMore = false;
      }
    }
  }

  function beginManagedInvitationCancellation(managed: ManagedPendingPathInvitation) {
    if (
      !selectedPath ||
      !sharingPath ||
      managedInvitationCancelBusy ||
      !effectivePathCapabilities(selectedPath).inviteMembers ||
      managed.invitation.pathId !== selectedPath.id ||
      !managedInvitations.items.some(({ invitation }) => invitation.id === managed.invitation.id)
    ) return;
    managedInvitationCancelReview = managed;
    managedInvitationCancelFailed = false;
  }

  function cancelManagedInvitationCancellation() {
    if (managedInvitationCancelBusy) return;
    managedInvitationCancelReview = null;
    managedInvitationCancelFailed = false;
  }

  async function confirmManagedInvitationCancellation() {
    if (!session || !profile || !selectedPath || !sharingPath || managedInvitationCancelBusy) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    const review = managedInvitationCancelReview;
    if (
      !review ||
      review.invitation.pathId !== pathID ||
      !effectivePathCapabilities(selectedPath).inviteMembers ||
      !managedInvitations.items.some(({ invitation }) => invitation.id === review.invitation.id)
    ) return;
    const invitationID = review.invitation.id;
    managedInvitationCancelBusy = true;
    managedInvitationCancelFailed = false;
    const result = await managedInvitationCancelOwner.submit(
      pathID,
      invitationID,
      true,
      async (_pathID, _invitationID, idempotencyKey) => pathInvitationOutputData(
        await invitationResponse(
          await createSessionApiClient(data.config.apiURL, () => current.token)
            .cancelPathInvitation(pathID, invitationID, idempotencyKey),
        ),
      ),
    );
    if (
      session !== current ||
      profile?.id !== ownerID ||
      selectedPath?.id !== pathID ||
      !sharingPath
    ) return;
    managedInvitationCancelBusy = false;
    const currentPath = paths?.find(({ id }) => id === pathID);
    if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) {
      resetInvitationShare();
      return;
    }
    if (result.kind === 'failed') {
      managedInvitationCancelFailed = true;
      return;
    }
    if (result.kind === 'canceled') {
      managedInvitationListOperations.invalidate();
      managedInvitationsLoading = false;
      managedInvitationsLoadingMore = false;
      managedInvitations = {
        ...managedInvitations,
        items: managedInvitations.items.filter(({ invitation }) => invitation.id !== invitationID),
      };
      managedInvitationCancelReview = null;
      managedInvitationCancelFailed = false;
    }
  }

  function openPathSharing() {
    if (!session || !profile || !selectedPath || !effectivePathCapabilities(selectedPath).inviteMembers) return;
    resetInvitationShare();
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    sharingPath = true;
    invitationOwnerID = ownerID;
    void loadManagedPathInvitations('', true, current, ownerID, pathID);
  }

  function cancelPathSharing() {
    resetInvitationShare();
  }

  function changeInvitationUsername(username: string) {
    invitationReviewOwner.cancel();
    invitationSendOwner.cancel(selectedPath?.id);
    invitationUsername = username;
    invitationReview = null;
    invitationErrorKey = null;
    invitationSent = false;
  }

  async function reviewInvitationRecipient() {
    if (!session || !profile || !selectedPath || invitationReviewBusy || !sharingPath) return;
    const path = paths?.find((candidate) => candidate.id === selectedPath?.id);
    if (!path || !effectivePathCapabilities(path).inviteMembers) return;
    const exactUsername = invitationUsername;
    if (!exactUsername || exactUsername.trim() !== exactUsername) {
      invitationErrorKey = 'pathInvitation.unavailable';
      return;
    }
    const current = session;
    const ownerID = profile.id;
    const pathID = path.id;
    invitationOwnerID = ownerID;
    invitationReviewBusy = true;
    invitationErrorKey = null;
    invitationSent = false;
    const result = await invitationReviewOwner.review(pathID, exactUsername, async (_pathID, _username) =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(data.config.apiURL, () => current.token)
        .reviewPathInvitationRecipient(pathID, exactUsername))),
    );
    if (session !== current || invitationOwnerID !== ownerID || selectedPath?.id !== pathID) return;
    invitationReviewBusy = false;
    const currentPath = paths?.find((candidate) => candidate.id === pathID);
    if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) {
      resetInvitationShare();
      return;
    }
    if (result.kind === 'reviewed') invitationReview = result.review;
    else if (result.kind === 'failed') invitationErrorKey = pathInvitationFailureMessageKey(result.failure);
  }

  function chooseInvitationRole(role: PathInvitationRole) {
    if (!selectedPath || !effectivePathCapabilities(selectedPath).inviteMembers || invitationSendBusy) return;
    if (role === invitationRole) return;
    invitationSendOwner.cancel(selectedPath.id);
    invitationRole = role;
    invitationErrorKey = null;
    invitationSent = false;
  }

  async function sendReviewedInvitation() {
    if (!session || !profile || !selectedPath || !invitationReview || invitationSendBusy || !sharingPath) return;
    const path = paths?.find((candidate) => candidate.id === selectedPath?.id);
    if (!path || !effectivePathCapabilities(path).inviteMembers || invitationReview.pathId !== path.id) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = path.id;
    const review = invitationReview;
    invitationOwnerID = ownerID;
    invitationSendBusy = true;
    invitationErrorKey = null;
    const result = await invitationSendOwner.submit(review, invitationRole, true, async (_pathID, body, idempotencyKey) =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(data.config.apiURL, () => current.token)
        .sendPathInvitation(pathID, body, idempotencyKey))),
    );
    if (session !== current || invitationOwnerID !== ownerID || selectedPath?.id !== pathID) return;
    invitationSendBusy = false;
    const currentPath = paths?.find((candidate) => candidate.id === pathID);
    if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) {
      resetInvitationShare();
      return;
    }
    if (result.kind === 'sent') {
      invitationSent = true;
      managedInvitationListOperations.invalidate();
      managedInvitationsLoading = false;
      managedInvitationsLoadingMore = false;
      await loadManagedPathInvitations('', true, current, ownerID, pathID);
    }
    else if (result.kind === 'failed') invitationErrorKey = pathInvitationFailureMessageKey(result.failure);
  }

  function acceptPendingInvitation(invitationID: string) {
    if (!session || !profile || pendingInvitationsBusy || pendingInvitationBusy[invitationID]) return;
    const pending = pendingInvitations.items.find((candidate) => candidate.invitation.id === invitationID);
    if (!pending) return;
    if (pendingInvitationReview && pendingInvitationReview.invitationId !== invitationID) {
      invitationAcceptOwner.cancel(pendingInvitationReview.invitationId);
      pendingInvitationReview = null;
    }
    const review = reviewPendingPathInvitationAcceptance(pending);
    invitationOwnerID = profile.id;
    pendingInvitationErrors = { ...pendingInvitationErrors, [invitationID]: undefined };
    if (review.kind === 'confirmation-required') {
      invitationAcceptOwner.cancel(invitationID);
      pendingInvitationFocusTarget = { kind: 'confirm', invitationID };
      pendingInvitationReview = review;
      return;
    }
    if (review.kind === 'ready') void submitPendingInvitationAcceptance(review, true);
  }

  function confirmPendingInvitation(invitationID: string) {
    const review = pendingInvitationReview;
    if (!review || review.invitationId !== invitationID || pendingInvitationsBusy) return;
    void submitPendingInvitationAcceptance(review, true);
  }

  function cancelPendingInvitationReview(invitationID: string) {
    if (pendingInvitationReview?.invitationId !== invitationID) return;
    invitationAcceptOwner.cancel(invitationID);
    pendingInvitationFocusTarget = { kind: 'accept', invitationID };
    pendingInvitationReview = null;
    pendingInvitationBusy = { ...pendingInvitationBusy, [invitationID]: false };
    pendingInvitationErrors = { ...pendingInvitationErrors, [invitationID]: undefined };
  }

  function pendingInvitationFocus(
    node: HTMLElement,
    kind: 'accept' | 'confirm',
    invitationID: string,
  ) {
    if (
      pendingInvitationFocusTarget?.kind === kind &&
      pendingInvitationFocusTarget.invitationID === invitationID
    ) {
      focusAccessibleElement(node);
      pendingInvitationFocusTarget = null;
    }
  }

  function focusPendingInvitationAccept(node: HTMLElement, invitationID: string) {
    pendingInvitationFocus(node, 'accept', invitationID);
  }

  function focusPendingInvitationConfirm(node: HTMLElement, invitationID: string) {
    pendingInvitationFocus(node, 'confirm', invitationID);
  }

  async function submitPendingInvitationAcceptance(
    review: PathInvitationAcceptanceReview,
    confirmed: boolean,
  ) {
    const invitationID = review.invitationId;
    if (!session || !profile || pendingInvitationsBusy || pendingInvitationBusy[invitationID]) return;
    const pending = pendingInvitations.items.find(
      (candidate) => candidate.invitation.id === invitationID,
    );
    if (!pending) {
      invitationAcceptOwner.cancel(invitationID);
      pendingInvitationReview = null;
      return;
    }
    const current = session;
    const ownerID = profile.id;
    invitationOwnerID = ownerID;
    pendingInvitationBusy = { ...pendingInvitationBusy, [invitationID]: true };
    pendingInvitationErrors = { ...pendingInvitationErrors, [invitationID]: undefined };
    const result = await invitationAcceptOwner.submit(review, confirmed, async (id, idempotencyKey, body) =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(data.config.apiURL, () => current.token)
        .acceptPathInvitation(id, idempotencyKey, body))),
    );
    if (session !== current || invitationOwnerID !== ownerID) return;
    if (result.kind === 'cancelled') {
      pendingInvitationBusy = { ...pendingInvitationBusy, [invitationID]: false };
      pendingInvitationReview = null;
      return;
    }
    if (result.kind === 'failed') {
      if (pendingInvitationReview?.invitationId === invitationID) pendingInvitationReview = null;
      pendingInvitationErrors = {
        ...pendingInvitationErrors,
        [invitationID]: pathInvitationFailureMessageKey(result.failure),
      };
      await loadPendingInvitations('', true);
      if (session === current && invitationOwnerID === ownerID) {
        pendingInvitationBusy = { ...pendingInvitationBusy, [invitationID]: false };
      }
      return;
    }
    pendingInvitationBusy = { ...pendingInvitationBusy, [invitationID]: false };
    if (result.kind !== 'accepted') return;
    pendingInvitations = {
      ...pendingInvitations,
      items: pendingInvitations.items.filter((candidate) => candidate.invitation.id !== invitationID),
    };
    if (pendingInvitationReview?.invitationId === invitationID) pendingInvitationReview = null;
    acceptedInvitation = { role: result.invitation.offeredRole };
    await attemptProfile(current, current.expiresAt);
  }

  function reviewArchiveChange() {
    if (!selectedPath || !effectivePathCapabilities(selectedPath).manageLifecycle || archiveBusy) return;
    archiveErrorKey = null;
    archiveStatusKey = null;
    archiveReview = reviewPathArchiveChange(selectedPath);
  }

  function cancelArchiveChange() {
    if (archiveReview) archiveOperations.cancel(archiveReview.pathId);
    archiveReview = null;
    archiveErrorKey = null;
  }

  async function confirmArchiveChange() {
    if (!session || !paths || !selectedPath || !effectivePathCapabilities(selectedPath).manageLifecycle || !archiveReview || archiveBusy) return;
    const current = session;
    const review = archiveReview;
    archiveBusy = true;
    archiveErrorKey = null;
    let unarchivedTimer: TimerState | undefined;
    const result = await archiveOperations.submit(review, true, async (pathID, body, idempotencyKey) => {
      const path = await validateSessionCredential<SessionPath>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).setPathArchiveState(pathID, body as PathArchiveStateDraft, idempotencyKey),
      ));
      if (!body.archived && effectivePathCapabilities(path).trackTime) {
        unarchivedTimer = await validateSessionCredential<TimerState>(current, async (credential) => generatedResponse(
          await createSessionApiClient(data.config.apiURL, () => credential.token).currentTimer(pathID),
        ));
      }
      return path;
    });
    if (result.kind === 'superseded') return;
    archiveBusy = false;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      archiveErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      if (presentation.discardCredential) handlePathDetailFailure(failure);
      return;
    }
    if (result.kind !== 'applied') return;
    const restoredTimerStates = unarchivedTimer
      ? { ...timerStates, [review.pathId]: { ...unarchivedTimer, running: false, timer: undefined } }
      : timerStates;
    const applied = applyPathArchiveResult({ activePaths: paths, archivedPaths, selectedPath, timerStates: restoredTimerStates }, result.path);
    paths = applied.activePaths as SessionPath[];
    archivedPaths = applied.archivedPaths as SessionPath[];
    selectedPath = applied.selectedPath as SessionPath;
    timerStates = applied.timerStates as Record<string, TimerState>;
    archiveStatusKey = result.path.archivedAt ? 'pathArchive.archived' : 'pathArchive.unarchived';
    archiveReview = null;
  }

  function openPathManagement() {
    if (!selectedPath || !profile) return;
    const capabilities = effectivePathCapabilities(selectedPath);
    if (!(capabilities.manageGoals || capabilities.renamePath || capabilities.manageLifecycle || capabilities.manageVisibility)) return;
    if (manualBusy || deleteBusy || timerBusy[selectedPath.id]) return;
    resetGoalManagement();
    resetPathRename();
    goalForm = goalFormState(selectedPath);
    goalUpdateOwnerID = profile.id;
    managingPathGoals = true;
    if (capabilities.manageVisibility) pathVisibilityDraft = pathVisibilityFromAPI(selectedPath.visibility);
    if (effectivePathCapabilities(selectedPath).renamePath) {
      pathRenameDraft = selectedPath.name;
      renamingPath = true;
    }
  }

  async function openPathMembers(cursor = '', replace = true, openDestination = true) {
    if (!session || !selectedPath || !profile) return;
    const current = session;
    const pathID = selectedPath.id;
    const ticket = pathMemberListOperations.issue();
    if (openDestination) pathMembersOpen = true;
    pathMembersFailed = false;
    if (replace) pathMembersLoading = true;
    else pathMembersLoadingMore = true;
    try {
      const page = await validateSessionCredential<CursorPage<PathMember>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).pathMembers(pathID, cursor || undefined),
      ));
      if (!ticket.current() || session !== current || selectedPath?.id !== pathID) return;
      const people = page.items.map((member) => ({ ...member, isViewer: member.userId === profile?.id }));
      pathMembers = replace ? people : [
        ...pathMembers,
        ...people.filter((candidate) => !pathMembers.some(({ userId }) => userId === candidate.userId)),
      ];
      pathMembersNextCursor = page.nextCursor ?? '';
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathMembersFailed = true;
      if (presentation.discardCredential) handlePathDetailFailure(failure);
    } finally {
      if (ticket.current()) {
        pathMembersLoading = false;
        pathMembersLoadingMore = false;
      }
    }
  }

  async function loadPathMemberActivities(member: PresentedPathMember, cursor = '') {
    if (!session || !selectedPath) return;
    const current = session;
    const pathID = selectedPath.id;
    const ticket = pathMemberActivityOperations.issue();
    pathMemberActivitiesLoading = true;
    pathMemberActivitiesFailed = false;
    try {
      const page = await validateSessionCredential<CursorPage<ActivityDetail>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activities(pathID, cursor || undefined, member.userId),
      ));
      if (!ticket.current() || session !== current || selectedPath?.id !== pathID || selectedPathMember?.userId !== member.userId) return;
      pathMemberActivities = cursor ? mergeActivityHistory(pathMemberActivities, page.items, false) : page.items;
      pathMemberActivitiesNextCursor = page.nextCursor ?? '';
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (webSessionFailure(failure).discardCredential) handlePathDetailFailure(failure);
      pathMemberActivitiesFailed = true;
    } finally {
      if (ticket.current()) pathMemberActivitiesLoading = false;
    }
  }

  async function inspectPathMember(member: PresentedPathMember) {
    if (!session || !selectedPath) return;
    const current = session;
    const pathID = selectedPath.id;
    const ticket = pathMemberReviewOperations.issue();
    selectedPathMember = member;
    pathMemberRemovalReview = null;
    pathMemberReviewLoading = true;
    pathMemberRemovalError = false;
    pathMemberPendingRole = null;
    pathMemberRoleChangeError = false;
    pathMemberActivities = [];
    pathMemberActivitiesNextCursor = '';
    pathMemberActivitiesFailed = false;
    void loadPathMemberActivities(member);
    if (!member.canRemove) {
      pathMemberReviewLoading = false;
      return;
    }
    try {
      const response = await validateSessionCredential<GeneratedMemberRemovalReview>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).reviewPathMemberRemoval(pathID, member.userId),
      ));
      const review = reviewPathMemberRemoval({ ...response, pathId: pathID });
      if (!ticket.current() || session !== current || selectedPath?.id !== pathID || selectedPathMember?.userId !== member.userId) return;
      pathMemberRemovalReview = review;
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (webSessionFailure(failure).discardCredential) handlePathDetailFailure(failure);
    } finally { if (ticket.current()) pathMemberReviewLoading = false; }
  }

  function openComparedPathMember(member: PathMember) {
    const presented = pathMembers.find(({ userId }) => userId === member.userId);
    if (!presented) return;
    pathMembersOpen = true;
    void inspectPathMember(presented);
  }

  async function removeSelectedPathMember() {
    if (!session || !selectedPath || !pathMemberRemovalReview || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy || selectedPath.archivedAt) return;
    const current = session;
    const review = pathMemberRemovalReview;
    const pathID = selectedPath.id;
    if (review.pathId !== pathID || effectivePathCapabilities(selectedPath).manageMembers !== true) return;
    pathMemberRemovalBusy = true;
    pathMemberRemovalError = false;
    const result = await pathMemberRemovalOperations.submit(review, true, (requestedPathID, userID, body, idempotencyKey) =>
      validateSessionCredential<MemberRemovalReceipt>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).removePathMember(requestedPathID, userID, body, idempotencyKey),
      )),
    );
    if (session !== current || selectedPath?.id !== pathID || selectedPathMember?.userId !== review.userId) {
      if (result.kind === 'applied' && pathMembersOpen && session === current && selectedPath?.id === pathID) void openPathMembers();
      return;
    }
    pathMemberRemovalBusy = false;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathMemberRemovalError = true;
      if (presentation.discardCredential) handlePathDetailFailure(failure);
      return;
    }
    if (result.kind !== 'applied') return;
    const applied = applyPathMemberRemovalResult({
      members: pathMembers.map((member) => ({ ...member, pathId: review.pathId })),
      selected: review,
    }, result);
    pathMembers = applied.members.map(({ pathId: _pathId, ...member }) => member);
    closePathMemberReview();
  }

  function choosePathMemberRole(role: PathMemberAccessRole) {
    if (!selectedPathMember || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy) return;
    const allowed = role === 'administrator' ? selectedPathMember.canGrantAdministrator
      : selectedPathMember.role === 'administrator' ? selectedPathMember.canRevokeAdministrator || selectedPathMember.canStepDownAdministrator
      : selectedPathMember.canChangeRole;
    if (!allowed || (selectedPathMember.role !== 'administrator' && !pathMemberRemovalReview)) return;
    if (role === selectedPathMember.role) return;
    pathMemberRoleChangeError = false;
    if (role === 'supporter' || role === 'administrator' || selectedPathMember.role === 'administrator') pathMemberPendingRole = role;
    else void changeSelectedPathMemberRole(role);
  }

  function roleChangeReviewSource(member: PresentedPathMember, pathId: string) {
    return pathMemberRemovalReview ?? {
      displayName: member.displayName,
      pathId,
      role: member.role,
      runningTimer: false,
      sessionCount: member.sessionCount,
      totalTrackedSeconds: member.totalTrackedSeconds,
      userId: member.userId,
      username: member.username,
    };
  }

  function cancelPathMemberRoleChange() {
    if (pathMemberRoleChangeBusy || !selectedPath || !selectedPathMember) return;
    pathMemberRoleChangeOperations.cancel(selectedPath.id, selectedPathMember.userId);
    pathMemberPendingRole = null;
    pathMemberRoleChangeError = false;
  }

  async function changeSelectedPathMemberRole(role = pathMemberPendingRole) {
    if (!session || !selectedPath || !selectedPathMember || !role
      || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy || selectedPath.archivedAt) return;
    const current = session;
    const pathID = selectedPath.id;
    const member = selectedPathMember;
    const allowed = role === 'administrator' ? member.canGrantAdministrator
      : member.role === 'administrator' ? member.canRevokeAdministrator || member.canStepDownAdministrator
      : member.canChangeRole;
    if (!allowed) return;
    const review = reviewPathMemberRoleChange(roleChangeReviewSource(member, pathID), role);
    pathMemberRoleChangeBusy = true;
    pathMemberRoleChangeError = false;
    const result = await pathMemberRoleChangeOperations.submit(review, true, (requestedPathID, userID, body, idempotencyKey) =>
      validateSessionCredential<PathMemberRoleChangeReceipt>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).changePathMemberRole(requestedPathID, userID, body, idempotencyKey),
      )),
    );
    if (session !== current || selectedPath?.id !== pathID || selectedPathMember?.userId !== member.userId) return;
    pathMemberRoleChangeBusy = false;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (webSessionFailure(failure).discardCredential) handlePathDetailFailure(failure);
      pathMemberRoleChangeError = true;
      return;
    }
    if (result.kind !== 'applied') return;
    const applied = applyPathMemberRoleChangeResult({ members: pathMembers.map((candidate) => ({ ...candidate, pathId: pathID })) }, result);
    pathMembers = applied.members.map(({ pathId: _pathId, ...candidate }) => candidate.userId === member.userId ? {
      ...candidate,
      canChangeRole: false,
      canGrantAdministrator: false,
      canRemove: false,
      canRevokeAdministrator: false,
      canStepDownAdministrator: false,
    } : candidate);
    selectedPathMember = pathMembers.find((candidate) => candidate.userId === member.userId) ?? null;
    if (result.receipt.activityDeleted) {
      pathMemberActivities = [];
      pathMemberActivitiesNextCursor = '';
    }
    pathMemberPendingRole = null;
    await openPathMembers('', true, false);
    const refreshed = pathMembers.find((candidate) => candidate.userId === member.userId);
    if (refreshed) await inspectPathMember(refreshed);
  }

  async function unblockPathMember(member: PresentedPathMember) {
    if (!session || !profile || !member.blockedByViewer || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath?.id;
    const ticket = pathMemberUnblockOperations.issue();
    pathMemberUnblockBusy = true;
    pathMemberUnblockError = false;
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token)
          .unblockAccount(member.userId, crypto.randomUUID()),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const result = unblockResultFromAPI(await response.json());
      if (!ticket.current() || session !== current || profile?.id !== ownerID || selectedPath?.id !== pathID || selectedPathMember?.userId !== member.userId || result.target.userId !== member.userId) return;
      pathMembers = pathMembers.map((candidate) => candidate.userId === member.userId
        ? { ...candidate, blockedByViewer: false }
        : candidate);
      if (selectedPathMember?.userId === member.userId) selectedPathMember = { ...selectedPathMember, blockedByViewer: false };
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (webSessionFailure(failure).discardCredential) handlePathDetailFailure(failure);
      pathMemberUnblockError = true;
    } finally {
      if (ticket.current()) pathMemberUnblockBusy = false;
    }
  }

  async function openPathMemberActivity(activityID: string) {
    closePathMembers();
    historyOpen = true;
    await inspectActivity(activityID);
  }

  function showModal(node: HTMLDialogElement) {
    node.showModal();
    void Promise.resolve().then(() => node.querySelector<HTMLInputElement>('input[name="path-leave-retention"]')?.focus());
    return { destroy: () => { if (node.open) node.close(); } };
  }

  function beginPathLeaveReview(event: MouseEvent) {
    if (!selectedPath || pathLeaveBusy || effectivePathCapabilities(selectedPath).leavePath !== true) return;
    pathLeaveReturnFocus = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
    pathLeaveReview = reviewPathLeave(selectedPath, true);
    pathLeaveErrorKey = null;
  }

  function choosePathLeaveRetention(retainActivity: boolean) {
    if (!selectedPath || !pathLeaveReview || pathLeaveBusy) return;
    if (!retainActivity && effectivePathCapabilities(selectedPath).trackTime !== true) return;
    if (pathLeaveReview.retainActivity !== retainActivity) pathLeaveOperations.cancel(pathLeaveReview.pathId);
    pathLeaveReview = reviewPathLeave(selectedPath, retainActivity);
    pathLeaveErrorKey = null;
  }

  function cancelPathLeave() {
    if (pathLeaveBusy || !pathLeaveReview) return;
    pathLeaveOperations.cancel(pathLeaveReview.pathId);
    pathLeaveReview = null;
    pathLeaveErrorKey = null;
    const returnFocus = pathLeaveReturnFocus;
    pathLeaveReturnFocus = null;
    void Promise.resolve().then(() => returnFocus?.focus());
  }

  async function confirmPathLeave() {
    if (!session || !profile || !paths || !selectedPath || !pathLeaveReview || pathLeaveBusy) return;
    const current = session;
    const ownerID = profile.id;
    const review = pathLeaveReview;
    if (selectedPath.id !== review.pathId || selectedPath.name !== review.pathName || effectivePathCapabilities(selectedPath).leavePath !== true || (!review.retainActivity && effectivePathCapabilities(selectedPath).trackTime !== true)) return;
    pathLeaveBusy = true;
    pathLeaveErrorKey = null;
    const result = await pathLeaveOperations.submit(review, true, (pathID, body, idempotencyKey) =>
      validateSessionCredential<PathLeaveReceipt>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).leavePath(pathID, body, idempotencyKey),
      )),
    );
    if (session !== current || profile?.id !== ownerID) return;
    pathLeaveBusy = false;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathLeaveErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      if (presentation.discardCredential) handlePathDetailFailure(failure);
      return;
    }
    if (result.kind !== 'applied') return;
    const applied = applyPathLeaveResult({ activePaths: paths, archivedPaths, selectedPath, timerStates }, result);
    paths = applied.activePaths as SessionPath[];
    archivedPaths = applied.archivedPaths as SessionPath[];
    selectedPath = applied.selectedPath as SessionPath | null;
    timerStates = applied.timerStates as Record<string, TimerState>;
    resetPathDetails();
    pathLeaveStatus = { pathName: review.pathName, retained: result.receipt.activityRetained };
    pathLeaveReturnFocus = null;
  }

  function beginPathDeletionReview() {
    if (!selectedPath || pathDeletionBusy || !effectivePathCapabilities(selectedPath).manageLifecycle) return;
    pathDeletionReview = reviewPathDeletion(selectedPath);
    pathDeletionErrorKey = null;
  }

  function cancelPathDeletion() {
    if (pathDeletionBusy || !pathDeletionReview) return;
    pathDeletionOperations.cancel(pathDeletionReview.pathId);
    pathDeletionReview = null;
    pathDeletionErrorKey = null;
  }

  async function confirmPathDeletion() {
    if (!session || !profile || !paths || !selectedPath || !pathDeletionReview || pathDeletionBusy) return;
    const current = session;
    const ownerID = profile.id;
    const review = pathDeletionReview;
    if (selectedPath.id !== review.pathId || selectedPath.name !== review.expectedName || !effectivePathCapabilities(selectedPath).manageLifecycle) return;
    pathDeletionBusy = true;
    pathDeletionErrorKey = null;
    const result = await pathDeletionOperations.submit(review, true, (pathID, body, idempotencyKey) =>
      validateSessionCredential<{ pathId: string; deleted: true }>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).deletePath(pathID, body, idempotencyKey),
      )),
    );
    if (session !== current || profile?.id !== ownerID) return;
    pathDeletionBusy = false;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathDeletionErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      if (presentation.discardCredential) handlePathDetailFailure(failure);
      return;
    }
    if (result.kind !== 'applied') return;
    const applied = applyPathDeletionResult({ activePaths: paths, archivedPaths, selectedPath, timerStates }, result);
    paths = applied.activePaths as SessionPath[];
    archivedPaths = applied.archivedPaths as SessionPath[];
    timerStates = applied.timerStates as Record<string, TimerState>;
    selectedPath = null;
    resetPathDetails();
    resetGoalManagement();
    resetPathRename();
  }

  function reviewGoalChanges() {
    if (!selectedPath || !effectivePathCapabilities(selectedPath).manageGoals || !goalForm || goalUpdateBusy) return;
    goalUpdateErrorKey = null;
    const goalDraft = buildGoalDraft(goalForm);
    if (!goalDraft.ok) {
      goalUpdateErrorKey = goalDraft.reason === 'duration' ? 'pathCreate.durationInvalid' : 'pathCreate.alignmentInvalid';
      return;
    }
    goalReview = compareGoalConfigurations({
      intervalGoal: selectedPath.intervalGoal,
      overallTarget: selectedPath.overallTarget,
    }, goalDraft.goals);
    goalUpdateIdempotencyKey = '';
  }

  function cancelGoalChanges() {
    resetGoalManagement();
    resetPathRename();
  }

  async function confirmGoalChanges() {
    if (!session || !profile || !paths || !selectedPath || !effectivePathCapabilities(selectedPath).manageGoals || !goalReview || !goalReview.changed || goalUpdateBusy || sessionOperationBusy) return;
    if (goalReview.proposed.intervalGoal && !goalReview.proposed.intervalGoal.alignment) {
      goalUpdateErrorKey = 'pathCreate.alignmentInvalid';
      return;
    }
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    const ticket = goalManagementOperations.issue();
    goalUpdateOwnerID = ownerID;
    if (!goalUpdateIdempotencyKey) goalUpdateIdempotencyKey = crypto.randomUUID();
    const idempotencyKey = goalUpdateIdempotencyKey;
    const body = { confirmed: true, expectedGoals: goalReview.current, ...goalReview.proposed } as PathGoalUpdateDraft;
    goalUpdateBusy = true;
    if (refreshTimer) clearTimeout(refreshTimer);
    goalUpdateErrorKey = null;
    try {
      const result = await validateSessionCredential<PathGoalMutationResult>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).updatePathGoals(pathID, body, idempotencyKey),
      ));
      if (!ticket.current() || goalUpdateOwnerID !== ownerID || selectedPath?.id !== pathID || session !== current) return;
      if (result.path.id !== pathID) {
        goalUpdateErrorKey = 'errors.apiRejected';
        return;
      }
      const applied = applyGoalMutationResult({ paths, selectedPath, timerStates }, result);
      paths = applied.paths as SessionPath[];
      selectedPath = applied.selectedPath as SessionPath;
      timerStates = applied.timerStates as Record<string, TimerState>;
      pathMembers = pathMembers.map((member) => ({ ...member, intervalProgress: undefined, overallProgress: undefined }));
      void openPathMembers('', true, false);
      resetGoalManagement();
      resetPathRename();
      goalUpdateSaved = true;
      scheduleOwnedSession();
    } catch (cause) {
      if (!ticket.current() || goalUpdateOwnerID !== ownerID || selectedPath?.id !== pathID || session !== current) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      if (!presentation.discardCredential) {
        goalUpdateErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
        return;
      }
      handlePathDetailFailure(failure);
    } finally {
      if (ticket.current() && goalUpdateOwnerID === ownerID && selectedPath?.id === pathID) {
        goalUpdateBusy = false;
        if (session === current) scheduleOwnedSession();
      }
    }
  }

  function reviewPathVisibility() {
    if (!profile || !selectedPath || !effectivePathCapabilities(selectedPath).manageVisibility || pathVisibilityBusy) return;
    pathVisibilityErrorKey = null;
    pathVisibilitySaved = false;
    const review = reviewPathVisibilityChange(visibilitySessionPath(selectedPath), pathVisibilityDraft, profile.profileVisibility);
    if (review.kind === 'unchanged') {
      pathVisibilityReview = null;
      return;
    }
    if (review.kind === 'not-permitted') {
      pathVisibilityReview = null;
      pathVisibilityErrorKey = 'errors.forbidden';
      return;
    }
    pathVisibilityReview = review;
    if (!review.broader) void submitPathVisibility(review);
  }

  function cancelPathVisibilityConfirmation() {
    pathVisibilityReview = null;
    pathVisibilityErrorKey = null;
    if (selectedPath) pathVisibilityDraft = pathVisibilityFromAPI(selectedPath.visibility);
  }

  async function confirmPathVisibility() {
    if (pathVisibilityReview?.broader) await submitPathVisibility(pathVisibilityReview);
  }

  async function reloadVisibilityPath(current: ApplicationSession, ownerID: string, pathID: string) {
    try {
      const [refreshedActive, refreshedArchived] = await Promise.all([
        validateSessionCredential<SessionPath[]>(current, async (credential) => generatedResponse(
          await createSessionApiClient(data.config.apiURL, () => credential.token).paths(),
        )),
        validateSessionCredential<CursorPage<SessionPath>>(current, async (credential) => cursorGeneratedResponse(
          await createSessionApiClient(data.config.apiURL, () => credential.token).archivedPaths(),
        )),
      ]);
      if (session !== current || profile?.id !== ownerID || selectedPath?.id !== pathID) return;
      paths = refreshedActive;
      archivedPaths = refreshedArchived.items;
      const authoritative = refreshedActive.find(({ id }) => id === pathID) ??
        refreshedArchived.items.find(({ id }) => id === pathID);
      if (!authoritative) {
        resetPathDetails();
        return;
      }
      selectedPath = authoritative;
      if (!effectivePathCapabilities(authoritative).manageVisibility) {
        resetGoalManagement();
        resetPathRename();
        return;
      }
      pathVisibilityDraft = pathVisibilityFromAPI(authoritative.visibility);
      pathVisibilityReview = null;
    } catch {
      // The original conflict remains visible and retryable if convergence cannot be loaded yet.
    }
  }

  async function submitPathVisibility(review: Extract<PathVisibilityChangeReview, { kind: 'ready' }>) {
    if (!session || !profile || !paths || !selectedPath || pathVisibilityBusy ||
      !effectivePathCapabilities(selectedPath).manageVisibility ||
      selectedPath.id !== review.pathId || selectedPath.visibility !== review.current) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    pathVisibilityBusy = true;
    pathVisibilityErrorKey = null;
    pathVisibilitySaved = false;
    const result = await pathVisibilityOperations.submit(review, true, async (requestedPathID, body, idempotencyKey) =>
      validateSessionCredential<SessionPath>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).setPathVisibility(requestedPathID, body, idempotencyKey),
      )).then(visibilitySessionPath),
    );
    if (session !== current || profile?.id !== ownerID || selectedPath?.id !== pathID) return;
    pathVisibilityBusy = false;
    if (result.kind === 'superseded' || result.kind === 'cancelled') return;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathVisibilityErrorKey = presentation.retryable ? 'pathVisibility.unavailable' : presentation.message;
      if (failure.kind === 'http' && failure.status === 409) await reloadVisibilityPath(current, ownerID, pathID);
      if (presentation.discardCredential) handlePathDetailFailure(failure);
      return;
    }
    const applied = applyPathVisibilityResult({
      activePaths: paths.map(visibilitySessionPath),
      archivedPaths: archivedPaths.map(visibilitySessionPath),
      selectedPath: visibilitySessionPath(selectedPath),
    }, result.path);
    paths = applied.activePaths as SessionPath[];
    archivedPaths = applied.archivedPaths as SessionPath[];
    selectedPath = applied.selectedPath as SessionPath;
    pathVisibilityDraft = result.path.visibility;
    pathVisibilityReview = null;
    pathVisibilitySaved = true;
    scheduleOwnedSession();
  }

  async function submitPathRename() {
    if (!session || !paths || !selectedPath || !managingPathGoals || !effectivePathCapabilities(selectedPath).renamePath || pathRenameBusy) return;
    let review: ReturnType<typeof reviewPathRename>;
    try {
      review = reviewPathRename(selectedPath, pathRenameDraft);
    } catch {
      pathRenameErrorKey = 'pathRename.noChanges';
      return;
    }
    if (!review.changed) {
      pathRenameErrorKey = 'pathRename.noChanges';
      return;
    }
    const current = session;
    pathRenameBusy = true;
    pathRenameErrorKey = null;
    const result = await pathRenameOperations.submit(review, async (pathID, body, idempotencyKey) =>
      validateSessionCredential<SessionPath>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).renamePath(pathID, body, idempotencyKey),
      )),
    );
    if (result.kind === 'superseded' || session !== current) return;
    pathRenameBusy = false;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const presentation = webSessionFailure(failure);
      pathRenameErrorKey = presentation.retryable ? 'errors.temporarilyUnavailable' : presentation.message;
      if (presentation.discardCredential) handlePathDetailFailure(failure);
      return;
    }
    if (result.kind !== 'applied') return;
    const applied = applyPathRenameResult({ paths, selectedPath }, result.path);
    paths = applied.paths as SessionPath[];
    selectedPath = applied.selectedPath as SessionPath;
    renamingPath = true;
    pathRenameDraft = result.path.name;
    pathRenameStatusName = result.path.name;
  }

  async function openPathHistory() {
    await loadActivityPage(undefined, true);
  }

  async function loadActivityPage(cursor: string | undefined, replace = false) {
    if (!session || !profile || !selectedPath || activityPageBusy || pathDetailBusy || revisionPageBusy) return;
    const current = session;
    const pathID = selectedPath.id;
    const ownerID = profile.id;
    const ticket = pathDetailOperations.issue();
    detailOwnerID = ownerID;
    activityPageBusy = true;
    pathDetailErrorKey = null;
    try {
      const page = await validateSessionCredential<CursorPage<ActivityDetail>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activities(pathID, cursor),
      ));
      if (!ticket.current() || detailOwnerID !== ownerID || selectedPath?.id !== pathID) return;
      activityHistoryItems = mergeActivityHistory(activityHistoryItems, page.items, replace);
      activityDays = groupActivitiesNewestFirst(activityHistoryItems);
      activityNextCursor = page.nextCursor ?? null;
      activityPageFailed = false;
      historyOpen = true;
    } catch (cause) {
      if (ticket.current() && detailOwnerID === ownerID && selectedPath?.id === pathID) {
        activityPageFailed = true;
        activityRetryCursor = cursor ?? null;
        activityRetryReplace = replace;
        handlePathDetailFailure(cause);
      }
    } finally {
      if (ticket.current() && detailOwnerID === ownerID && selectedPath?.id === pathID) activityPageBusy = false;
    }
  }

  async function retryActivityPage() {
    await loadActivityPage(activityRetryCursor ?? undefined, activityRetryReplace);
  }

  async function loadMoreActivities() {
    if (activityNextCursor) await loadActivityPage(activityNextCursor);
  }

  function closePathDetails() {
    resetManualActivity();
    resetPathDetails();
  }

  function beginDeleteSelectedActivity() {
    if (!profile || !selectedPath || !effectivePathCapabilities(selectedPath).trackTime || !selectedActivity || selectedActivity.activity.participantId !== profile.id || deleteBusy) return;
    resetDeleteActivity();
    deleteConfirmActivityID = selectedActivity.activity.id;
    deleteOwnerID = profile.id;
  }

  function cancelDeleteActivity() {
    resetDeleteActivity();
  }

  async function confirmDeleteActivity() {
    if (!session || !profile || !selectedPath || !effectivePathCapabilities(selectedPath).trackTime || !selectedActivity || deleteBusy || manualBusy || managingPathGoals) return;
    const current = session;
    const ownerID = profile.id;
    const pathID = selectedPath.id;
    const activityID = selectedActivity.activity.id;
    if (selectedActivity.activity.participantId !== ownerID || deleteOwnerID !== ownerID || deleteConfirmActivityID !== activityID) return;
    if (!deleteIdempotencyKey) deleteIdempotencyKey = crypto.randomUUID();
    const idempotencyKey = deleteIdempotencyKey;
    const ticket = deleteOperations.issue();
    deleteBusy = true;
    deleteFailed = false;
    pathDetailErrorKey = null;
    try {
      const result = await validateSessionCredential<ActivityDeletionResult>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).deleteActivity(pathID, activityID, idempotencyKey),
      ));
      if (!ticket.current() || deleteOwnerID !== ownerID || selectedPath?.id !== pathID || selectedActivity?.activity.id !== activityID || deleteConfirmActivityID !== activityID) return;
      resetManualActivity();
      activityHistoryItems = removeActivity(activityHistoryItems, activityID);
      activityDays = groupActivitiesNewestFirst(activityHistoryItems);
      timerStates = { ...timerStates, [pathID]: { ...(timerStates[pathID] ?? { running: false }), accumulatedSeconds: result.accumulatedSeconds, intervalProgress: result.intervalProgress } };
      selectedActivity = null;
      activityRevisions = [];
      revisionNextCursor = null;
      revisionPageFailed = false;
      revisionRetryCursor = null;
      revisionRetryReplace = true;
      revisionRetryActivityID = null;
      resetDeleteActivity();
    } catch (cause) {
      if (ticket.current() && deleteOwnerID === ownerID && selectedPath?.id === pathID && selectedActivity?.activity.id === activityID && deleteConfirmActivityID === activityID) {
        deleteFailed = true;
        handlePathDetailFailure(cause);
      }
    } finally {
      if (ticket.current() && deleteOwnerID === ownerID && selectedPath?.id === pathID && selectedActivity?.activity.id === activityID && deleteConfirmActivityID === activityID) deleteBusy = false;
    }
  }

  async function inspectActivity(activityID: string) {
    if (!session || !profile || !selectedPath || activityPageBusy || pathDetailBusy || revisionPageBusy) return;
    resetDeleteActivity();
    resetManualActivity();
    const current = session;
    const pathID = selectedPath.id;
    const ownerID = profile.id;
    const ticket = pathDetailOperations.issue();
    detailOwnerID = ownerID;
    pathDetailBusy = true;
    pathDetailErrorKey = null;
    selectedActivity = null;
    activityRevisions = [];
    revisionNextCursor = null;
    revisionPageFailed = false;
    let detailLoaded = false;
    try {
      const detail = await validateSessionCredential<ActivityDetail>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activity(pathID, activityID),
      ));
      if (!ticket.current() || detailOwnerID !== ownerID || selectedPath?.id !== pathID) return;
      selectedActivity = detail;
      detailLoaded = true;
      const page = await validateSessionCredential<CursorPage<ActivityRevision>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activityRevisions(pathID, activityID, undefined),
      ));
      if (!ticket.current() || detailOwnerID !== ownerID || selectedPath?.id !== pathID || selectedActivity?.activity.id !== activityID) return;
      activityRevisions = mergeRevisionHistory(activityRevisions, page.items, true);
      revisionNextCursor = page.nextCursor ?? null;
      revisionPageFailed = false;
    } catch (cause) {
      if (ticket.current() && detailOwnerID === ownerID && selectedPath?.id === pathID) {
        if (detailLoaded && selectedActivity?.activity.id === activityID) {
          revisionPageFailed = true;
          revisionRetryCursor = null;
          revisionRetryReplace = true;
          revisionRetryActivityID = activityID;
        }
        handlePathDetailFailure(cause);
      }
    } finally {
      if (ticket.current()) pathDetailBusy = false;
    }
  }

  async function loadRevisionPage(activityID: string, cursor: string | undefined, replace = false) {
    if (!session || !profile || !selectedPath || selectedActivity?.activity.id !== activityID || activityPageBusy || pathDetailBusy || revisionPageBusy) return;
    const current = session;
    const pathID = selectedPath.id;
    const ownerID = profile.id;
    const ticket = pathDetailOperations.issue();
    detailOwnerID = ownerID;
    revisionPageBusy = true;
    pathDetailErrorKey = null;
    try {
      const page = await validateSessionCredential<CursorPage<ActivityRevision>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activityRevisions(pathID, activityID, cursor),
      ));
      if (!ticket.current() || detailOwnerID !== ownerID || selectedPath?.id !== pathID || selectedActivity?.activity.id !== activityID) return;
      activityRevisions = mergeRevisionHistory(activityRevisions, page.items, replace);
      revisionNextCursor = page.nextCursor ?? null;
      revisionPageFailed = false;
    } catch (cause) {
      if (ticket.current() && detailOwnerID === ownerID && selectedPath?.id === pathID && selectedActivity?.activity.id === activityID) {
        revisionPageFailed = true;
        revisionRetryCursor = cursor ?? null;
        revisionRetryReplace = replace;
        revisionRetryActivityID = activityID;
        handlePathDetailFailure(cause);
      }
    } finally {
      if (ticket.current() && detailOwnerID === ownerID && selectedPath?.id === pathID && selectedActivity?.activity.id === activityID) revisionPageBusy = false;
    }
  }

  async function retryRevisionPage() {
    if (revisionRetryActivityID) await loadRevisionPage(revisionRetryActivityID, revisionRetryCursor ?? undefined, revisionRetryReplace);
  }

  async function loadMoreRevisions() {
    if (selectedActivity && revisionNextCursor) await loadRevisionPage(selectedActivity.activity.id, revisionNextCursor);
  }

  async function refreshPathActivity(pathID: string, activityID: string, ticket: SessionOperationTicket, ownerID: string | null) {
    if (!session) return;
    const current = session;
    const [activityPage, detail, revisionPage] = await Promise.all([
      validateSessionCredential<CursorPage<ActivityDetail>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activities(pathID, undefined),
      )),
      validateSessionCredential<ActivityDetail>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activity(pathID, activityID),
      )),
      validateSessionCredential<CursorPage<ActivityRevision>>(current, async (credential) => cursorGeneratedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).activityRevisions(pathID, activityID, undefined),
      )),
    ]);
    if (!ticket.current() || manualOwnerID !== ownerID || selectedPath?.id !== pathID) return;
    activityHistoryItems = mergeActivityHistory(activityHistoryItems, activityPage.items, true);
    activityDays = groupActivitiesNewestFirst(activityHistoryItems);
    activityNextCursor = activityPage.nextCursor ?? null;
    activityPageFailed = false;
    historyOpen = true;
    selectedActivity = detail;
    activityRevisions = mergeRevisionHistory(activityRevisions, revisionPage.items, true);
    revisionNextCursor = revisionPage.nextCursor ?? null;
    revisionPageFailed = false;
  }

  async function editSelectedActivity() {
    if (!session || !profile || !selectedPath || !effectivePathCapabilities(selectedPath).trackTime || !selectedActivity || selectedActivity.activity.participantId !== profile.id) return;
    const current = session;
    const pathID = selectedPath.id;
    const ownerID = profile.id;
    const activity = selectedActivity;
    const ticket = manualOperations.issue();
    manualOwnerID = ownerID;
    manualBusy = true;
    manualErrorKey = null;
    try {
      const defaults = await validateSessionCredential<ManualActivityDefaults>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).manualActivityDefaults(pathID),
      ));
      if (!ticket.current() || manualOwnerID !== ownerID) return;
      manualPathID = pathID;
      manualDefaults = defaults;
      manualDefaultsLoadedAt = Date.now();
      manualForm = activityEditForm(activity.activity);
      manualOccurrenceTimeZone = activity.activity.occurrenceTimeZone;
      manualNote = activity.activity.note ?? '';
      manualActivity = { id: activity.activity.id, version: activity.version };
      manualIdempotencyKey = crypto.randomUUID();
    } catch (cause) {
      if (ticket.current()) handleManualFailure(cause);
    } finally {
      if (ticket.current()) manualBusy = false;
    }
  }

  async function openManualActivity(pathID: string) {
    if (!session || !profile || managingPathGoals) return;
    const path = selectedPath?.id === pathID ? selectedPath : paths?.find((candidate) => candidate.id === pathID);
    if (!path || !effectivePathCapabilities(path).trackTime) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = manualOperations.issue();
    manualOwnerID = ownerID;
    manualBusy = true; manualErrorKey = null;
    try {
      const defaults = await validateSessionCredential<ManualActivityDefaults>(current, async (credential) => generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => credential.token).manualActivityDefaults(pathID),
      ));
      if (!ticket.current() || manualOwnerID !== ownerID) return;
      manualPathID = pathID; manualDefaults = defaults; manualDefaultsLoadedAt = Date.now();
      manualForm = createManualActivityFormState(manualActivityParticipantNow(defaults.currentInstant, defaults.timeZone));
      manualOccurrenceTimeZone = null; manualNote = ''; manualActivity = null; manualIdempotencyKey = crypto.randomUUID();
    } catch (cause) { if (ticket.current()) handleManualFailure(cause); }
    finally { if (ticket.current()) manualBusy = false; }
  }

  function changeManualOccurrence(patch: Partial<ManualActivityLocalDateTime>) {
    if (!manualForm) return;
    manualForm = overrideManualActivityOccurrence(manualForm, patch); manualIdempotencyKey = crypto.randomUUID(); manualErrorKey = null;
  }

  function changeManualDuration(value: string) {
    if (!manualForm) return;
    manualForm = updateManualActivityDuration(manualForm, value, manualLocalNow()); manualIdempotencyKey = crypto.randomUUID(); manualErrorKey = null;
  }

  async function submitManualActivity() {
    if (!session || !manualPathID || !manualForm || manualBusy || managingPathGoals) return;
    const path = selectedPath?.id === manualPathID ? selectedPath : paths?.find((candidate) => candidate.id === manualPathID);
    if (!path || !effectivePathCapabilities(path).trackTime) return;
    const serialized = serializeManualActivityForm(manualForm, manualLocalNow());
    if (!serialized.ok) { manualErrorKey = serialized.reason === 'future_end' ? 'activity.futureEnd' : 'activity.invalid'; return; }
    const current = session;
    const pathID = manualPathID;
    const activity = manualActivity;
    const idempotencyKey = manualIdempotencyKey;
    const ownerID = manualOwnerID;
    const ticket = manualOperations.issue();
    manualBusy = true; manualErrorKey = null;
    try {
      const body = { localDate: serialized.fields.localDate, localStartTime: serialized.fields.localTime, durationSeconds: serialized.fields.durationSeconds, note: manualNote || undefined };
      const result = await validateSessionCredential<ActivityMutationResult>(current, async (credential) => generatedResponse(
        activity
          ? await createSessionApiClient(data.config.apiURL, () => credential.token).updateActivity(pathID, activity.id, body, idempotencyKey)
          : await createSessionApiClient(data.config.apiURL, () => credential.token).createManualActivity(pathID, body, idempotencyKey),
      ));
      if (!ticket.current() || manualOwnerID !== ownerID) return;
      manualActivity = { id: result.activity.id, version: result.version };
      manualIdempotencyKey = crypto.randomUUID();
      timerStates = { ...timerStates, [pathID]: { ...(timerStates[pathID] ?? { running: false }), accumulatedSeconds: result.accumulatedSeconds, intervalProgress: result.intervalProgress } };
      try {
        await refreshPathActivity(pathID, result.activity.id, ticket, ownerID);
      } catch (cause) {
        if (ticket.current()) handlePathDetailFailure(cause);
      }
    } catch (cause) { if (ticket.current()) handleManualFailure(cause); }
    finally { if (ticket.current()) manualBusy = false; }
  }

  function showPrimarySurface(surface: PrimarySurface) {
    primarySurface = surface;
    if (surface === 'following') return;
    socialProfileDetailOperations.invalidate();
    selectedSocialProfile = null;
    selectedSocialUsername = '';
    socialProfileState = 'idle';
    socialFollowRequestsOpen = false;
  }

  function updateSocialQuery(value: string) {
    socialQuery = value;
    if (profileSearchQuery(value)) return;
    socialProfileSearchOwner.cancel();
    socialProfileSearchOperations.invalidate();
    socialSearch = { query: '', items: [], nextCursor: '' };
    socialSearchState = 'hint';
    socialLoadingMore = false;
  }

  async function requestSocialProfiles(
    current: ApplicationSession,
    query: string,
    cursor: string,
    ticket: SessionOperationTicket,
  ) {
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token)
          .searchProfiles(query, cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      return profileSearchPageFromAPI(await response.json());
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      throw cause;
    }
  }

  async function searchSocialProfiles() {
    if (!session || !profile || primarySurface !== 'following') return;
    const query = profileSearchQuery(socialQuery);
    if (!query) { updateSocialQuery(socialQuery); return; }
    const current = session;
    const ownerID = profile.id;
    const ticket = socialProfileSearchOperations.issue();
    socialQuery = query;
    socialSearchState = 'loading';
    socialLoadingMore = false;
    const result = await socialProfileSearchOwner.search(
      query,
      (requestedQuery, cursor) => requestSocialProfiles(current, requestedQuery, cursor, ticket),
    );
    if (!ticket.current() || session !== current || profile?.id !== ownerID) return;
    if (result.kind === 'loaded') {
      socialSearch = result.state;
      socialSearchState = result.state.items.length === 0 ? 'empty' : 'results';
    } else if (result.kind === 'failed') {
      socialSearchState = 'error';
    }
  }

  async function loadMoreSocialProfiles() {
    if (!session || !profile || primarySurface !== 'following' || !socialSearch.nextCursor || socialLoadingMore) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = socialProfileSearchOperations.issue();
    socialLoadingMore = true;
    const result = await socialProfileSearchOwner.loadMore(
      socialSearch,
      (query, cursor) => requestSocialProfiles(current, query, cursor, ticket),
    );
    if (!ticket.current() || session !== current || profile?.id !== ownerID) return;
    socialLoadingMore = false;
    if (result.kind === 'loaded') {
      socialSearch = result.state;
      socialSearchState = result.state.items.length === 0 ? 'empty' : 'results';
    } else if (result.kind === 'failed') {
      socialSearchState = 'error';
    }
  }

  async function openSocialProfile(userID: string) {
    const username = socialSearch.items.find(({ userId }) => userId === userID)?.username;
    if (!username) return;
    await openSocialProfileByUsername(username);
  }

  async function openSocialProfileByUsername(username: string) {
    if (!session || !profile || primarySurface !== 'following') return;
    const current = session;
    const ownerID = profile.id;
    const ticket = socialProfileDetailOperations.issue();
    selectedSocialProfile = null;
    selectedSocialUsername = username;
    socialProfileState = 'loading';
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).profileByUsername(username),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const publicProfile = publicProfileFromAPI(await response.json());
      if (!ticket.current() || session !== current || profile?.id !== ownerID || selectedSocialUsername !== username) return;
      selectedSocialProfile = publicProfile;
      selectedSocialUsername = publicProfile.username;
      socialProfileState = 'ready';
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current() && session === current && profile?.id === ownerID && selectedSocialUsername === username) {
        socialProfileState = 'error';
      }
    }
  }

  function closeSocialProfile() {
    socialProfileDetailOperations.invalidate();
    selectedSocialProfile = null;
    selectedSocialUsername = '';
    socialProfileState = 'idle';
  }

  async function mutateSocialRelationship(action: 'follow' | 'cancel-request' | 'unfollow') {
    if (!session || !profile || !selectedSocialProfile || socialRelationshipBusy) return;
    const current = session;
    const ownerID = profile.id;
    const username = selectedSocialProfile.username;
    const ticket = socialProfileDetailOperations.issue();
    socialRelationshipBusy = true;
    socialRelationshipError = false;
    try {
      const api = createSessionApiClient(data.config.apiURL, () => current.token);
      const idempotencyKey = crypto.randomUUID();
      const result = action === 'follow'
        ? await api.followProfile(username, idempotencyKey)
        : action === 'cancel-request'
          ? await api.cancelFollowRequest(username, idempotencyKey)
          : await api.unfollowProfile(username, idempotencyKey);
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const mutation = relationshipMutationResultFromAPI(await response.json());
      if (!ticket.current() || session !== current || profile?.id !== ownerID || mutation.profile.username.toLowerCase() !== username.toLowerCase()) return;
      selectedSocialProfile = mutation.profile;
      socialSearch = { ...socialSearch, items: socialSearch.items.map((item) => item.userId === mutation.profile.userId ? mutation.profile : item) };
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current()) socialRelationshipError = true;
    } finally {
      if (ticket.current()) socialRelationshipBusy = false;
    }
  }

  async function loadSocialFollowRequests(cursor = '') {
    if (!session || !profile || socialFollowRequestBusyID) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = socialFollowRequestOperations.issue();
    socialFollowRequestsOpen = true;
    socialFollowRequestsState = cursor ? 'ready' : 'loading';
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).followRequests(cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const page = followRequestPageFromAPI(await response.json());
      const next = cursor ? mergeFollowRequestPage(socialFollowRequests, page, cursor) : page;
      if (!ticket.current() || session !== current || profile?.id !== ownerID) return;
      socialFollowRequests = next;
      socialFollowRequestsState = next.items.length === 0 ? 'empty' : 'ready';
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current()) socialFollowRequestsState = 'error';
    }
  }

  async function reviewSocialFollowRequest(decision: 'accept' | 'reject', requestID: string) {
    if (!session || !profile || socialFollowRequestBusyID || !socialFollowRequests.items.some(({ id }) => id === requestID)) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = socialFollowRequestOperations.issue();
    socialFollowRequestBusyID = requestID;
    try {
      const api = createSessionApiClient(data.config.apiURL, () => current.token);
      const key = crypto.randomUUID();
      const result = decision === 'accept'
        ? await api.acceptFollowRequest(requestID, key)
        : await api.rejectFollowRequest(requestID, key);
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const reviewed = followRequestReviewResultFromAPI(await response.json());
      if (reviewed.request.id !== requestID || reviewed.decision !== (decision === 'accept' ? 'accepted' : 'rejected')) throw new Error('mismatched_follow_request_review');
      if (!ticket.current() || session !== current || profile?.id !== ownerID) return;
      socialFollowRequests = removeResolvedFollowRequest(socialFollowRequests, requestID);
      socialFollowRequestsState = socialFollowRequests.items.length === 0 ? 'empty' : 'ready';
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current()) socialFollowRequestsState = 'error';
    } finally {
      if (ticket.current()) socialFollowRequestBusyID = '';
    }
  }

  async function reviewSocialBlock(username: string) {
    if (!session || !profile || socialBlockBusy || selectedSocialProfile?.username !== username) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = socialBlockOperations.issue();
    socialBlockBusy = true;
    socialBlockError = false;
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).reviewProfileBlock(username),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const review = blockReviewFromAPI(await response.json());
      if (!ticket.current() || session !== current || profile?.id !== ownerID || selectedSocialProfile?.userId !== review.target.userId) return;
      socialBlockReview = review;
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current() && session === current && profile?.id === ownerID) socialBlockError = true;
    } finally {
      if (ticket.current()) socialBlockBusy = false;
    }
  }

  function cancelSocialBlock() {
    if (socialBlockBusy) return;
    socialBlockOperations.invalidate();
    socialBlockReview = null;
    socialBlockError = false;
  }

  async function confirmSocialBlock() {
    if (!session || !profile || !socialBlockReview || socialBlockBusy) return;
    const current = session;
    const ownerID = profile.id;
    const review = socialBlockReview;
    const ticket = socialBlockOperations.issue();
    socialBlockBusy = true;
    socialBlockError = false;
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token)
          .blockProfile(review.target.username, crypto.randomUUID(), {
            version: review.acknowledgement.version,
            token: review.acknowledgement.token,
            expiresAt: review.acknowledgement.expiresAt,
          }),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const result = blockResultFromAPI(await response.json());
      if (!ticket.current() || session !== current || profile?.id !== ownerID || result.target.userId !== review.target.userId) return;
      socialSearch = { ...socialSearch, items: socialSearch.items.filter(({ userId }) => userId !== result.target.userId) };
      socialBlockReview = null;
      closeSocialProfile();
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current() && session === current && profile?.id === ownerID) socialBlockError = true;
    } finally {
      if (ticket.current()) socialBlockBusy = false;
    }
  }

  async function loadBlockedAccounts(cursor = '') {
    if (!session || !profile || socialBlockBusy) return;
    const current = session;
    const ownerID = profile.id;
    const ticket = socialBlockedAccountOperations.issue();
    socialBlockedAccountsOpen = true;
    socialBlockedAccountsState = cursor ? 'ready' : 'loading';
    socialBlockError = false;
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token).blockedAccounts(cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const page = blockedAccountPageFromAPI(await response.json());
      const next = cursor ? mergeBlockedAccountPage(socialBlockedAccounts, page, cursor) : page;
      if (!ticket.current() || session !== current || profile?.id !== ownerID) return;
      socialBlockedAccounts = next;
      socialBlockedAccountsState = next.items.length === 0 ? 'empty' : 'ready';
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current() && session === current && profile?.id === ownerID) socialBlockedAccountsState = 'error';
    }
  }

  function closeBlockedAccounts() {
    socialBlockedAccountOperations.invalidate();
    socialBlockedAccountsOpen = false;
    socialUnblockReview = null;
    socialBlockError = false;
  }

  function reviewSocialUnblock(username: string) {
    if (socialBlockBusy) return;
    socialUnblockReview = socialBlockedAccounts.items.find(({ identity }) => identity.username === username) ?? null;
    socialBlockError = false;
  }

  function cancelSocialUnblock() {
    if (socialBlockBusy) return;
    socialUnblockReview = null;
    socialBlockError = false;
  }

  async function confirmSocialUnblock() {
    if (!session || !profile || !socialUnblockReview || socialBlockBusy) return;
    const current = session;
    const ownerID = profile.id;
    const review = socialUnblockReview;
    const ticket = socialBlockedAccountOperations.issue();
    socialBlockBusy = true;
    socialBlockError = false;
    try {
      const response = generatedResponse(
        await createSessionApiClient(data.config.apiURL, () => current.token)
          .unblockAccount(review.identity.userId, crypto.randomUUID()),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const result = unblockResultFromAPI(await response.json());
      if (!ticket.current() || session !== current || profile?.id !== ownerID || result.target.userId !== review.identity.userId) return;
      socialBlockedAccounts = {
        ...socialBlockedAccounts,
        items: socialBlockedAccounts.items.filter(({ identity }) => identity.userId !== result.target.userId),
      };
      socialBlockedAccountsState = socialBlockedAccounts.items.length === 0 ? 'empty' : 'ready';
      socialUnblockReview = null;
    } catch (cause) {
      handleFailure(cause, current, current.expiresAt, 'profile', ticket);
      if (ticket.current() && session === current && profile?.id === ownerID) socialBlockError = true;
    } finally {
      if (ticket.current()) socialBlockBusy = false;
    }
  }

  function closeManualActivity() {
    resetManualActivity();
  }
</script>

<main>
  <h1>{i18n.t('app.title')}</h1>
  {#if !ready}<p>{i18n.t('common.loading')}</p>
  {:else if accessState === 'authenticated_offline' && session}<p>{i18n.t('auth.offline')}</p><button onclick={openSignOutDialog}>{i18n.t('auth.signOut')}</button>
  {:else if profile && paths}
    <p>{i18n.t('auth.signedInAs', { name: profile.displayName || profile.email })}</p>
    <nav class="primary-tabs" aria-label={i18n.t('app.title')}>
      <button
        type="button"
        aria-current={primarySurface === 'home' ? 'page' : undefined}
        onclick={() => showPrimarySurface('home')}
      >{i18n.t('home.heading')}</button>
      <button
        type="button"
        aria-current={primarySurface === 'following' ? 'page' : undefined}
        onclick={() => showPrimarySurface('following')}
      >{i18n.t('social.following')}</button>
    </nav>
    {#if primarySurface === 'following'}
      <SocialProfileDiscovery
        {i18n}
        query={socialQuery}
        searchState={socialSearchState}
        results={socialSearch.items}
        nextCursor={socialSearch.nextCursor}
        loadingMore={socialLoadingMore}
        selectedProfile={selectedSocialProfile}
        profileState={socialProfileState}
        relationshipBusy={socialRelationshipBusy}
        relationshipError={socialRelationshipError}
        followRequestsOpen={socialFollowRequestsOpen}
        followRequestsState={socialFollowRequestsState}
        followRequests={socialFollowRequests.items}
        followRequestsNextCursor={socialFollowRequests.nextCursor}
        busyFollowRequestID={socialFollowRequestBusyID}
        blockReview={socialBlockReview}
        blockBusy={socialBlockBusy}
        blockError={socialBlockError}
        blockedAccountsOpen={socialBlockedAccountsOpen}
        blockedAccountsState={socialBlockedAccountsState}
        blockedAccounts={presentedSocialBlockedAccounts}
        blockedAccountsNextCursor={socialBlockedAccounts.nextCursor}
        unblockReview={presentedSocialUnblockReview}
        onQueryChange={updateSocialQuery}
        onSearch={() => void searchSocialProfiles()}
        onSelectProfile={(userID) => void openSocialProfile(userID)}
        onRetrySearch={() => void searchSocialProfiles()}
        onLoadMore={() => void loadMoreSocialProfiles()}
        onCloseProfile={closeSocialProfile}
        onRefreshProfile={() => void openSocialProfile(
          socialSearch.items.find(({ username }) => username === selectedSocialUsername)?.userId ?? '',
        )}
        onRelationshipAction={(action) => void mutateSocialRelationship(action)}
        onOpenFollowRequests={() => void loadSocialFollowRequests()}
        onCloseFollowRequests={() => { socialFollowRequestsOpen = false; }}
        onLoadMoreFollowRequests={() => void loadSocialFollowRequests(socialFollowRequests.nextCursor)}
        onReviewFollowRequest={(decision, requestID) => void reviewSocialFollowRequest(decision, requestID)}
        onReviewBlock={(username) => void reviewSocialBlock(username)}
        onCancelBlock={cancelSocialBlock}
        onConfirmBlock={() => void confirmSocialBlock()}
        onOpenBlockedAccounts={() => void loadBlockedAccounts()}
        onCloseBlockedAccounts={closeBlockedAccounts}
        onRetryBlockedAccounts={() => void loadBlockedAccounts()}
        onLoadMoreBlockedAccounts={() => void loadBlockedAccounts(socialBlockedAccounts.nextCursor)}
        onReviewUnblock={reviewSocialUnblock}
        onCancelUnblock={cancelSocialUnblock}
        onConfirmUnblock={() => void confirmSocialUnblock()}
      />
    {:else}
    <button
      aria-expanded={notificationsOpen}
      aria-controls="notification-history"
      onclick={toggleNotifications}
    >{i18n.t('notification.bellLabel')}{#if notificationUnreadCountAuthoritative} <span aria-live="polite">{i18n.t('notification.unreadCount', { count: notificationUnreadCount })}</span>{/if}</button>
    {#if notificationsOpen}
      <section id="notification-history" aria-labelledby="notification-heading">
        <h2 id="notification-heading">{i18n.t('notification.heading')}</h2>
        <button disabled={notificationMutationBusy} onclick={() => void markAllNotificationsRead()}>
          {i18n.t('notification.markAllRead')}
        </button>
        {#if notificationsBusy && notificationHistory.items.length === 0}
          <p>{i18n.t('notification.loading')}</p>
        {:else if notificationGroups.actionable.length === 0 && notificationGroups.informational.length === 0}
          <p>{i18n.t('notification.empty')}</p>
        {:else}
          <section aria-labelledby="actionable-notifications-heading">
            <h3 id="actionable-notifications-heading">{i18n.t('notification.actionableHeading')}</h3>
            <ol class="notification-list">
              {#each notificationGroups.actionable as notification (notification.id)}
                <li>
                  {#if !notification.read}<span>{i18n.t('notification.unread')}</span>{/if}
                  {#if notification.type === 'path_deleted' || notification.type === 'path_member_removed'}
                    <p>{i18n.t(notificationPresentationMessageKey(notification), {
                      displayName: notification.actor.displayName,
                      username: notification.actor.username,
                      pathName: 'pathName' in notification ? notification.pathName : '',
                      pathVisibility: 'pathVisibility' in notification ? pathVisibilityLabel(notification.pathVisibility) : '',
                    })}</p>
                  {:else}
                    <button class="notification-link" disabled={notificationMutationBusy} onclick={() => void openNotification(notification)}>
                      {i18n.t(notificationPresentationMessageKey(notification), {
                        displayName: notification.actor.displayName,
                        username: notification.actor.username,
                        pathName: 'pathName' in notification ? notification.pathName : '',
                        pathVisibility: 'pathVisibility' in notification ? pathVisibilityLabel(notification.pathVisibility) : '',
                      })}
                    </button>
                  {/if}
                  <button disabled={notificationMutationBusy} onclick={() => void deleteNotification(notification)}>
                    {i18n.t('notification.delete')}
                  </button>
                  <time datetime={notification.createdAt}>{i18n.date(new Date(notification.createdAt), { dateStyle: 'medium' })} {i18n.time(new Date(notification.createdAt), { timeStyle: 'short' })}</time>
                </li>
              {/each}
            </ol>
          </section>
          <section aria-labelledby="informational-notifications-heading">
            <h3 id="informational-notifications-heading">{i18n.t('notification.informationalHeading')}</h3>
            <ol class="notification-list">
              {#each notificationGroups.informational as notification (notification.id)}
                <li>
                  {#if !notification.read}<span>{i18n.t('notification.unread')}</span>{/if}
                  {#if notification.type === 'path_deleted' || notification.type === 'path_member_removed'}
                    <p>{i18n.t(notificationPresentationMessageKey(notification), {
                      displayName: notification.actor.displayName,
                      username: notification.actor.username,
                      pathName: 'pathName' in notification ? notification.pathName : '',
                      pathVisibility: 'pathVisibility' in notification ? pathVisibilityLabel(notification.pathVisibility) : '',
                    })}</p>
                  {:else}
                    <button class="notification-link" disabled={notificationMutationBusy} onclick={() => void openNotification(notification)}>
                      {i18n.t(notificationPresentationMessageKey(notification), {
                        displayName: notification.actor.displayName,
                        username: notification.actor.username,
                        pathName: 'pathName' in notification ? notification.pathName : '',
                        pathVisibility: 'pathVisibility' in notification ? pathVisibilityLabel(notification.pathVisibility) : '',
                      })}
                    </button>
                  {/if}
                  <button disabled={notificationMutationBusy} onclick={() => void deleteNotification(notification)}>
                    {i18n.t('notification.delete')}
                  </button>
                  <time datetime={notification.createdAt}>{i18n.date(new Date(notification.createdAt), { dateStyle: 'medium' })} {i18n.time(new Date(notification.createdAt), { timeStyle: 'short' })}</time>
                </li>
              {/each}
            </ol>
          </section>
        {/if}
        {#if notificationHistory.nextCursor}
          <button disabled={notificationsBusy} onclick={() => void loadNotifications(notificationHistory.nextCursor)}>
            {i18n.t(notificationsBusy ? 'notification.loadingMore' : 'notification.loadMore')}
          </button>
        {/if}
        {#if notificationErrorKey}
          <p role="alert">{i18n.t(notificationErrorKey)}</p>
          <button disabled={notificationsBusy} onclick={retryNotificationHistory}>{i18n.t('common.retry')}</button>
        {/if}
        {#if notificationMutationErrorKey}
          <p role="alert">{i18n.t(notificationMutationErrorKey)}</p>
          <button disabled={notificationMutationBusy} onclick={() => void retryNotificationMutation()}>{i18n.t('common.retry')}</button>
        {/if}
      </section>
    {/if}
    <section aria-labelledby="pending-invitations-heading">
      <h2 id="pending-invitations-heading">{i18n.t('pathInvitation.pendingHeading')}</h2>
      {#if pendingInvitations.items.length === 0}
        <p>{i18n.t('pathInvitation.pendingEmpty')}</p>
      {:else}
        <p>{i18n.t('pathInvitation.pendingCount', { count: pendingInvitations.items.length })}</p>
        <ul>
          {#each pendingInvitations.items as pending (pending.invitation.id)}
            {@const invitation = pending.invitation}
            <li id={invitation.id}>
              <p>{i18n.t('pathInvitation.pendingContext', {
                displayName: pending.inviter.displayName,
                username: pending.inviter.username,
                pathName: pending.pathName,
              })}</p>
              <p>{i18n.t(invitation.offeredRole === 'participant' ? 'pathInvitation.role.participant' : 'pathInvitation.role.supporter')}</p>
              <p>{i18n.t(invitation.offeredRole === 'participant' ? 'pathInvitation.role.participantEffect' : 'pathInvitation.role.supporterEffect')}</p>
              {#if pendingInvitationReview?.invitationId === invitation.id && pendingInvitationReview.kind === 'confirmation-required'}
                <div
                  role="alertdialog"
                  aria-labelledby="path-invitation-warning-heading-{invitation.id}"
                >
                  <h3 id="path-invitation-warning-heading-{invitation.id}">
                    {i18n.t('pathInvitation.visibilityWarning.heading')}
                  </h3>
                  <p>{i18n.t(
                    `pathInvitation.visibilityWarning.audience.${pendingInvitationReview.warning.pathVisibility}` as MessageKey,
                  )}</p>
                  <p>{i18n.t('pathInvitation.visibilityWarning.exposure')}</p>
                  <p>{i18n.t('pathInvitation.visibilityWarning.privacyScope')}</p>
                  {#if pendingInvitationReview.warning.hasRetainedActivity}
                    <p>{i18n.t('pathInvitation.visibilityWarning.retainedActivity')}</p>
                  {/if}
                  <button
                    use:focusPendingInvitationConfirm={invitation.id}
                    disabled={pendingInvitationBusy[invitation.id]}
                    onclick={() => confirmPendingInvitation(invitation.id)}
                  >{i18n.t('pathInvitation.visibilityWarning.confirm')}</button>
                  <button
                    disabled={pendingInvitationBusy[invitation.id]}
                    onclick={() => cancelPendingInvitationReview(invitation.id)}
                  >{i18n.t('pathInvitation.visibilityWarning.cancel')}</button>
                </div>
              {:else}
                <button use:focusPendingInvitationAccept={invitation.id} disabled={pendingInvitationBusy[invitation.id]} onclick={() => acceptPendingInvitation(invitation.id)}>{i18n.t(pendingInvitationBusy[invitation.id] ? 'pathInvitation.accepting' : 'pathInvitation.accept')}</button>
              {/if}
              {#if pendingInvitationErrors[invitation.id]}
                <p role="alert">{i18n.t(pendingInvitationErrors[invitation.id]!)}</p>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
      {#if pendingInvitations.nextCursor}<button disabled={pendingInvitationsBusy || pendingInvitationAcceptanceInProgress()} onclick={() => void loadPendingInvitations(pendingInvitations.nextCursor)}>{i18n.t('common.loadMore')}</button>{/if}
      {#if pendingInvitationsErrorKey}<p role="alert">{i18n.t(pendingInvitationsErrorKey)}</p>{/if}
      {#if acceptedInvitation}
        <p role="status">{i18n.t('pathInvitation.accepted', { role: i18n.t(acceptedInvitation.role === 'participant' ? 'pathInvitation.role.participant' : 'pathInvitation.role.supporter') })}</p>
        <p>{i18n.t(acceptedInvitation.role === 'participant' ? 'pathInvitation.participantTracking' : 'pathInvitation.supporterReadOnly')}</p>
      {/if}
    </section>
    {#if visiblePendingOwnershipTransfers.length > 0}
      <section class="ownership-card" aria-labelledby="pending-ownership-heading">
        <h2 id="pending-ownership-heading">{i18n.t('pathOwnership.pendingHeading')}</h2>
        <ul class="ownership-list">
          {#each visiblePendingOwnershipTransfers as transfer (transfer.id)}
            {@const path = [...paths, ...archivedPaths].find(({ id }) => id === transfer.pathId)}
            {@const viewerRole = ownershipTransferViewerRole(transfer)}
            {@const expiration = transfer.viewerTimeZone ? ownershipTransferExpiration(transfer.createdAt, transfer.expiresAt, transfer.viewerTimeZone, i18n) : undefined}
            <li class="ownership-row" tabindex="-1" use:focusPendingOwnershipTransfer={transfer.id}>
              <div class="ownership-row-copy">
                <strong>{path?.name ?? i18n.t('pathOwnership.pendingWith')}</strong>
                {#if transfer.counterpart}<span>{transfer.counterpart.displayName} (@{transfer.counterpart.username})</span>{/if}
                <span>{i18n.t(viewerRole === 'creator' ? 'pathOwnership.creatorPendingExplanation' : 'pathOwnership.recipientPendingExplanation')}</span>
                {#if expiration}<span>{expiration.summary}</span>{/if}
              </div>
              <div class="ownership-row-actions">
                {#if path?.archivedAt}
                  <span>{i18n.t('pathArchive.readOnly')}</span>
                {:else if viewerRole === 'creator'}
                  <button class="ownership-action destructive" disabled={ownershipMutationBusy} onclick={() => void mutateOwnershipTransfer(transfer, 'cancel')}>{i18n.t(ownershipMutationAction === 'cancel' && ownershipMutationTargetID === transfer.id ? 'pathOwnership.canceling' : 'pathOwnership.cancel')}</button>
                {:else}
                  <button class="ownership-action" disabled={ownershipMutationBusy} onclick={() => void mutateOwnershipTransfer(transfer, 'accept')}>{i18n.t(ownershipMutationAction === 'accept' && ownershipMutationTargetID === transfer.id ? 'pathOwnership.accepting' : 'pathOwnership.accept')}</button>
                  <button class="ownership-action destructive" disabled={ownershipMutationBusy} onclick={() => void mutateOwnershipTransfer(transfer, 'decline')}>{i18n.t(ownershipMutationAction === 'decline' && ownershipMutationTargetID === transfer.id ? 'pathOwnership.declining' : 'pathOwnership.decline')}</button>
                {/if}
              </div>
            </li>
          {/each}
        </ul>
        {#if ownershipMutationBusy}<p role="status">{i18n.t('common.loading')}</p>{/if}
        {#if ownershipErrorKey}<p role="alert">{i18n.t(ownershipErrorKey)}</p>{/if}
      </section>
    {/if}
    {#if selectedPath}
      {@const selectedCapabilities = effectivePathCapabilities(selectedPath)}
      {#if pathMembersOpen}
        <PathMemberAccess
          activities={pathMemberActivities}
          activitiesFailed={pathMemberActivitiesFailed}
          activitiesLoading={pathMemberActivitiesLoading}
          activitiesNextCursor={pathMemberActivitiesNextCursor}
          archived={Boolean(selectedPath.archivedAt)}
          failed={pathMembersFailed}
          i18n={i18n}
          loading={pathMembersLoading}
          loadingMore={pathMembersLoadingMore}
          members={pathMembers}
          nextCursor={pathMembersNextCursor}
          onBackToMembers={closePathMemberReview}
          onCancelRoleChange={cancelPathMemberRoleChange}
          onChooseRole={choosePathMemberRole}
          onConfirmRoleChange={() => void changeSelectedPathMemberRole()}
          onClose={() => closePathMembers(true)}
          onLoadMore={() => void openPathMembers(pathMembersNextCursor, false)}
          onLoadMoreActivities={() => { if (selectedPathMember) void loadPathMemberActivities(selectedPathMember, pathMemberActivitiesNextCursor); }}
          onOpenActivity={(activityID) => void openPathMemberActivity(activityID)}
          onRefresh={() => void openPathMembers()}
          onRemove={() => void removeSelectedPathMember()}
          onRetryReview={() => { if (selectedPathMember) void inspectPathMember(selectedPathMember); }}
          onSelect={(member) => void inspectPathMember(member)}
          onUnblock={() => { if (selectedPathMember) void unblockPathMember(selectedPathMember); }}
          pendingRole={pathMemberPendingRole}
          removalBusy={pathMemberRemovalBusy}
          removalError={pathMemberRemovalError}
          review={pathMemberRemovalReview}
          reviewLoading={pathMemberReviewLoading}
          roleChangeBusy={pathMemberRoleChangeBusy}
          roleChangeError={pathMemberRoleChangeError}
          selected={selectedPathMember}
          unblockBusy={pathMemberUnblockBusy}
          unblockError={pathMemberUnblockError}
        />
      {:else}
      <button disabled={pathLeaveInteractionBlocked} onclick={closePathDetails}>{i18n.t('pathDetails.back')}</button>
      <section aria-labelledby="path-details-heading">
        <h2 id="path-details-heading">{selectedPath.name}</h2>
        {#if selectedPath.archivedAt}<p>{i18n.t('pathArchive.readOnly')}</p>{/if}
        {#if !selectedCapabilities.trackTime}<PathMemberComparison
          failed={pathMembersFailed}
          i18n={i18n}
          loading={pathMembersLoading && pathMembers.length === 0}
          loadingMore={pathMembersLoadingMore}
          members={pathMembers}
          nextCursor={pathMembersNextCursor}
          onLoadMore={() => void openPathMembers(pathMembersNextCursor, false, false)}
          onRetry={() => void openPathMembers('', true, false)}
          onSelect={openComparedPathMember}
        />{/if}
        {#if selectedTimerState}
          <p>{i18n.t('path.progress.accumulated', { seconds: i18n.number(selectedTimerState.accumulatedSeconds) })}</p>
          {@const selectedIntervalProgress = intervalProgressPresentation(selectedTimerState.intervalProgress)}
          {#if selectedIntervalProgress}
            <div>
              <progress max={selectedIntervalProgress.targetSeconds} value={selectedIntervalProgress.visualSeconds} aria-label={intervalProgressMessage(selectedIntervalProgress)}></progress>
              <p>{intervalProgressMessage(selectedIntervalProgress)}</p>
            </div>
          {/if}
          {#if selectedPath.overallTarget}
            {@const selectedOverallProgress = overallProgress(timerStates[selectedPath.id]!.accumulatedSeconds, selectedPath.overallTarget)}
            {#if selectedOverallProgress}
              <div>
                <progress max={selectedOverallProgress.targetSeconds} value={selectedOverallProgress.visualSeconds} aria-label={overallProgressMessage(selectedOverallProgress)}></progress>
                <p>{overallProgressMessage(selectedOverallProgress)}</p>
              </div>
            {/if}
          {/if}
        {/if}
        {#if selectedPath.intervalGoal}<p>{i18n.t('path.goal.intervalSummary', { seconds: i18n.number(selectedPath.intervalGoal.targetSeconds), recurrence: recurrenceLabel(selectedPath.intervalGoal.recurrence) })}</p>{/if}
        {#if selectedCapabilities.manageGoals || selectedCapabilities.renamePath || selectedCapabilities.manageLifecycle || selectedCapabilities.manageVisibility}
          <button disabled={managingPathGoals || manualBusy || deleteBusy || pathDeletionBusy || timerBusy[selectedPath.id] || pathLeaveInteractionBlocked || pathDetailBusy || archiveBusy} onclick={openPathManagement}>{i18n.t('pathManage.action')}</button>
        {/if}
        {#if selectedCapabilities.trackTime}
          <button disabled={renamingPath || managingPathGoals || manualBusy || pathLeaveInteractionBlocked || pathDetailBusy || activityPageBusy || revisionPageBusy || archiveBusy} onclick={() => void openManualActivity(selectedPath!.id)}>{i18n.t('activity.add')}</button>
          <PathMemberComparison
            failed={pathMembersFailed}
            i18n={i18n}
            loading={pathMembersLoading && pathMembers.length === 0}
            loadingMore={pathMembersLoadingMore}
            members={pathMembers}
            nextCursor={pathMembersNextCursor}
            onLoadMore={() => void openPathMembers(pathMembersNextCursor, false, false)}
            onRetry={() => void openPathMembers('', true, false)}
            onSelect={openComparedPathMember}
          />
        {/if}
        {#if selectedCapabilities.inviteMembers}
          <button disabled={invitationReviewBusy || invitationSendBusy || pathLeaveInteractionBlocked} onclick={openPathSharing}>{i18n.t('pathInvitation.share')}</button>
        {/if}
        <button disabled={pathLeaveInteractionBlocked || pathMemberRemovalBusy || pathMemberUnblockBusy} onclick={() => void openPathMembers()}>{i18n.t('pathMembers.heading')}</button>
        {#if selectedCapabilities.transferOwnership}
          <button disabled={ownershipMutationBusy || ownershipCandidatesBusy || pathLeaveInteractionBlocked} onclick={openOwnershipTransfer}>{i18n.t('pathOwnership.heading')}</button>
        {/if}
        {#if selectedCapabilities.leavePath}
          <button class="destructive-action" disabled={pathLeaveInteractionBlocked || manualBusy || timerBusy[selectedPath.id]} onclick={beginPathLeaveReview}>{i18n.t('pathLeave.action')}</button>
          {#if pathLeaveReview}
            <dialog
              use:showModal
              role="alertdialog"
              aria-busy={pathLeaveBusy}
              aria-labelledby="path-leave-heading"
              aria-describedby="path-leave-warning path-leave-choice-description"
              oncancel={(event) => { event.preventDefault(); cancelPathLeave(); }}
            >
              <h3 id="path-leave-heading">{i18n.t('pathLeave.heading', { pathName: pathLeaveReview.pathName })}</h3>
              <p id="path-leave-warning">{i18n.t(selectedCapabilities.trackTime ? 'pathLeave.warning' : 'pathLeave.supporterWarning')}</p>
              {#if selectedCapabilities.trackTime}
                <fieldset disabled={pathLeaveBusy}>
                  <legend>{i18n.t('pathLeave.choicePrompt')}</legend>
                  <label class="leave-choice">
                    <input type="radio" name="path-leave-retention" checked={pathLeaveReview.retainActivity} onchange={() => choosePathLeaveRetention(true)} />
                    <span><strong>{i18n.t('pathLeave.keepActivity')}</strong><small>{i18n.t('pathLeave.keepActivityDescription')}</small></span>
                  </label>
                  <label class="leave-choice">
                    <input type="radio" name="path-leave-retention" checked={!pathLeaveReview.retainActivity} onchange={() => choosePathLeaveRetention(false)} />
                    <span><strong>{i18n.t('pathLeave.deleteActivity')}</strong><small>{i18n.t('pathLeave.deleteActivityDescription')}</small></span>
                  </label>
                </fieldset>
              {/if}
              <p id="path-leave-choice-description" class:destructive-copy={!pathLeaveReview.retainActivity}>
                {i18n.t(pathLeaveReview.retainActivity ? 'pathLeave.keepActivityDescription' : 'pathLeave.deleteWarning')}
              </p>
              {#if pathLeaveErrorKey}<p role="alert">{i18n.t(pathLeaveErrorKey)}</p>{/if}
              <div class="dialog-actions">
                <button disabled={pathLeaveBusy} onclick={cancelPathLeave}>{i18n.t('common.cancel')}</button>
                <button class:destructive-action={!pathLeaveReview.retainActivity} disabled={pathLeaveBusy} onclick={() => void confirmPathLeave()}>{i18n.t(pathLeaveBusy ? 'pathLeave.leaving' : pathLeaveReview.retainActivity ? 'pathLeave.confirm' : 'pathLeave.confirmDelete')}</button>
              </div>
            </dialog>
          {/if}
        {/if}
        <button disabled={pathLeaveInteractionBlocked || pathDetailBusy || activityPageBusy || revisionPageBusy} onclick={() => void openPathHistory()}>{i18n.t('pathDetails.openHistory')}</button>
        {#if selectedCapabilities.manageLifecycle}
          <button disabled={archiveBusy || manualBusy || deleteBusy || pathLeaveInteractionBlocked || pathDetailBusy} onclick={reviewArchiveChange}>{i18n.t(selectedPath.archivedAt ? 'pathArchive.unarchiveAction' : 'pathArchive.action')}</button>
          {#if archiveReview}
            <section aria-labelledby="path-archive-heading">
              <h3 id="path-archive-heading">{i18n.t('pathArchive.heading')}</h3>
              <p>{i18n.t(archiveReview.archived ? 'pathArchive.warning' : 'pathArchive.unarchiveWarning')}</p>
              <button disabled={archiveBusy} onclick={() => void confirmArchiveChange()}>{i18n.t(archiveReview.archived ? 'pathArchive.confirm' : 'pathArchive.confirmUnarchive')}</button>
              <button disabled={archiveBusy} onclick={cancelArchiveChange}>{i18n.t('common.cancel')}</button>
            </section>
          {/if}
          {#if archiveErrorKey}<p role="alert">{i18n.t(archiveErrorKey)}</p>{/if}
          {#if archiveStatusKey}<p role="status">{i18n.t(archiveStatusKey)}</p>{/if}
        {/if}
        {#if selectedCapabilities.manageGoals && goalUpdateSaved}<p role="status">{i18n.t('pathManage.saved')}</p>{/if}
        {#if (selectedCapabilities.manageGoals || selectedCapabilities.renamePath || selectedCapabilities.manageLifecycle || selectedCapabilities.manageVisibility) && managingPathGoals && goalForm}
          <section aria-labelledby="path-manage-heading">
            <h3 id="path-manage-heading">{i18n.t('pathManage.heading')}</h3>
            {#if selectedCapabilities.renamePath && renamingPath}
              <section aria-labelledby="path-rename-heading">
                <h4 id="path-rename-heading">{i18n.t('pathRename.heading')}</h4>
                <p>{i18n.t('pathRename.explanation')}</p>
                <label for="path-rename-name">{i18n.t('pathRename.nameLabel')}</label>
                <input
                  id="path-rename-name"
                  type="text"
                  maxlength="100"
                  autocomplete="off"
                  disabled={pathRenameBusy}
                  bind:value={pathRenameDraft}
                />
                <button disabled={pathRenameBusy} onclick={() => void submitPathRename()}>
                  {i18n.t(pathRenameBusy ? 'pathRename.saving' : 'pathRename.save')}
                </button>
                {#if pathRenameErrorKey}<p role="alert">{i18n.t(pathRenameErrorKey)}</p>{/if}
                {#if pathRenameStatusName}<p role="status">{i18n.t('pathRename.saved', { name: pathRenameStatusName })}</p>{/if}
              </section>
            {/if}
            {#if selectedCapabilities.manageVisibility && profile}
              <section class="path-visibility" aria-labelledby="path-visibility-heading">
                <h4 id="path-visibility-heading">{i18n.t('pathVisibility.heading')}</h4>
                <p>{i18n.t('pathVisibility.current', {
                  visibility: pathVisibilityLabel(selectedPath.visibility),
                })}</p>
                <fieldset disabled={pathVisibilityBusy}>
                  <legend>{i18n.t('pathVisibility.choiceLabel')}</legend>
                  {#each pathVisibilityOptions(profile.profileVisibility) as visibility}
                    <label class="visibility-choice">
                      <input type="radio" name="path-visibility" value={visibility} bind:group={pathVisibilityDraft} />
                      <span>{i18n.t(`pathVisibility.option.${visibility}`)}</span>
                    </label>
                  {/each}
                </fieldset>
                <button disabled={pathVisibilityBusy || pathVisibilityDraft === selectedPath.visibility} onclick={reviewPathVisibility}>
                  {i18n.t(pathVisibilityBusy ? 'pathVisibility.saving' : 'pathVisibility.save')}
                </button>
                {#if pathVisibilityErrorKey}<p role="alert">{i18n.t(pathVisibilityErrorKey)}</p>{/if}
                {#if pathVisibilitySaved}<p role="status">{i18n.t('pathVisibility.saved')}</p>{/if}
                {#if pathVisibilityReview?.broader}
                  <dialog
                    use:showModal
                    role="alertdialog"
                    aria-busy={pathVisibilityBusy}
                    aria-labelledby="path-visibility-confirmation-heading"
                    aria-describedby="path-visibility-transition path-visibility-history path-visibility-scope"
                    oncancel={(event) => { event.preventDefault(); cancelPathVisibilityConfirmation(); }}
                  >
                    <h4 id="path-visibility-confirmation-heading">{i18n.t('pathVisibility.confirmation.heading')}</h4>
                    <p id="path-visibility-transition">{i18n.t('pathVisibility.confirmation.transition', {
                      pathName: pathVisibilityReview.pathName,
                      current: i18n.t(`pathVisibility.option.${pathVisibilityReview.current}`),
                      proposed: i18n.t(`pathVisibility.option.${pathVisibilityReview.proposed}`),
                    })}</p>
                    <p id="path-visibility-history">{i18n.t('pathVisibility.confirmation.historyExposure')}</p>
                    <p id="path-visibility-scope">{i18n.t('pathVisibility.confirmation.unchangedScope')}</p>
                    {#if pathVisibilityErrorKey}<p role="alert">{i18n.t(pathVisibilityErrorKey)}</p>{/if}
                    <div class="dialog-actions">
                      <button disabled={pathVisibilityBusy} onclick={cancelPathVisibilityConfirmation}>{i18n.t('common.cancel')}</button>
                      <button disabled={pathVisibilityBusy} onclick={() => void confirmPathVisibility()}>{i18n.t(pathVisibilityBusy ? 'pathVisibility.saving' : 'pathVisibility.confirmation.confirm')}</button>
                    </div>
                  </dialog>
                {/if}
              </section>
            {/if}
            {#if selectedCapabilities.manageGoals}
            {#if !goalReview}
              <fieldset disabled={goalUpdateBusy}>
                <legend>{i18n.t('pathCreate.intervalHeading')}</legend>
                <label><input type="checkbox" bind:checked={goalForm.intervalEnabled} /> {i18n.t('pathCreate.intervalEnabled')}</label>
                {#if goalForm.intervalEnabled}
                  <div class="duration-fields">
                    <label for="manage-interval-hours">{i18n.t('pathCreate.duration.hours')}</label>
                    <input id="manage-interval-hours" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.intervalDuration.hours} />
                    <label for="manage-interval-minutes">{i18n.t('pathCreate.duration.minutes')}</label>
                    <input id="manage-interval-minutes" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.intervalDuration.minutes} />
                    <label for="manage-interval-seconds">{i18n.t('pathCreate.duration.seconds')}</label>
                    <input id="manage-interval-seconds" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.intervalDuration.seconds} />
                  </div>
                  <label for="manage-interval-recurrence">{i18n.t('pathCreate.recurrence')}</label>
                  <select id="manage-interval-recurrence" bind:value={goalForm.recurrence}>
                    <option value="hourly">{i18n.t('pathCreate.recurrence.hourly')}</option>
                    <option value="daily">{i18n.t('pathCreate.recurrence.daily')}</option>
                    <option value="weekly">{i18n.t('pathCreate.recurrence.weekly')}</option>
                    <option value="monthly">{i18n.t('pathCreate.recurrence.monthly')}</option>
                    <option value="yearly">{i18n.t('pathCreate.recurrence.yearly')}</option>
                  </select>
                  {#if goalForm.recurrence === 'yearly'}
                    <label for="manage-yearly-month">{i18n.t('pathCreate.alignment.month')}</label>
                    <select id="manage-yearly-month" bind:value={goalForm.yearlyMonth}>{#each calendarMonths as month}<option value={String(month)}>{monthName(month)}</option>{/each}</select>
                    <label for="manage-yearly-day">{i18n.t('pathCreate.alignment.day')}</label>
                    <input id="manage-yearly-day" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.yearlyDay} />
                  {:else if goalForm.recurrence === 'weekly'}
                    <label for="manage-alignment-value">{i18n.t('pathCreate.alignment.weekday')}</label>
                    <select id="manage-alignment-value" bind:value={goalForm.alignmentValue}><option value="">{i18n.t('pathCreate.alignment.chooseWeekday')}</option>{#each isoWeekdays as weekday}<option value={String(weekday)}>{weekdayName(weekday)}</option>{/each}</select>
                  {:else}
                    <label for="manage-alignment-value">{alignmentInputLabel(goalForm.recurrence)}</label>
                    <input id="manage-alignment-value" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.alignmentValue} />
                  {/if}
                {/if}
              </fieldset>
              <fieldset disabled={goalUpdateBusy}>
                <legend>{i18n.t('pathCreate.overallHeading')}</legend>
                <label><input type="checkbox" bind:checked={goalForm.overallEnabled} /> {i18n.t('pathCreate.overallEnabled')}</label>
                {#if goalForm.overallEnabled}
                  <div class="duration-fields">
                    <label for="manage-overall-hours">{i18n.t('pathCreate.duration.hours')}</label>
                    <input id="manage-overall-hours" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.overallDuration.hours} />
                    <label for="manage-overall-minutes">{i18n.t('pathCreate.duration.minutes')}</label>
                    <input id="manage-overall-minutes" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.overallDuration.minutes} />
                    <label for="manage-overall-seconds">{i18n.t('pathCreate.duration.seconds')}</label>
                    <input id="manage-overall-seconds" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={goalForm.overallDuration.seconds} />
                  </div>
                {/if}
              </fieldset>
              <button disabled={goalUpdateBusy} onclick={reviewGoalChanges}>{i18n.t('pathManage.review')}</button>
            {/if}
            {#if goalReview}
              <div class="goal-comparison">
                <section>
                  <h4>{i18n.t('pathManage.current')}</h4>
                  <p>{goalReview.current.intervalGoal ? intervalGoalSummary(goalReview.current.intervalGoal) : i18n.t('pathManage.none')}</p>
                  <p>{goalReview.current.overallTarget ? i18n.t('path.goal.overallSummary', { seconds: i18n.number(goalReview.current.overallTarget.targetSeconds) }) : i18n.t('pathManage.none')}</p>
                </section>
                <section>
                  <h4>{i18n.t('pathManage.proposed')}</h4>
                  <p>{goalReview.proposed.intervalGoal ? intervalGoalSummary(goalReview.proposed.intervalGoal) : i18n.t('pathManage.none')}</p>
                  <p>{goalReview.proposed.overallTarget ? i18n.t('path.goal.overallSummary', { seconds: i18n.number(goalReview.proposed.overallTarget.targetSeconds) }) : i18n.t('pathManage.none')}</p>
                </section>
              </div>
              {#if goalReview.changed}<p>{i18n.t('pathManage.changed')}</p>{/if}
              <p role="note">{i18n.t('pathManage.goalWarning')}</p>
              <button disabled={goalUpdateBusy} onclick={cancelGoalChanges}>{i18n.t('common.cancel')}</button>
              <button disabled={goalUpdateBusy || sessionOperationBusy || !goalReview.changed} onclick={() => void confirmGoalChanges()}>{i18n.t(goalUpdateBusy ? 'pathManage.confirming' : 'pathManage.confirm')}</button>
            {/if}
            {#if !goalReview}<button disabled={goalUpdateBusy} onclick={cancelGoalChanges}>{i18n.t('common.cancel')}</button>{/if}
            {#if goalUpdateErrorKey}<p role="alert">{i18n.t(goalUpdateErrorKey)}</p>{/if}
            {:else}
              <button disabled={pathRenameBusy || pathDeletionBusy} onclick={cancelGoalChanges}>{i18n.t('common.done')}</button>
            {/if}
          </section>
        {/if}
        {#if selectedCapabilities.manageLifecycle && managingPathGoals}
          <section aria-labelledby="path-delete-heading">
            {#if !pathDeletionReview}
              <button class="danger" disabled={pathDeletionBusy} onclick={beginPathDeletionReview}>{i18n.t('pathDelete.action')}</button>
            {:else}
              <div role="alertdialog" aria-labelledby="path-delete-heading" aria-describedby="path-delete-warning">
                <h3 id="path-delete-heading">{i18n.t('pathDelete.heading', { pathName: pathDeletionReview.expectedName })}</h3>
                <p id="path-delete-warning">{i18n.t('pathDelete.warning')}</p>
                <p>{i18n.t('pathDelete.timerWarning')}</p>
                <p>{i18n.t('pathDelete.archiveAlternative')}</p>
                {#if pathDeletionErrorKey}<p role="alert">{i18n.t(pathDeletionErrorKey)}</p>{/if}
                <button class="danger" disabled={pathDeletionBusy} onclick={() => void confirmPathDeletion()}>{i18n.t(pathDeletionBusy ? 'pathDelete.deleting' : 'pathDelete.confirm')}</button>
                <button disabled={pathDeletionBusy} onclick={cancelPathDeletion}>{i18n.t('common.cancel')}</button>
              </div>
            {/if}
          </section>
        {/if}
        {#if sharingPath && selectedCapabilities.inviteMembers}
          <section aria-labelledby="path-invitation-heading">
            <h3 id="path-invitation-heading">{i18n.t('pathInvitation.heading')}</h3>
            <label for="invitation-username">{i18n.t('pathInvitation.usernameLabel')}</label>
            <input id="invitation-username" value={invitationUsername} disabled={invitationReviewBusy || invitationSendBusy} oninput={(event) => changeInvitationUsername(event.currentTarget.value)} />
            <p>{i18n.t('pathInvitation.usernameHint')}</p>
            <button disabled={invitationReviewBusy || invitationSendBusy} onclick={() => void reviewInvitationRecipient()}>{i18n.t(invitationReviewBusy ? 'pathInvitation.reviewing' : 'pathInvitation.review')}</button>
            {#if invitationReview}
              <p>{i18n.t('pathInvitation.reviewedIdentity', { displayName: invitationReview.recipient.displayName, username: invitationReview.recipient.username })}</p>
              <fieldset disabled={invitationSendBusy}>
                <legend>{i18n.t('pathInvitation.roleLabel')}</legend>
                <button aria-pressed={invitationRole === 'participant'} onclick={() => chooseInvitationRole('participant')}>{i18n.t('pathInvitation.role.participant')}</button>
                <p>{i18n.t('pathInvitation.role.participantEffect')}</p>
                <button aria-pressed={invitationRole === 'supporter'} onclick={() => chooseInvitationRole('supporter')}>{i18n.t('pathInvitation.role.supporter')}</button>
                <p>{i18n.t('pathInvitation.role.supporterEffect')}</p>
              </fieldset>
              <h4>{i18n.t('pathInvitation.confirmHeading')}</h4>
              <p>{i18n.t('pathInvitation.confirmSend', {
                displayName: invitationReview.recipient.displayName,
                username: invitationReview.recipient.username,
                role: i18n.t(invitationRole === 'participant' ? 'pathInvitation.role.participant' : 'pathInvitation.role.supporter'),
              })}</p>
              <button disabled={invitationSendBusy} onclick={() => void sendReviewedInvitation()}>{i18n.t(invitationSendBusy ? 'pathInvitation.sending' : 'pathInvitation.send')}</button>
            {/if}
            <button disabled={invitationReviewBusy || invitationSendBusy} onclick={cancelPathSharing}>{i18n.t('common.cancel')}</button>
            {#if invitationSent && invitationReview}<p role="status">{i18n.t('pathInvitation.sent', { displayName: invitationReview.recipient.displayName, username: invitationReview.recipient.username })}</p>{/if}
            {#if invitationErrorKey}<p role="alert">{i18n.t(invitationErrorKey)}</p>{/if}
            <section class="managed-invitations" aria-labelledby="managed-invitations-heading" aria-busy={managedInvitationsLoading || managedInvitationsLoadingMore}>
              <h4 id="managed-invitations-heading">{i18n.t('pathInvitation.managed.heading')}</h4>
              {#if managedInvitations.items.length > 0}
                <p>{i18n.t('pathInvitation.managed.count', { count: managedInvitations.items.length })}</p>
                <ul class="managed-invitation-list">
                  {#each managedInvitations.items as managed (managed.invitation.id)}
                    <li class="managed-invitation-row">
                      <div class="managed-invitation-copy">
                        <strong>{i18n.t('pathInvitation.managed.recipient', { displayName: managed.recipient.displayName, username: managed.recipient.username })}</strong>
                        <span>{i18n.t('pathInvitation.managed.role', { role: i18n.t(managed.invitation.offeredRole === 'participant' ? 'pathInvitation.role.participant' : 'pathInvitation.role.supporter') })}</span>
                        <span>{i18n.t('pathInvitation.managed.inviter', { displayName: managed.inviter.displayName, username: managed.inviter.username })}</span>
                        <time datetime={managed.invitation.createdAt}>{i18n.t('pathInvitation.managed.sentAt', {
                          date: i18n.date(new Date(managed.invitation.createdAt), { dateStyle: 'medium' }),
                          time: i18n.time(new Date(managed.invitation.createdAt), { timeStyle: 'short' }),
                        })}</time>
                      </div>
                      <button
                        aria-label={i18n.t('pathInvitation.managed.cancelFor', {
                          displayName: managed.recipient.displayName,
                          role: i18n.t(`pathInvitation.role.${managed.invitation.offeredRole}`),
                          username: managed.recipient.username,
                        })}
                        class="destructive-action"
                        disabled={managedInvitationCancelBusy}
                        onclick={() => beginManagedInvitationCancellation(managed)}
                      >{i18n.t('pathInvitation.managed.cancel')}</button>
                    </li>
                  {/each}
                </ul>
              {:else if managedInvitationsLoading}
                <p role="status">{i18n.t('pathInvitation.managed.loading')}</p>
              {:else if !managedInvitationsFailed}
                <h5>{i18n.t('pathInvitation.managed.emptyHeading')}</h5>
                <p>{i18n.t('pathInvitation.managed.emptyDescription')}</p>
              {/if}
              {#if managedInvitationsFailed}
                <div role="alert">
                  <strong>{i18n.t('pathInvitation.managed.unavailableHeading')}</strong>
                  <button onclick={() => {
                    if (session && profile && selectedPath) void loadManagedPathInvitations(
                      managedInvitationRetryCursor,
                      managedInvitationRetryCursor === '',
                      session,
                      profile.id,
                      selectedPath.id,
                    );
                  }}>{i18n.t('common.retry')}</button>
                </div>
              {/if}
              {#if managedInvitationsLoadingMore}
                <p role="status">{i18n.t('pathInvitation.managed.loadingMore')}</p>
              {:else if managedInvitations.nextCursor}
                <button onclick={() => {
                  if (session && profile && selectedPath) void loadManagedPathInvitations(
                    managedInvitations.nextCursor,
                    false,
                    session,
                    profile.id,
                    selectedPath.id,
                  );
                }}>{i18n.t('common.loadMore')}</button>
              {/if}
            </section>
            {#if managedInvitationCancelReview}
              <dialog
                use:showModal
                role="alertdialog"
                aria-busy={managedInvitationCancelBusy}
                aria-labelledby="managed-invitation-cancel-heading"
                aria-describedby="managed-invitation-cancel-description"
                oncancel={(event) => { event.preventDefault(); cancelManagedInvitationCancellation(); }}
              >
                <h4 id="managed-invitation-cancel-heading">{i18n.t('pathInvitation.managed.cancelConfirmationHeading')}</h4>
                <p id="managed-invitation-cancel-description">{i18n.t('pathInvitation.managed.cancelConfirmationBody', {
                  displayName: managedInvitationCancelReview.recipient.displayName,
                  role: i18n.t(`pathInvitation.role.${managedInvitationCancelReview.invitation.offeredRole}`),
                  username: managedInvitationCancelReview.recipient.username,
                })}</p>
                {#if managedInvitationCancelFailed}<p role="alert">{i18n.t('pathInvitation.managed.unavailableHeading')}</p>{/if}
                <div class="dialog-actions">
                  <button class="destructive-action" disabled={managedInvitationCancelBusy} onclick={() => void confirmManagedInvitationCancellation()}>{i18n.t(managedInvitationCancelBusy ? 'pathInvitation.managed.canceling' : 'pathInvitation.managed.cancel')}</button>
                  <button disabled={managedInvitationCancelBusy} onclick={cancelManagedInvitationCancellation}>{i18n.t('common.cancel')}</button>
                </div>
              </dialog>
            {/if}
          </section>
        {/if}
        {#if ownershipOpen && selectedCapabilities.transferOwnership}
          <section class="ownership-card" aria-labelledby="path-ownership-heading">
            <h3 id="path-ownership-heading">{i18n.t(ownershipReviewing ? 'pathOwnership.reviewHeading' : 'pathOwnership.heading')}</h3>
            {#if ownershipReviewing && ownershipSelectedCandidate}
              <dl class="ownership-review">
                <dt>{i18n.t('pathOwnership.reviewRecipient')}</dt>
                <dd><strong>{ownershipSelectedCandidate.displayName}</strong> <span>@{ownershipSelectedCandidate.username}</span></dd>
                <dt>{i18n.t('pathOwnership.recipientRole')}</dt>
                <dd>{i18n.t('pathOwnership.creator')}</dd>
                <dt>{i18n.t('pathOwnership.yourRole')}</dt>
                <dd>{i18n.t('pathOwnership.administrator')}</dd>
              </dl>
              <p>{i18n.t('pathOwnership.recipientBecomesCreator')}</p>
              <p>{i18n.t('pathOwnership.creatorBecomesAdministrator')}</p>
              <p>{i18n.t('pathOwnership.noChangeUntilAccepted')}</p>
              {#if ownershipReviewExpiration}
                <p>{i18n.t('pathOwnership.expiration', { relative: ownershipReviewExpiration.relative, exact: ownershipReviewExpiration.exact })}</p>
              {:else}
                <p role="alert">{i18n.t('pathOwnership.unavailable')}</p>
              {/if}
              <div class="ownership-row-actions">
                <button disabled={ownershipMutationBusy} onclick={() => { ownershipReviewing = false; }}>{i18n.t('common.back')}</button>
                <button disabled={ownershipMutationBusy || !ownershipReviewExpiration || (ownershipReview && Date.parse(ownershipReview.expiresAt) <= now)} onclick={() => void initiateOwnershipTransfer()}>{i18n.t(ownershipMutationAction === 'initiate' ? 'pathOwnership.sending' : 'pathOwnership.confirm')}</button>
              </div>
            {:else}
              <p>{i18n.t('pathOwnership.explanation')}</p>
              {#if ownershipCandidatesBusy && ownershipCandidates.length === 0}
                <p role="status">{i18n.t('pathOwnership.loadingCandidates')}</p>
              {:else if ownershipErrorKey && ownershipCandidates.length === 0}
                <p role="alert">{i18n.t(ownershipErrorKey)}</p>
                <button disabled={ownershipCandidatesBusy} onclick={() => void loadOwnershipCandidates()}>{i18n.t('common.retry')}</button>
              {:else if ownershipCandidates.length === 0}
                <h4>{i18n.t('pathOwnership.candidatesEmpty')}</h4>
                <p>{i18n.t('pathOwnership.candidatesEmptyDescription')}</p>
              {:else}
                <ul class="ownership-list">
                  {#each ownershipCandidates as candidate (candidate.userId)}
                    <li>
                      <button class="ownership-row" aria-label={i18n.t('pathOwnership.candidateAccessibility', { name: candidate.displayName })} disabled={ownershipMutationBusy} onclick={() => void selectOwnershipCandidate(candidate)}>
                        <span class="ownership-row-copy"><strong>{candidate.displayName}</strong><span>@{candidate.username}</span>{#if candidate.administrator}<span class="ownership-role">{i18n.t('pathOwnership.administrator')}</span>{/if}</span>
                        <span aria-hidden="true">›</span>
                      </button>
                    </li>
                  {/each}
                </ul>
                <p class="hint">{i18n.t('pathOwnership.candidateFooter')}</p>
                {#if ownershipCandidateNextCursor}<button disabled={ownershipCandidatesBusy} onclick={() => void loadOwnershipCandidates(ownershipCandidateNextCursor)}>{i18n.t(ownershipCandidatesBusy ? 'pathOwnership.loadingMore' : 'common.loadMore')}</button>{/if}
              {/if}
              <button disabled={ownershipCandidatesBusy || ownershipMutationBusy} onclick={closeOwnershipTransfer}>{i18n.t('common.done')}</button>
            {/if}
            {#if ownershipErrorKey && ownershipCandidates.length > 0}<p role="alert">{i18n.t(ownershipErrorKey)}</p>{/if}
          </section>
        {/if}
        {#if pathDetailBusy || activityPageBusy || revisionPageBusy}<p>{i18n.t('common.loading')}</p>{/if}
        {#if pathDetailErrorKey}<p role="alert">{i18n.t(pathDetailErrorKey)}</p>{/if}
        {#if activityPageFailed}<button disabled={activityPageBusy} onclick={() => void retryActivityPage()}>{i18n.t('common.retry')}</button>{/if}
        {#if historyOpen}
          <h3>{i18n.t('pathDetails.history')}</h3>
          {#if activityDays.length === 0}<p>{i18n.t('pathDetails.historyEmpty')}</p>
          {:else}
            {#each activityDays as day (day.localDate)}
              <section class="activity-day" aria-label={day.localDate}>
                <h4>{i18n.date(activityDayDate(day.localDate), { dateStyle: 'long' })}</h4>
                <ul>
                  {#each day.items as item (item.activity.id)}
                    <li><button class="activity-row" data-activity-id={item.activity.id} disabled={pathDetailBusy || activityPageBusy || revisionPageBusy} onclick={() => void inspectActivity(item.activity.id)}>{i18n.t(item.version > 1 ? 'pathDetails.historyRowEdited' : 'pathDetails.historyRow', { participant: item.activity.participantId, time: i18n.time(new Date(item.activity.startedAt), { timeStyle: 'medium', timeZone: item.activity.occurrenceTimeZone }), seconds: i18n.number(item.activity.durationSeconds) })}</button></li>
                  {/each}
                </ul>
              </section>
            {/each}
          {/if}
          {#if activityNextCursor && !activityPageFailed}<button disabled={activityPageBusy} onclick={() => void loadMoreActivities()}>{i18n.t('common.loadMore')}</button>{/if}
        {/if}
        {#if historyOpen && selectedActivity}
          <section aria-labelledby="activity-detail-heading">
            <h3 id="activity-detail-heading">{i18n.t('pathDetails.activityHeading')}</h3>
            <dl>
              <dt>{i18n.t('pathDetails.started')}</dt><dd>{i18n.date(new Date(selectedActivity.activity.startedAt), { dateStyle: 'medium', timeZone: selectedActivity.activity.occurrenceTimeZone })} {i18n.time(new Date(selectedActivity.activity.startedAt), { timeStyle: 'medium', timeZone: selectedActivity.activity.occurrenceTimeZone })}</dd>
              <dt>{i18n.t('pathDetails.ended')}</dt><dd>{i18n.date(new Date(selectedActivity.activity.endedAt), { dateStyle: 'medium', timeZone: selectedActivity.activity.occurrenceTimeZone })} {i18n.time(new Date(selectedActivity.activity.endedAt), { timeStyle: 'medium', timeZone: selectedActivity.activity.occurrenceTimeZone })}</dd>
              <dt>{i18n.t('activity.durationSeconds')}</dt><dd>{i18n.number(selectedActivity.activity.durationSeconds)}</dd>
            </dl>
            {#if selectedCapabilities.trackTime && selectedActivity.activity.participantId === profile.id}
              {#if selectedActivity.activity.note}<p>{i18n.t('activity.note')}: {selectedActivity.activity.note}</p>{/if}
              <button disabled={managingPathGoals || manualBusy || deleteBusy} onclick={() => void editSelectedActivity()}>{i18n.t('pathDetails.edit')}</button>
              {#if deleteConfirmActivityID === selectedActivity.activity.id}
                <p>{i18n.t('pathDetails.deleteConfirmation')}</p>
                <button disabled={deleteBusy} onclick={cancelDeleteActivity}>{i18n.t('common.cancel')}</button>
                <button disabled={managingPathGoals || deleteBusy || manualBusy} onclick={() => void confirmDeleteActivity()}>{i18n.t(deleteFailed ? 'common.retry' : 'pathDetails.confirmDelete')}</button>
              {:else}
                <button disabled={managingPathGoals || manualBusy || deleteBusy} onclick={beginDeleteSelectedActivity}>{i18n.t('pathDetails.delete')}</button>
              {/if}
            {/if}
            <h4>{i18n.t('pathDetails.revisions')}</h4>
            {#if activityRevisions.length === 0}
              {#if !revisionPageFailed}<p>{i18n.t('pathDetails.revisionsEmpty')}</p>{/if}
            {:else}<ol>{#each activityRevisions as revision (revision.version)}<li><h5>{i18n.t('pathDetails.revision', { version: i18n.number(revision.version) })}</h5><dl><dt>{i18n.t('pathDetails.started')}</dt><dd>{i18n.date(new Date(revision.startedAt), { dateStyle: 'medium', timeZone: revision.occurrenceTimeZone })} {i18n.time(new Date(revision.startedAt), { timeStyle: 'medium', timeZone: revision.occurrenceTimeZone })}</dd><dt>{i18n.t('pathDetails.ended')}</dt><dd>{i18n.date(new Date(revision.endedAt), { dateStyle: 'medium', timeZone: revision.occurrenceTimeZone })} {i18n.time(new Date(revision.endedAt), { timeStyle: 'medium', timeZone: revision.occurrenceTimeZone })}</dd><dt>{i18n.t('activity.durationSeconds')}</dt><dd>{i18n.number(revision.durationSeconds)}</dd></dl>{#if selectedActivity.activity.participantId === profile.id && revision.note}<p>{i18n.t('pathDetails.priorNote')}: {revision.note}</p>{/if}</li>{/each}</ol>{/if}
            {#if revisionPageFailed}<button disabled={revisionPageBusy} onclick={() => void retryRevisionPage()}>{i18n.t('common.retry')}</button>
            {:else if revisionNextCursor}<button disabled={revisionPageBusy} onclick={() => void loadMoreRevisions()}>{i18n.t('common.loadMore')}</button>{/if}
          </section>
        {/if}
      </section>
      {#if selectedCapabilities.trackTime && manualPathID && manualForm}
        <section aria-labelledby="manual-activity-heading">
          <h3 id="manual-activity-heading">{i18n.t(manualActivity ? 'activity.editHeading' : 'activity.addHeading')}</h3>
          <label for="activity-date">{i18n.t('activity.date')}</label>
          <input id="activity-date" type="date" value={manualForm.localDate} disabled={manualBusy} oninput={(event) => changeManualOccurrence({ localDate: event.currentTarget.value })} />
          <label for="activity-start">{i18n.t('activity.startTime')}</label>
          <input id="activity-start" type="time" step="1" value={manualForm.localTime} disabled={manualBusy} oninput={(event) => changeManualOccurrence({ localTime: event.currentTarget.value })} />
          <label for="activity-duration">{i18n.t('activity.durationSeconds')}</label>
          <input id="activity-duration" type="text" inputmode="numeric" pattern="[0-9]*" value={manualForm.durationSeconds} disabled={manualBusy} oninput={(event) => changeManualDuration(event.currentTarget.value)} />
          <label for="activity-note">{i18n.t('activity.note')}</label>
          <textarea id="activity-note" bind:value={manualNote} disabled={manualBusy} oninput={() => { manualIdempotencyKey = crypto.randomUUID(); manualErrorKey = null; }}></textarea>
          <button disabled={manualBusy} onclick={() => void submitManualActivity()}>{i18n.t(manualActivity ? 'activity.saveEdit' : 'activity.save')}</button>
          <button disabled={manualBusy} onclick={closeManualActivity}>{i18n.t('common.cancel')}</button>
          {#if manualActivity}<p role="status">{i18n.t('activity.savedVersion', { version: i18n.number(manualActivity.version) })}</p>{/if}
          {#if manualErrorKey}<p role="alert">{i18n.t(manualErrorKey)}</p>{/if}
        </section>
      {/if}
      {/if}
    {:else}
      <h2>{i18n.t('home.heading')}</h2>
      <button aria-pressed={!showingArchived} onclick={() => { showingArchived = false; }}>{i18n.t('home.activePaths')}</button>
      <button aria-pressed={showingArchived} onclick={() => { showingArchived = true; }}>{i18n.t('home.archivedPaths')}</button>
      {#if showingArchived && archivedPaths.length === 0}
        <p>{i18n.t('home.archivedEmpty')}</p>
      {:else if showingArchived}
        <ul class="path-list">{#each archivedPaths as path (path.id)}<li><button class="path-link" onclick={() => void openPathDetails(path)}>{path.name}</button><p>{i18n.t('pathArchive.readOnly')}</p></li>{/each}</ul>
      {:else if paths.length === 0}
        <section aria-labelledby="empty-home-heading">
          <h3 id="empty-home-heading">{i18n.t('home.empty.heading')}</h3>
          <p>{i18n.t('home.empty.explanation')}</p>
        </section>
      {:else}
        <ul class="path-list">{#each paths as path (path.id)}{@const state = timerStates[path.id]}{@const pathCapabilities = effectivePathCapabilities(path)}<li><button class="path-link" onclick={() => void openPathDetails(path)}>{path.name}</button>{#if path.intervalGoal}<p>{i18n.t('path.goal.intervalSummary', { seconds: i18n.number(path.intervalGoal.targetSeconds), recurrence: recurrenceLabel(path.intervalGoal.recurrence) })}</p>{/if}{#if pathCapabilities.trackTime && state}<p>{i18n.t('path.progress.accumulated', { seconds: i18n.number(state.accumulatedSeconds) })}</p>{@const pathIntervalProgress = intervalProgressPresentation(state.intervalProgress)}{#if pathIntervalProgress}<div><progress max={pathIntervalProgress.targetSeconds} value={pathIntervalProgress.visualSeconds} aria-label={intervalProgressMessage(pathIntervalProgress)}></progress><p>{intervalProgressMessage(pathIntervalProgress)}</p></div>{/if}{#if path.overallTarget}{@const pathOverallProgress = overallProgress(state.accumulatedSeconds, path.overallTarget)}{#if pathOverallProgress}<div><progress max={pathOverallProgress.targetSeconds} value={pathOverallProgress.visualSeconds} aria-label={overallProgressMessage(pathOverallProgress)}></progress><p>{overallProgressMessage(pathOverallProgress)}</p></div>{/if}{/if}{#if state.running}<span aria-label={i18n.t('timer.elapsedValue', { duration: formatTimerDuration(activeTimerSeconds(state.timer?.startedAt, now)) })}>{formatTimerDuration(activeTimerSeconds(state.timer?.startedAt, now))}</span>{/if} <button disabled={timerBusy[path.id] || timerMutationLocked} onclick={() => void toggleTimer(path.id)}>{i18n.t(timerMutationPresentation(state).controlMessage)}</button>{#if timerErrorKeys[path.id]}<p role="alert">{i18n.t(timerErrorKeys[path.id]!)}</p>{/if}{/if}</li>{/each}</ul>
      {/if}
      {#if pathCreated}<p role="status">{i18n.t('pathCreate.created')}</p>{/if}
      {#if timerNoticeKey}<p role="status">{i18n.t(timerNoticeKey)}</p>{/if}
    {/if}
    {#if pathLeaveStatus}<p role="status">{i18n.t(pathLeaveStatus.retained ? 'pathLeave.completedRetained' : 'pathLeave.completedDeleted', { pathName: pathLeaveStatus.pathName })}</p>{/if}
    {#if !selectedPath && !creatingPath}
      <button onclick={openPathCreation}>{i18n.t('home.createPath')}</button>
    {:else if !selectedPath}
      <section aria-labelledby="create-path-heading">
        <h3 id="create-path-heading">{i18n.t('pathCreate.heading')}</h3>
        <label for="path-name">{i18n.t('pathCreate.nameLabel')}</label>
        <input id="path-name" maxlength="100" bind:value={pathName} disabled={pathSubmitting} required />
        <fieldset disabled={pathSubmitting}>
          <legend>{i18n.t('pathCreate.intervalHeading')}</legend>
          <label><input type="checkbox" bind:checked={intervalGoalEnabled} /> {i18n.t('pathCreate.intervalEnabled')}</label>
          {#if intervalGoalEnabled}
            <div class="duration-fields">
              <label for="interval-hours">{i18n.t('pathCreate.duration.hours')}</label>
              <input id="interval-hours" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={intervalHours} />
              <label for="interval-minutes">{i18n.t('pathCreate.duration.minutes')}</label>
              <input id="interval-minutes" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={intervalMinutes} />
              <label for="interval-seconds">{i18n.t('pathCreate.duration.seconds')}</label>
              <input id="interval-seconds" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={intervalSeconds} />
            </div>
            <label for="interval-recurrence">{i18n.t('pathCreate.recurrence')}</label>
            <select id="interval-recurrence" bind:value={intervalRecurrence}>
              <option value="hourly">{i18n.t('pathCreate.recurrence.hourly')}</option>
              <option value="daily">{i18n.t('pathCreate.recurrence.daily')}</option>
              <option value="weekly">{i18n.t('pathCreate.recurrence.weekly')}</option>
              <option value="monthly">{i18n.t('pathCreate.recurrence.monthly')}</option>
              <option value="yearly">{i18n.t('pathCreate.recurrence.yearly')}</option>
            </select>
            <label><input type="checkbox" bind:checked={customAlignment} /> {i18n.t('pathCreate.alignment.customize')}</label>
            {#if !customAlignment}
              <p class="hint">{i18n.t('pathCreate.alignment.default')}</p>
            {:else if intervalRecurrence === 'yearly'}
              <label for="yearly-month">{i18n.t('pathCreate.alignment.month')}</label>
              <select id="yearly-month" bind:value={yearlyMonth}>{#each calendarMonths as month}<option value={String(month)}>{monthName(month)}</option>{/each}</select>
              <label for="yearly-day">{i18n.t('pathCreate.alignment.day')}</label>
              <input id="yearly-day" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={yearlyDay} />
            {:else if intervalRecurrence === 'weekly'}
              <label for="alignment-value">{i18n.t('pathCreate.alignment.weekday')}</label>
              <select id="alignment-value" bind:value={alignmentValue}><option value="">{i18n.t('pathCreate.alignment.chooseWeekday')}</option>{#each isoWeekdays as weekday}<option value={String(weekday)}>{weekdayName(weekday)}</option>{/each}</select>
            {:else}
              <label for="alignment-value">{alignmentInputLabel(intervalRecurrence)}</label>
              <input id="alignment-value" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={alignmentValue} />
            {/if}
          {/if}
        </fieldset>
        <fieldset disabled={pathSubmitting}>
          <legend>{i18n.t('pathCreate.overallHeading')}</legend>
          <label><input type="checkbox" bind:checked={overallTargetEnabled} /> {i18n.t('pathCreate.overallEnabled')}</label>
          {#if overallTargetEnabled}
            <div class="duration-fields">
              <label for="overall-hours">{i18n.t('pathCreate.duration.hours')}</label>
              <input id="overall-hours" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={overallHours} />
              <label for="overall-minutes">{i18n.t('pathCreate.duration.minutes')}</label>
              <input id="overall-minutes" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={overallMinutes} />
              <label for="overall-seconds">{i18n.t('pathCreate.duration.seconds')}</label>
              <input id="overall-seconds" type="text" inputmode="numeric" pattern="[0-9]*" bind:value={overallSeconds} />
            </div>
          {/if}
        </fieldset>
        <button disabled={pathSubmitting} onclick={() => void submitPath()}>{i18n.t(pathSubmitting ? 'pathCreate.submitting' : pathErrorKey ? 'pathCreate.retry' : 'pathCreate.submit')}</button>
        <button disabled={pathSubmitting} onclick={cancelPathCreation}>{i18n.t('common.cancel')}</button>
        {#if pathErrorKey}<p role="alert">{i18n.t(pathErrorKey)}</p>{/if}
      </section>
    {/if}
    {/if}
    <button disabled={timerMutationLocked} onclick={() => void openSignOutDialog()}>{i18n.t('auth.signOut')}</button>
  {:else}<p>{i18n.t('app.ready')}</p><button onclick={signIn}>{i18n.t('auth.signIn')}</button>{/if}
  {#if errorKey}<p role="alert">{i18n.t(errorKey)}</p>{/if}
  {#if signOutDialogOpen}
  <dialog open aria-labelledby="sign-out-dialog-heading" oncancel={cancelSignOut}>
    {#if signOutRunningTimerCount > 0}
      <h2 id="sign-out-dialog-heading">{i18n.t('settings.account.activeTimers.title')}</h2>
      <p>{i18n.t('settings.account.activeTimers.message', { count: signOutRunningTimerCount })}</p>
      {#if signOutErrorKey}<p role="alert">{i18n.t(signOutErrorKey)}</p>{/if}
      <div class="dialog-actions">
        <button type="button" disabled={signOutBusy} onclick={() => void stopTimersAndSignOut()}>
          {i18n.t('settings.account.activeTimers.stopAndSignOut')}
        </button>
        <button type="button" disabled={signOutBusy} onclick={() => void keepTimersRunningAndSignOut()}>
          {i18n.t('settings.account.activeTimers.keepRunningAndSignOut')}
        </button>
        <button type="button" disabled={signOutBusy} onclick={cancelSignOut}>{i18n.t('common.cancel')}</button>
      </div>
    {:else}
      <h2 id="sign-out-dialog-heading">{i18n.t('settings.account.signOutConfirmTitle')}</h2>
      <p>{i18n.t('settings.account.signOutConfirmMessage', { name: profile?.displayName || profile?.email || '' })}</p>
      <div class="dialog-actions">
        <button type="button" disabled={signOutBusy} onclick={() => void completeOrdinarySignOut()}>{i18n.t('auth.signOut')}</button>
        <button type="button" disabled={signOutBusy} onclick={cancelSignOut}>{i18n.t('common.cancel')}</button>
      </div>
    {/if}
  </dialog>
  {/if}
</main>
    <style>main{font:16px system-ui;max-width:42rem;margin:10vh auto;padding:2rem}.primary-tabs{display:grid;grid-template-columns:1fr 1fr;gap:.2rem;margin:1rem 0 1.5rem;padding:.2rem;border-radius:.75rem;background:#e5e5ea}.primary-tabs button{background:transparent;color:#1c1c1e}.primary-tabs button[aria-current="page"]{background:#fff;box-shadow:0 1px 3px #0002}button{display:inline-block;padding:.65rem 1rem;border:0;border-radius:.5rem;background:#111;color:white}.destructive-action{background:transparent;color:#b42318}.destructive-copy{color:#b42318}input,select{display:block;margin:.5rem 0;padding:.65rem}fieldset{margin:1rem 0;border:1px solid #ccc;border-radius:.5rem}.leave-choice{display:flex;align-items:flex-start;gap:.75rem;padding:.75rem 0}.leave-choice input{flex:0 0 auto;margin:.2rem 0}.leave-choice span{display:flex;flex-direction:column;gap:.2rem}.leave-choice small{color:#636366}.duration-fields{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:.75rem}.duration-fields input{width:calc(100% - 1.3rem)}.hint{color:#555}.path-list,.activity-day ul,.notification-list,.ownership-list,.managed-invitation-list{list-style:none;padding:0}.path-list li,.activity-day,.notification-list li{margin:1rem 0}.path-link,.activity-row,.notification-link{background:transparent;color:#111;padding:.25rem 0;text-align:left;text-decoration:underline}.path-link{font-size:1.1rem;font-weight:700}.notification-link{display:block}.notification-list time{display:block;color:#555;margin-top:.25rem}dt{font-weight:700}dd{margin:0 0 .75rem}.ownership-card,.managed-invitations{margin:1rem 0;padding:1rem;border:1px solid #d8d8dc;border-radius:1rem;background:#f7f7f8}.ownership-list,.managed-invitation-list{margin:.75rem 0}.ownership-list>li+li,.managed-invitation-list>li+li{border-top:1px solid #d8d8dc}.ownership-row,.managed-invitation-row{display:flex;width:100%;min-height:3.5rem;align-items:center;justify-content:space-between;gap:1rem;padding:.75rem .25rem}.managed-invitation-copy{display:flex;min-width:0;flex:1;flex-direction:column;gap:.2rem}.managed-invitation-copy span,.managed-invitation-copy time{color:#636366;font-size:.875rem}.ownership-row{border-radius:0;background:transparent;color:#111;text-align:left}.ownership-row-copy{display:flex;min-width:0;flex:1;flex-direction:column;gap:.2rem}.ownership-row-copy span{color:#636366;font-size:.875rem}.ownership-row-copy .ownership-role{color:#8a6b00;font-weight:600}.ownership-row-actions{display:flex;flex-wrap:wrap;gap:.5rem}.ownership-action{padding:.5rem .75rem;background:#f2c94c;color:#1c1c1e}.ownership-action.destructive{background:transparent;color:#b42318}.ownership-review{margin:1rem 0;padding:.75rem;background:white;border-radius:.75rem}dialog{max-width:min(32rem,calc(100vw - 3rem));padding:1.5rem;border:1px solid #d8d8dc;border-radius:1rem}dialog::backdrop{background:#0008}.dialog-actions{display:flex;flex-direction:column;gap:.5rem;margin-top:1.5rem}@media(prefers-color-scheme:dark){main{color:#f5f5f7;background:#111}.primary-tabs{background:#2c2c2e}.primary-tabs button{color:#f5f5f7}.primary-tabs button[aria-current="page"]{background:#48484a}.ownership-card,.managed-invitations{border-color:#3a3a3c;background:#1c1c1e}.ownership-row{color:#f5f5f7}.managed-invitation-copy span,.managed-invitation-copy time{color:#aaa}.ownership-review{background:#2c2c2e}.path-link,.activity-row,.notification-link{color:#f5f5f7}.hint,.leave-choice small{color:#aaa}dialog{border-color:#3a3a3c;background:#1c1c1e;color:#f5f5f7}}</style>
