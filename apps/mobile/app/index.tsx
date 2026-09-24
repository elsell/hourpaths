import * as SecureStore from 'expo-secure-store';
import * as Crypto from 'expo-crypto';
import Constants from 'expo-constants';
import { getCalendars, getLocales } from 'expo-localization';
import { router, useGlobalSearchParams, usePathname } from 'expo-router';
import { useEffect, useMemo, useRef, useState } from 'react';
import { AccessibilityInfo, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { createPathSubmissionOwner, createSessionApiClient, createTimerOperationOwner, formatTimerDuration, generatedResponse, sessionExpiryAdvanced, sessionRefreshDelay, sessionRefreshLeadMs, timerMutationPresentation, type ActivityDeletionResult, type ActivityDetail, type ActivityMutationResult, type GeneratedOperationResult, type ManualActivityDefaults, type MemberRemovalReceipt, type MemberRemovalReview as GeneratedMemberRemovalReview, type OnboardingActivationInput, type OwnershipTransferCandidate as GeneratedOwnershipTransferCandidate, type OwnershipTransferResult, type OwnershipTransferReview, type PathGoalMutationResult, type PathGoalUpdateDraft as GeneratedPathGoalUpdateDraft, type PathRecurrence, type SessionPath, type TimerState, type TimerStopResult } from '@hourpaths/api-client';
import { activeTimerSeconds, appendOptimisticPracticeComment, applyActivityDeletionResult, applyNotificationMutation, applyPathArchiveResult, applyPathDeletionResult, applyPathLeaveResult, applyPathMemberRemovalResult, applyPathMemberRoleChangeResult, applyPathRenameResult, classifySessionFailure, compareGoalConfigurations, createActivityDeletionOperationOwner, createAsyncMutationBarrier, createForegroundNotificationCoordinator, createManualActivityFormState, createPathArchiveOperationOwner, createPathDeletionOperationOwner, createPathInvitationAcceptOwner, createPathInvitationCancelOwner, createPathInvitationRecipientReviewOwner, createPathInvitationRejectOwner, createPathInvitationSendOwner, createPathLeaveOperationOwner, createPathMemberRemovalOperationOwner, createPathMemberRoleChangeOperationOwner, createPathRenameOperationOwner, createSessionOperationOwner, createSignOutTimerResolutionCoordinator, effectivePathCapabilities, exchangeSessionCredential, intervalProgress, isSessionFailure, manualActivityParticipantNow, mergeManagedPendingInvitationPage, mergeNotificationHistoryPage, mergePendingInvitationPage, mergePracticeCommentPage, overallProgress, overrideManualActivityOccurrence, pathInvitationFailureFromProblem, pathInvitationFailureMessageKey, pathInvitationOutputData, practiceCommentFromAPI, practiceCommentPageFromAPI, refreshSessionCredential, removePracticeComment, replacePracticeComment, reviewPathArchiveChange, reviewPathDeletion, reviewPathLeave, reviewPathMemberRemoval, reviewPathMemberRoleChange, reviewPathRename, reviewPendingPathInvitationAcceptance, serializeManualActivityForm, sessionRetryDelay, updateManualActivityDuration, updatePracticeCommentOptimistically, validateSessionCredential, validateSessionMutation, type ActivityDeletionIdentity, type ForegroundNotificationContext, type GoalConfiguration, type GoalConfigurationComparison, type IntervalProgress, type ManagedPendingPathInvitation, type ManagedPendingPathInvitationPage, type ManagedPendingPathInvitationState, type ManualActivityFormState, type ManualActivityLocalDateTime, type ManualActivityParticipantNow, type NotificationHistoryPage, type NotificationHistoryState, type NotificationMutation, type OverallProgress, type PathArchiveReview, type PathDeletionReview, type PathInvitationAcceptanceReview, type PathInvitationFailure, type PathInvitationNotification, type PathInvitationRecipientReview, type PathInvitationRole, type PathLeaveReceipt, type PathLeaveReview, type PathMemberAccessRole, type PathMemberRemovalReview, type PathMemberRoleChangeReceipt, type PendingPathInvitation, type PendingPathInvitationPage, type PracticeComment, type PracticeCommentPage, type RunningTimerSnapshot, type SessionAccessState, type SessionExchangeCredential, type SessionFailure, type SessionOperationTicket } from '@hourpaths/client-core';
import { mergePracticeCommentHistoryPage, practiceCommentHistoryPageFromAPI, practiceCommentMutationFromAPI, restoreDeletedPracticeComment, rollbackPracticeCommentEdit, type PracticeCommentHistoryPage } from '@hourpaths/client-core';
import { applyPathVisibilityResult, createPathVisibilityOperationOwner, pathVisibilityOptions, reviewPathVisibilityChange, type PathVisibility, type PathVisibilityChangeReview } from '@hourpaths/client-core';
import {
  applyPracticeCommentHeartState,
  createPracticeCommentHeartOperationOwner,
  invalidatePracticeCommentHeartRoster,
  mergePracticeCommentHeartRosterPage,
  practiceCommentHeartRosterPageFromAPI,
  practiceCommentHeartStateFromAPI,
  rollbackPracticeCommentHeart,
  updatePracticeCommentHeartOptimistically,
  type PracticeCommentHearter,
  type PracticeCommentHeartRosterPage,
} from '@hourpaths/client-core';
import { createProfileSearchOwner, profileSearchPageFromAPI, profileSearchQuery, publicProfileFromAPI, relationshipMutationResultFromAPI, sessionFailureFromResponse, type ProfileSearchState, type PublicProfile } from '@hourpaths/client-core';
import { followRequestPageFromAPI, followRequestReviewResultFromAPI, mergeFollowRequestPage, removeResolvedFollowRequest, type FollowRequestState as CoreFollowRequestState } from '@hourpaths/client-core';
import {
  createNudgeAudienceOperationOwner,
  createNudgeSendOperationOwner,
  nudgeAudiencePreferenceFromAPI,
  nudgeChannelPreferenceFromAPI,
  nudgeEligibilityFromAPI,
  reviewNudgeAudienceChange,
  reviewNudgeSend,
  type NudgeAudience,
  type NudgeAudiencePreference,
  type NudgeChannelPreference,
  type NudgeChannelUpdateBody,
  type NudgePreset,
} from '@hourpaths/client-core';
import { problemMessageKey, type MessageKey } from '@hourpaths/i18n';
import { createDeviceTranslator } from '../src/i18n';
import { accountShellDestination } from '../src/auth-shell-presentation';
import { loadMobileConfig } from '../src/config';
import { presentNativeDestructiveConfirmation, presentNativePathLeaveChoice } from '../src/ui/native-confirmation';
import {
  ownsManualActivityPresentation,
  type ManualActivityPresentationOwner,
} from '../src/manual-activity-presentation';
import {
  manualActivityDraft,
  manualActivityDraftChanged,
  type ManualActivityDraft,
} from '../src/manual-activity-draft';
import { completeMobileOnboardingActivation } from '../src/onboarding-activation';
import { createOnboardingActivationOwner } from '../src/onboarding-activation-owner';
import { openNativePolicyLink } from '../src/policy-link-native';
import { activatePersistedMobileSession, completeMobileSessionExchange, recoverMobileSession, type MobileSessionDestination, type SessionRecoveryOperation } from '../src/session-exchange';
import { admitMobileSessionPaths, applyRefreshedMobilePath, canCompleteMobileOnboarding, createMobileOnboardingDraft, expoCalendarWeekdayToISO, loadMobileHomeProfile, loadMobileOnboardingProfile, refreshMobileOnboardingPolicy, replaceMobileOnboardingUsername, validProfileUsername, type MobileHomeProfile, type MobileOnboardingDraft } from '../src/session-destination';
import { continueMobileNewAccount } from '../src/recovery-decline';
import { interactionSettingsFromAPI, type InteractionSettings } from '../src/interaction-settings';
import { timeZonePreferenceFromAPI, type FrozenTimeZoneChangeIntent, type TimeZonePreference } from '../src/time-zone-settings';
import { restoreStoredSession } from '../src/session-restoration';
import { applyMobileSessionFailure, createSerializedMobileSessionStorage, disposeMobileSession, shouldTransitionMobileSessionForFeatureFailure } from '../src/session-state';
import { useProviderSignIn } from '../src/provider-auth';
import { buildPathCreateDraft, buildPathGoalUpdateDraft, initialPathGoalForm, pathGoalFormFromPath, type PathGoalForm, type PathGoalUpdateDraft } from '../src/path-goals';
import { defaultPathCreationVisibility, ownsPathCreationTarget, rotatePathCreationTarget, type PathCreationTarget } from '../src/path-creation-target';
import { createPathAdministrationTarget, currentPathAdministrationCredential, ownsPathAdministrationTarget, rotatePathAdministrationTarget, type PathAdministrationTarget } from '../src/path-administration-target';
import {
  createPathNudgePreferenceTarget,
  ownsPathNudgePreferenceTarget,
  pathNudgeFailureDisposition,
  rotatePathNudgePreferenceTarget,
  type PathNudgePreferenceTarget,
} from '../src/path-nudge-preference-target';
import { pathMemberPageFromAPI, type PathMemberPage } from '../src/path-member-page';
import { activityBelongsToProfile, activityEditSeed, appendUniqueActivities, appendUniqueRevisions, newestActivitiesFirst, type ActivityRevision } from '../src/activity-history';
import { applyLoadedActivityDeletionResult } from '../src/activity-deletion-projection';
import { useLatestState } from '../src/use-latest-state';
import { commitTimerProjectionBeforeRender } from '../src/timer-projection-commit';
import { createHomePreferenceOperationOwner, type HomePreferenceSnapshot } from '../src/home-preference-operation';
import { ActivityDetailView } from '../src/ui/activity-detail-view';
import { ActivityHistoryView } from '../src/ui/activity-history-view';
import { DuplicateEmailRecoveryScreen } from '../src/ui/duplicate-email-recovery-screen';
import { ManualActivityForm } from '../src/ui/manual-activity-form';
import { PathCreateForm } from '../src/ui/path-create-form';
import { PathDetailView } from '../src/ui/path-detail-view';
import { PathGoalManagementForm } from '../src/ui/path-goal-management-form';
import { PathArchiveConfirmationSheet } from '../src/ui/path-archive-confirmation-sheet';
import { PathShareSheet } from '../src/ui/path-share-sheet';
import { activityDetailRouteKey, activityHistoryRouteKey, allowNativeChildRouteDismissal, NativeChildRouteSource, pathMemberRemovalRouteKey, pathMembersRouteKey, pathNudgeSettingsRouteKey } from '../src/ui/native-child-route-presentation';
import { PathMemberManagementView, PathMemberRemovalReviewView, type PathMemberListState, type PathMemberSummary } from '../src/ui/path-member-management-view';
import { NudgeComposerSheet } from '../src/ui/nudge-composer-sheet';
import { PathNudgeSettingsView } from '../src/ui/path-nudge-settings-view';
import { nudgeActionState } from '../src/nudge-presentation';
import {
  createNativePushRegistrationCoordinator,
  clearNativePushDeregistration,
  getNativePushPermission,
  installNativeNotificationLifecycle,
  loadNativePushDeregistration,
  nativePushPlatform,
  requestNativePushPermission,
  setNativeNotificationBadge,
  stageNativePushDeregistration,
} from '../src/push-notifications-native';
import { interactionDisabledDestinationFromAPI, type NotificationDestination, type PushPermission } from '../src/push-notifications';
import { foregroundNotificationTargetKey, notificationDestinationTargetKey } from '../src/foreground-notification-routing';
import {
  createPathDetailTarget,
  createPathMemberTarget,
  createPathRouteTarget,
  ownsPathDetailTarget,
  ownsPathMemberTarget,
  ownsPathRouteTarget,
  pathRouteIntent,
  rotatePathDetailTarget,
  rotatePathMemberTarget,
  rotatePathRouteTarget,
  type PathDetailTarget,
  type PathMemberTarget,
  type PathRouteIntent,
  type PathRouteTarget,
} from '../src/path-route-recovery';
import {
  createSocialSessionTarget,
  currentSocialSessionCredential,
  ownsSocialSessionTarget,
  rotateSocialSessionTarget,
  socialRouteHref,
  socialRouteIntent,
  takeSocialRouteBootstrap,
  type SocialSessionTarget,
} from '../src/social-route-recovery';
import {
  createNotificationSessionTarget,
  currentNotificationSessionCredential,
  ownsNotificationSessionTarget,
  rotateNotificationSessionTarget,
  type NotificationSessionTarget,
} from '../src/notification-session-target';
import { shouldHandleSettingsOperationFailure } from '../src/settings-operation-ownership';
import { ProgressIndicator } from '../src/ui/progress-indicator';
import { PathCard } from '../src/ui/path-card';
import { HomeHeaderActions } from '../src/ui/home-header-actions';
import { homePresentation } from '../src/ui/home-presentation';
import { HomeView, type HomeViewSection } from '../src/ui/home-view';
import { HomeArrangementView, type HomeArrangementCollection } from '../src/ui/home-arrangement-view';
import { organizeHomePaths, pinHomePath, reorderVisibleHomePaths, unpinHomePath, type HomeFilter, type HomePath, type HomePreferences } from '../src/ui/home-organization';
import { homeIntervalProgress } from '../src/ui/home-path-presentation';
import { OnboardingForm } from '../src/ui/onboarding-form';
import { SignedOutScreen } from '../src/ui/signed-out-screen';
import { ActionButton, mobileShellStyles as styles, NativeSheet, SectionHeading, StatusBanner, ThemedText as Text } from '../src/ui/primitives';
import { TimerControl } from '../src/ui/timer-control';
import { SettingsPresentationSource, type SignOutPresentationResult, type SignOutTimerChoice } from '../src/ui/settings-presentation';
import { NativeRouteSource, type NativeRouteAction } from '../src/ui/native-route-presentation';
import { NativeRouteRecoveryView } from '../src/ui/native-route-recovery-view';
import { PathInvitationVisibilityWarningSheet } from '../src/ui/path-invitation-visibility-warning-sheet';
import { formatCompactDuration } from '../src/ui/compact-duration';
import {
  InvitationRouteSource,
  NotificationJourneyRecoverySource,
  NotificationRouteSource,
  prepareInvitationRoute,
  prepareNotificationRoute,
} from '../src/ui/notification-route-presentation';
import {
  notificationJourneyHref,
  notificationJourneyIntentFromPathname,
  takeNotificationJourneyBootstrap,
  type NotificationJourneyIntent,
} from '../src/notification-journey-route-recovery';
import {
  settingsJourneyHref,
  settingsJourneyIntentFromPathname,
  takeSettingsJourneyBootstrap,
} from '../src/settings-journey-route-recovery';
import { SettingsJourneyRecoverySource } from '../src/ui/settings-journey-route-presentation';
import { NotificationHistoryView } from '../src/ui/notification-history-view';
import { PendingInvitationsView } from '../src/ui/pending-invitations-view';
import { SocialProfileRouteSource, type SocialFollowRequestState, type SocialProfileDetailState, type SocialProfileSearchState } from '../src/ui/social-profile-route-presentation';
import { SocialRouteRecoverySource, type SocialRouteRecoveryState } from '../src/ui/social-route-recovery-presentation';
import { prepareSocialFeedActivityDetail } from '../src/social-feed-activity-detail';
import { SocialFeedActivityDetailRouteSource, socialFeedActivityDetailRouteKey } from '../src/ui/social-feed-activity-detail-route-presentation';
import { applySocialReactionSummary, mergeSocialFeedPage, preserveNewerSocialReactionSummaries, socialFeedEventFromAPI, socialReactionSummaryFromAPI, type PracticeSessionFeedEvent, type SocialFeedEvent, type SocialFeedPage } from '../src/ui/social-feed-presentation';
import type { SocialReaction } from '../src/ui/social-reaction-presentation';
import {
  activeFollowingItemFromAPI,
  mergeActiveFollowingPage,
  type ActiveFollowingPage,
} from '../src/ui/social-active-following-presentation';
import {
  SocialFeedRouteSource,
  type ActiveFollowingState,
  type SocialFeedState,
} from '../src/ui/social-feed-route-presentation';
import { PracticeCommentsRouteSource, preparePracticeCommentsRoute, type CommentVersionPresentation } from '../src/ui/practice-comments-route-presentation';
import { CommentHeartRosterRouteSource, prepareCommentHeartRosterRoute } from '../src/ui/comment-heart-roster-route-presentation';
import { clearAllPracticeCommentDrafts } from '../src/ui/comment-draft-presentation';
import { ownershipTransferExpirationPresentation } from '../src/ownership-transfer-expiration';
import {
  createUserBlockingGeneratedPort,
} from '../src/user-blocking-port';
import { UserBlockingRouteSource } from '../src/ui/user-blocking-route-presentation';
import {
  OwnershipTransferSheet,
  type OwnershipTransferBusyAction,
  type OwnershipTransferCandidate,
  type PendingOwnershipTransfer,
} from '../src/ui/ownership-transfer-sheet';

const {
  apiURL,
  oidcClientId: clientId,
  oidcIssuer: issuer,
  pushProjectId,
} = loadMobileConfig(Constants.expoConfig?.extra);
const storageKey = 'application_session';
const intervalProgressPresentation = intervalProgress;
const i18n = createDeviceTranslator(getLocales);

type Session = SessionExchangeCredential;
type Profile = MobileHomeProfile;
type OnboardingProfile = MobileOnboardingDraft;
type Destination = MobileSessionDestination<Profile, OnboardingProfile>;
type OnboardingHomeRecovery = Readonly<{
  sessionToken: string;
  status: 'loading' | 'offline' | 'error';
}>;
type HomeRecoveryStatus = 'loading' | 'offline' | 'error';
type HomeRecovery = Readonly<{
  sessionToken: string;
  status: HomeRecoveryStatus;
}>;
type ActivityPage = { items: ActivityDetail[]; nextCursor: string | null };
type RevisionPage = { items: ActivityRevision[]; nextCursor: string | null };
type GoalManagementReview = Omit<GoalConfigurationComparison, 'current' | 'proposed'> & {
  current: GeneratedPathGoalUpdateDraft['expectedGoals'];
  idempotencyKey: string;
  proposed: PathGoalUpdateDraft;
};
type FrozenOwnershipTransferReview = OwnershipTransferReview & {
  idempotencyKey: string;
};

function validOwnershipTransferText(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function pathMemberRemovalReviewFromAPI(pathId: string, value: GeneratedMemberRemovalReview): PathMemberRemovalReview {
  return reviewPathMemberRemoval({
    displayName: value.displayName,
    pathId,
    role: value.role,
    runningTimer: value.runningTimer,
    sessionCount: value.sessionCount,
    totalTrackedSeconds: value.totalTrackedSeconds,
    userId: value.userId,
    username: value.username,
  });
}

function validOwnershipTransferInstant(value: unknown): value is string {
  return validOwnershipTransferText(value) && Number.isFinite(Date.parse(value));
}

function validReviewedOwnershipTransfer(value: unknown): value is OwnershipTransferReview {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const review = value as Record<string, unknown>;
  const recipient = review.recipient;
  return Object.keys(review).sort().join(',') === 'expiresAt,recipient,reservationToken,reviewedAt,viewerTimeZone' &&
    recipient !== null && typeof recipient === 'object' && !Array.isArray(recipient) &&
    Object.keys(recipient).sort().join(',') === 'displayName,userId,username' &&
    validOwnershipTransferText((recipient as Record<string, unknown>).displayName) &&
    validOwnershipTransferText((recipient as Record<string, unknown>).userId) &&
    validOwnershipTransferText((recipient as Record<string, unknown>).username) &&
    validOwnershipTransferText(review.reservationToken) &&
    validOwnershipTransferInstant(review.reviewedAt) &&
    validOwnershipTransferInstant(review.expiresAt) &&
    validOwnershipTransferText(review.viewerTimeZone);
}

function validOwnershipTransferProjection(
  value: unknown,
  expectedPathID: string,
): value is OwnershipTransferResult {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const transfer = value as Record<string, unknown>;
  const counterpart = transfer.counterpart;
  const keys = Object.keys(transfer).sort().join(',');
  const retained = transfer.transfer;
  if (keys !== 'counterpart,counterpartRole,replayed,transfer,viewerTimeZone' &&
      keys !== 'counterpart,counterpartRole,path,replayed,transfer,viewerTimeZone') return false;
  if (!retained || typeof retained !== 'object' || Array.isArray(retained)) return false;
  const record = retained as Record<string, unknown>;
  const terminalKey = record.state === 'accepted' ? 'acceptedAt'
    : record.state === 'declined' ? 'declinedAt'
      : record.state === 'canceled' ? 'canceledAt' : undefined;
  const recordKeys = [
    'createdAt', 'creatorUserId', 'expiresAt', 'id', 'pathId', 'recipientUserId', 'reviewedAt', 'state',
    ...(terminalKey ? [terminalKey] : []),
  ].sort().join(',');
  return Object.keys(record).sort().join(',') === recordKeys &&
    counterpart !== null && typeof counterpart === 'object' && !Array.isArray(counterpart) &&
    Object.keys(counterpart).sort().join(',') === 'displayName,userId,username' &&
    validOwnershipTransferText((counterpart as Record<string, unknown>).displayName) &&
    validOwnershipTransferText((counterpart as Record<string, unknown>).userId) &&
    validOwnershipTransferText((counterpart as Record<string, unknown>).username) &&
    (transfer.counterpartRole === 'creator' || transfer.counterpartRole === 'recipient') &&
    typeof transfer.replayed === 'boolean' &&
    validOwnershipTransferText(record.id) && record.pathId === expectedPathID &&
    validOwnershipTransferText(record.creatorUserId) &&
    validOwnershipTransferText(record.recipientUserId) &&
    (record.state === 'pending' || record.state === 'accepted' || record.state === 'declined' || record.state === 'canceled') &&
    (!terminalKey || validOwnershipTransferInstant(record[terminalKey])) &&
    validOwnershipTransferInstant(record.reviewedAt) &&
    validOwnershipTransferInstant(record.createdAt) &&
    validOwnershipTransferInstant(record.expiresAt) &&
    validOwnershipTransferText(transfer.viewerTimeZone);
}
class LocalizedError extends Error { constructor(readonly key: MessageKey) { super(key); } }

function overallProgressMessage(progress: OverallProgress): string {
  return i18n.t(progress.completed ? 'path.progress.overallDetailComplete' : 'path.progress.overallDetail', {
    accumulated: formatCompactDuration(progress.accumulatedSeconds, i18n),
    target: formatCompactDuration(progress.targetSeconds, i18n),
  });
}

function OverallProgressIndicator({ compact = false, progress, pathName }: { compact?: boolean; progress: OverallProgress; pathName?: string }) {
  const text = compact
    ? i18n.t('path.progress.overallCompact', {
      accumulated: formatCompactDuration(progress.accumulatedSeconds, i18n),
      target: formatCompactDuration(progress.targetSeconds, i18n),
    })
    : overallProgressMessage(progress);
  return <ProgressIndicator
    accessibilityLabel={pathName
      ? i18n.t('path.progress.overallLabelForPath', { path: pathName })
      : i18n.t('path.progress.overallLabel')}
    compact={compact}
    targetValue={progress.targetSeconds}
    text={text}
    visualValue={progress.visualSeconds}
  />;
}

function intervalProgressMessage(progress: IntervalProgress): string {
  return i18n.t(progress.completed ? 'path.progress.intervalDetailComplete' : 'path.progress.intervalDetail', {
    accumulated: formatCompactDuration(progress.accumulatedSeconds, i18n),
    target: formatCompactDuration(progress.targetSeconds, i18n),
  });
}

function IntervalProgressIndicator({ compact = false, progress, pathName }: { compact?: boolean; progress: IntervalProgress; pathName?: string }) {
  const text = compact
    ? i18n.t('path.progress.intervalCompact', {
      accumulated: formatCompactDuration(progress.accumulatedSeconds, i18n),
      target: formatCompactDuration(progress.targetSeconds, i18n),
    })
    : intervalProgressMessage(progress);
  return <ProgressIndicator
    accessibilityLabel={pathName
      ? i18n.t('path.progress.intervalLabelForPath', { path: pathName })
      : i18n.t('path.progress.intervalLabel')}
    compact={compact}
    targetValue={progress.targetSeconds}
    text={text}
    visualValue={progress.visualSeconds}
  />;
}

const deviceCalendar = getCalendars()[0];
const deviceOnboardingDefaults = {
  timeZone: deviceCalendar?.timeZone ?? null,
  firstDayOfWeek: expoCalendarWeekdayToISO(deviceCalendar?.firstWeekday),
};

const serializedSessionStorage = createSerializedMobileSessionStorage({
  read: () => SecureStore.getItemAsync(storageKey),
  write: (session) => SecureStore.setItemAsync(storageKey, JSON.stringify(session)),
  discard: () => SecureStore.deleteItemAsync(storageKey),
});
let pushDeregistrationRetry: Promise<void> | null = null;
async function retryStoredPushDeregistration(): Promise<void> {
  if (pushDeregistrationRetry) return pushDeregistrationRetry;
  pushDeregistrationRetry = (async () => {
    const pending = await loadNativePushDeregistration();
    if (!pending) return;
    const result = generatedResponse(
      await createSessionApiClient(apiURL, () => pending.credential)
        .deletePushInstallation(pending.installationID),
    );
    if (!result.ok && result.status !== 404) throw new Error('push_deregistration_unavailable');
    await clearNativePushDeregistration();
  })();
  try {
    await pushDeregistrationRetry;
  } finally {
    pushDeregistrationRetry = null;
  }
}
const sessionOperations = createSessionOperationOwner();
const manualOperations = createSessionOperationOwner();
const pathDetailOperations = createSessionOperationOwner();
const activityDeletionOperations = createActivityDeletionOperationOwner(() => Crypto.randomUUID());
const goalManagementOperations = createSessionOperationOwner();
const pathVisibilityOperations = createPathVisibilityOperationOwner(() => Crypto.randomUUID());
const notificationRefreshOperations = createSessionOperationOwner();
const notificationMutationOperations = createSessionOperationOwner();
const notificationSettingsOperations = createSessionOperationOwner();
const socialProfileSearchOperations = createSessionOperationOwner();
const socialProfileDetailOperations = createSessionOperationOwner();
const socialFollowRequestOperations = createSessionOperationOwner();
const socialActiveFollowingOperations = createSessionOperationOwner();
const socialFeedOperations = createSessionOperationOwner();
const socialDeepEventOperations = createSessionOperationOwner();
const interactionDisabledEventOperations = createSessionOperationOwner();
const practiceCommentOperations = createSessionOperationOwner();
const practiceCommentHistoryOperations = createSessionOperationOwner();
const practiceCommentMutationOperations = new Map<string, ReturnType<typeof createSessionOperationOwner>>();
const practiceCommentMutationRetries = new Map<string, Readonly<{ intent: string; key: string }>>();
const practiceCommentMutationAdmissions = new Map<string, symbol>();
const practiceCommentHeartOperations = createPracticeCommentHeartOperationOwner(() => Crypto.randomUUID());
const practiceCommentHeartRosterOperations = createSessionOperationOwner();
const socialFeedActivityOperations = createSessionOperationOwner();
const socialFeedReactionOperations = new Map<string, ReturnType<typeof createSessionOperationOwner>>();
const socialFeedReactionRevisions = new Map<string, number>();
const socialFeedReactionAdmissions = new Map<string, symbol>();
const socialFeedReactionRetries = new Map<string, Readonly<{ key: string; reaction: SocialReaction | null }>>();
const socialFeedReactionTargets = new Map<string, SocialSessionTarget<Session>>();
const socialProfileSearchOwner = createProfileSearchOwner();
const pathArchiveOperations = createPathArchiveOperationOwner(() => Crypto.randomUUID());
const pathDeletionOperations = createPathDeletionOperationOwner(() => Crypto.randomUUID());
const pathLeaveOperations = createPathLeaveOperationOwner(() => Crypto.randomUUID());
const pathMemberListOperations = createSessionOperationOwner();
const pathMemberReviewOperations = createSessionOperationOwner();
const pathMemberActivityOperations = createSessionOperationOwner();
const pathMemberUnblockOperations = createSessionOperationOwner();
const nudgeEligibilityOperations = createSessionOperationOwner();
const nudgeSendOperations = createNudgeSendOperationOwner(() => Crypto.randomUUID());
const nudgeAudienceLoadOperations = createSessionOperationOwner();
const nudgeAudienceOperations = createNudgeAudienceOperationOwner(() => Crypto.randomUUID());
const pathMemberRemovalOperations = createPathMemberRemovalOperationOwner(() => Crypto.randomUUID());
const pathMemberRoleChangeOperations = createPathMemberRoleChangeOperationOwner(() => Crypto.randomUUID());
const pathRenameOperations = createPathRenameOperationOwner(() => Crypto.randomUUID());
const pathCreation = createPathSubmissionOwner(() => Crypto.randomUUID());
const timerOperations = createTimerOperationOwner(() => Crypto.randomUUID());
const homePreferenceOperations = createHomePreferenceOperationOwner(() => Crypto.randomUUID());
const timerMutationBarrier = createAsyncMutationBarrier();
const invitationReviewOwner = createPathInvitationRecipientReviewOwner();
const invitationSendOwner = createPathInvitationSendOwner(() => Crypto.randomUUID());
const invitationCancelOwner = createPathInvitationCancelOwner(() => Crypto.randomUUID());
const managedInvitationListOperations = createSessionOperationOwner();
const invitationAcceptOwner = createPathInvitationAcceptOwner(() => Crypto.randomUUID());
const invitationRejectOwner = createPathInvitationRejectOwner(() => Crypto.randomUUID());
const ownershipTransferOperations = createSessionOperationOwner();
const ownershipTransferReviewOperations = createSessionOperationOwner();
async function revokeSupersededSession(session: Session) {
  try { void createSessionApiClient(apiURL, () => session.token).revoke().catch(() => {}); }
  catch { /* Superseded local state remains authoritative. */ }
}
async function refreshSession(session: Session, ticket: SessionOperationTicket): Promise<Session> {
  const replacement = await refreshSessionCredential(
    session,
    async (current) => generatedResponse(await createSessionApiClient(apiURL, () => current.token).refresh()),
    async (replacement) => {
      if (!replacement.nextAction) throw { kind: 'local_storage', reason: 'missing_fields' } satisfies SessionFailure;
      await serializedSessionStorage.persist(replacement as Session, ticket.current);
    },
  );
  if (!replacement.nextAction) throw { kind: 'local_storage', reason: 'missing_fields' } satisfies SessionFailure;
  return replacement as Session;
}
async function loadActivityPage(session: Session, pathID: string, cursor?: string, participantID?: string): Promise<ActivityPage> {
  return validateSessionCredential<ActivityPage>(session, async (credential) => {
    const result = await createSessionApiClient(apiURL, () => credential.token).activities(pathID, cursor, participantID);
    return generatedResponse({
      response: result.response,
      error: result.error,
      data: result.data ? { data: { items: result.data.data, nextCursor: result.data.meta.nextCursor ?? null } } : undefined,
    });
  });
}
async function loadRevisionPage(session: Session, pathID: string, activityID: string, cursor?: string): Promise<RevisionPage> {
  return validateSessionCredential<RevisionPage>(session, async (credential) => {
    const result = await createSessionApiClient(apiURL, () => credential.token).activityRevisions(pathID, activityID, cursor);
    return generatedResponse({
      response: result.response,
      error: result.error,
      data: result.data ? { data: { items: result.data.data, nextCursor: result.data.meta.nextCursor ?? null } } : undefined,
    });
  });
}
async function invitationResponse<T>(result: GeneratedOperationResult<T>): Promise<T> {
  const response = generatedResponse(result);
  if (!response.ok) throw pathInvitationFailureFromProblem(response.status, response.problem);
  const value = await response.json();
  if (value === undefined) throw { kind: 'invalid_response' } as const;
  return value;
}
async function loadPendingInvitationPage(session: Session, cursor = ''): Promise<PendingPathInvitationPage> {
  const envelope = await invitationResponse<unknown>(
    await createSessionApiClient(apiURL, () => session.token).pendingPathInvitations(cursor || undefined),
  );
  if (!envelope || typeof envelope !== 'object') throw { kind: 'invalid_response' } as const;
  const record = envelope as { data?: unknown; meta?: { nextCursor?: unknown } };
  if (!Array.isArray(record.data) || !record.meta || (record.meta.nextCursor !== undefined && typeof record.meta.nextCursor !== 'string')) {
    throw { kind: 'invalid_response' } as const;
  }
  return {
    items: record.data as unknown as PendingPathInvitation[],
    nextCursor: record.meta.nextCursor ?? '',
  };
}
async function loadPendingInvitationsThroughTarget(
  session: Session,
  invitationID?: string,
  isCurrent: () => boolean = () => true,
): Promise<ReturnType<typeof mergePendingInvitationPage>> {
  let pendingInvitations = { items: [], nextCursor: '' } as ReturnType<typeof mergePendingInvitationPage>;
  let cursor = '';
  const visitedCursors = new Set<string>();
  for (let pageCount = 0; pageCount < 100; pageCount += 1) {
    if (!isCurrent()) throw { kind: 'superseded' } as const;
    const page = await loadPendingInvitationPage(session, cursor);
    if (!isCurrent()) throw { kind: 'superseded' } as const;
    pendingInvitations = mergePendingInvitationPage(pendingInvitations, page, cursor);
    if (
      !invitationID ||
      pendingInvitations.items.some((item) => item.invitation.id === invitationID) ||
      !pendingInvitations.nextCursor
    ) return pendingInvitations;
    cursor = pendingInvitations.nextCursor;
    if (visitedCursors.has(cursor)) throw { kind: 'invalid_response' } as const;
    visitedCursors.add(cursor);
  }
  throw { kind: 'invalid_response' } as const;
}
async function loadNotificationHistoryPage(session: Session, cursor = ''): Promise<NotificationHistoryPage> {
  const envelope = await invitationResponse<unknown>(
    await createSessionApiClient(apiURL, () => session.token).notifications(cursor || undefined),
  );
  if (!envelope || typeof envelope !== 'object') throw { kind: 'invalid_response' } as const;
  const record = envelope as { data?: unknown; meta?: { nextCursor?: unknown; unreadCount?: unknown } };
  if (
    !Array.isArray(record.data) ||
    !record.meta ||
    (record.meta.nextCursor !== undefined && typeof record.meta.nextCursor !== 'string') ||
    !Number.isSafeInteger(record.meta.unreadCount) ||
    (record.meta.unreadCount as number) < 0
  ) {
    throw { kind: 'invalid_response' } as const;
  }
  return {
    items: record.data as unknown as PathInvitationNotification[],
    nextCursor: record.meta.nextCursor ?? '',
    unreadCount: record.meta.unreadCount as number,
  };
}
function invitationFailureKey(cause: unknown): MessageKey {
  if (!cause || typeof cause !== 'object' || typeof (cause as { kind?: unknown }).kind !== 'string') {
    return 'pathInvitation.retry';
  }
  return pathInvitationFailureMessageKey(cause as PathInvitationFailure);
}
function failureMessage(failure: SessionFailure): MessageKey {
  const decision = classifySessionFailure(failure);
  if (decision.retryable) return 'errors.temporarilyUnavailable';
  if (decision.discardCredential) return failure.kind === 'local_storage' ? 'errors.localSessionUnreadable' : 'errors.sessionExpired';
  return problemMessageKey(failure.kind === 'http' ? failure.code : undefined);
}
function localizedFailure(cause: unknown, fallback: MessageKey): MessageKey {
  if (cause instanceof LocalizedError) return cause.key;
  if (!isSessionFailure(cause)) return fallback;
  const decision = classifySessionFailure(cause);
  if (decision.retryable) return 'errors.temporarilyUnavailable';
  if (cause.kind === 'local_storage') return 'errors.localSessionUnreadable';
  return problemMessageKey(cause.kind === 'http' ? cause.code : undefined);
}

export default function IndexRedirect() {
  return <HomeScreen />;
}

export function HomeScreen() {
  const pathname = usePathname();
  const routeParameters = useGlobalSearchParams<Record<string, string | string[]>>();
  const currentPathRouteIntent = pathRouteIntent(pathname, routeParameters);
  const currentSocialRouteIntent = socialRouteIntent(pathname, routeParameters);
  const currentNotificationJourneyIntent = notificationJourneyIntentFromPathname(pathname);
  const currentSettingsJourneyIntent = settingsJourneyIntentFromPathname(pathname);
  const foregroundTargetKey = useRef<string | null>(null);
  foregroundTargetKey.current = foregroundNotificationTargetKey(pathname, routeParameters);
  const [session, setSession] = useState<Session | null>(null);
  const userBlockingPort = useMemo(() => {
    const admittedToken = session?.token ?? null;
    return createUserBlockingGeneratedPort(() =>
      createSessionApiClient(apiURL, () => admittedToken),
    );
  }, [session?.token]);
  const [accessState, setAccessState] = useState<SessionAccessState>('authentication_required');
  const [sessionRenewable, setSessionRenewable] = useState(true);
  const [retryAttempt, setRetryAttempt] = useState(0);
  const [retryOperation, setRetryOperation] = useState<SessionRecoveryOperation>('refresh');
  const [destination, setDestination] = useState<Destination | null>(null);
  const homeProjectionSessionToken = useRef<string | null>(null);
  const [decliningRecovery, setDecliningRecovery] = useState(false);
  const [activatingOnboarding, setActivatingOnboarding] = useState(false);
  const [onboardingHomeRecovery, setOnboardingHomeRecovery] = useState<OnboardingHomeRecovery | null>(null);
  const [homeRecovery, setHomeRecovery] = useState<HomeRecovery | null>(null);
  const onboardingActivations = useRef(createOnboardingActivationOwner()).current;
  function invalidateOnboardingActivation() {
    onboardingActivations.invalidate();
    setActivatingOnboarding(false);
    setOnboardingHomeRecovery(null);
  }
  const [ready, setReady] = useState(false);
  const [errorKey, setErrorKey] = useState<MessageKey | null>(null);
  const [offlineStatusDismissed, setOfflineStatusDismissed] = useState(false);
  const [creatingPath, setCreatingPath] = useState(false);
  const [pathName, setPathName] = useState('');
  const [pathVisibility, setPathVisibility] = useState<PathVisibility>('private');
  const [pathGoalForm, setPathGoalForm] = useState<PathGoalForm>(initialPathGoalForm);
  const [pathSubmitting, setPathSubmitting] = useState(false);
  const [pathErrorKey, setPathErrorKey] = useState<MessageKey | null>(null);
  const [pathCreated, setPathCreated] = useState(false);
  const pathCreationTarget = useRef<PathCreationTarget | null>(null);
  const pathCreationBusy = useRef(false);
  const [timerBusy, setTimerBusy] = useState<Record<string, boolean>>({});
  const [timerErrorKeys, setTimerErrorKeys] = useState<Record<string, MessageKey | undefined>>({});
  const [timerNoticeKey, setTimerNoticeKey] = useState<MessageKey | null>(null);
  const [now, setNow] = useState(Date.now());
  const [archivedPathsOpen, setArchivedPathsOpen] = useState(false);
  const [homeArrangementOpen, setHomeArrangementOpen] = useState(false);
  const [homeFilter, setHomeFilter] = useState<HomeFilter>('all');
  const [homePreferences, setHomePreferences] = useState<HomePreferences>({ order: 'recent', pinnedPathIDs: [], manualPathIDs: [] });
  const [homePreferenceBusy, setHomePreferenceBusy] = useState(false);
  const [homePreferenceErrorKey, setHomePreferenceErrorKey] = useState<MessageKey | null>(null);
  const [pathArchiveReview, setPathArchiveReview] = useState<PathArchiveReview | null>(null);
  const [pathArchiveBusy, setPathArchiveBusy] = useState(false);
  const [pathArchiveErrorKey, setPathArchiveErrorKey] = useState<MessageKey | null>(null);
  const [pathArchiveSavedKey, setPathArchiveSavedKey] = useState<MessageKey | null>(null);
  const pathArchiveTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const [pathDeletionReview, setPathDeletionReview] = useState<PathDeletionReview | null>(null);
  const [pathDeletionBusy, setPathDeletionBusy] = useState(false);
  const [pathDeletionErrorKey, setPathDeletionErrorKey] = useState<MessageKey | null>(null);
  const pathDeletionTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const [pathLeaveReview, setPathLeaveReview] = useState<PathLeaveReview | null>(null);
  const [pathLeaveBusy, setPathLeaveBusy] = useState(false);
  const [pathLeaveErrorKey, setPathLeaveErrorKey] = useState<MessageKey | null>(null);
  const [pathLeaveNotice, setPathLeaveNotice] = useState<string | null>(null);
  const pathLeaveTarget = useRef<{ ownerID: string; pathID: string; pathName: string; session: Session } | null>(null);
  const [pathMembersOpen, setPathMembersOpen] = useState(false);
  const [pathMembers, setPathMembers, pathMembersRef] = useLatestState<PathMemberSummary[]>([]);
  const [pathMembersCursor, setPathMembersCursor] = useState<string | null>(null);
  const [pathMembersLoading, setPathMembersLoading] = useState(false);
  const [pathMembersLoadingMore, setPathMembersLoadingMore] = useState(false);
  const [pathMembersError, setPathMembersError] = useState(false);
  const [selectedPathMember, setSelectedPathMember, selectedPathMemberRef] = useLatestState<PathMemberSummary | null>(null);
  const [pathMemberRemovalReview, setPathMemberRemovalReview] = useState<PathMemberRemovalReview | null>(null);
  const [pathMemberReviewLoading, setPathMemberReviewLoading] = useState(false);
  const [pathMemberRemovalBusy, setPathMemberRemovalBusy] = useState(false);
  const [pathMemberRemovalErrorKey, setPathMemberRemovalErrorKey] = useState<MessageKey | null>(null);
  const [pathMemberPendingRole, setPathMemberPendingRole] = useState<PathMemberAccessRole | null>(null);
  const [pathMemberRoleChangeBusy, setPathMemberRoleChangeBusy] = useState(false);
  const [pathMemberRoleChangeErrorKey, setPathMemberRoleChangeErrorKey] = useState<MessageKey | null>(null);
  const [pathMemberUnblockBusy, setPathMemberUnblockBusy] = useState(false);
  const [pathMemberUnblockErrorKey, setPathMemberUnblockErrorKey] = useState<MessageKey | null>(null);
  const [nudgeComposerOpen, setNudgeComposerOpen] = useState(false);
  const [nudgeComposerPreset, setNudgeComposerPreset] = useState<NudgePreset | null>(null);
  const [nudgeSendBusy, setNudgeSendBusy] = useState(false);
  const nudgeSendAdmission = useRef<symbol | null>(null);
  const [nudgeSendErrorKey, setNudgeSendErrorKey] = useState<MessageKey | null>(null);
  const [nudgeSendConfirmationKey, setNudgeSendConfirmationKey] = useState<MessageKey | null>(null);
  const [pathNudgeSettingsOpen, setPathNudgeSettingsOpen] = useState(false);
  const [pathNudgePreference, setPathNudgePreference] = useState<NudgeAudiencePreference | null>(null);
  const [pathNudgePreferenceLoading, setPathNudgePreferenceLoading] = useState(false);
  const [pathNudgePreferenceBusy, setPathNudgePreferenceBusy] = useState(false);
  const pathNudgePreferenceAdmission = useRef<symbol | null>(null);
  const [pathNudgePreferenceError, setPathNudgePreferenceError] = useState(false);
  const pathNudgePreferenceTarget = useRef<PathNudgePreferenceTarget | null>(null);
  const [pathMemberActivities, setPathMemberActivities, pathMemberActivitiesRef] = useLatestState<ActivityDetail[]>([]);
  const [pathMemberActivitiesCursor, setPathMemberActivitiesCursor] = useState<string | null>(null);
  const [pathMemberActivitiesLoading, setPathMemberActivitiesLoading] = useState(false);
  const [pathMemberActivitiesErrorKey, setPathMemberActivitiesErrorKey] = useState<MessageKey | null>(null);
  const pathMemberTarget = useRef<PathMemberTarget | null>(null);
  const [pathRenamePathID, setPathRenamePathID] = useState<string | null>(null);
  const [pathRenameName, setPathRenameName] = useState('');
  const [pathRenameBusy, setPathRenameBusy] = useState(false);
  const [pathRenameErrorKey, setPathRenameErrorKey] = useState<MessageKey | null>(null);
  const [pathRenameSavedName, setPathRenameSavedName] = useState<string | null>(null);
  const pathRenameTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const [goalManagementPathID, setGoalManagementPathID] = useState<string | null>(null);
  const [goalManagementCurrent, setGoalManagementCurrent] = useState<SessionPath | null>(null);
  const [goalManagementForm, setGoalManagementForm] = useState<PathGoalForm | null>(null);
  const [goalManagementReview, setGoalManagementReview] = useState<GoalManagementReview | null>(null);
  const [goalManagementBusy, setGoalManagementBusy] = useState(false);
  const [goalManagementErrorKey, setGoalManagementErrorKey] = useState<MessageKey | null>(null);
  const [goalManagementSaved, setGoalManagementSaved] = useState(false);
  const goalManagementOwnerID = useRef<string | null>(null);
  const goalManagementTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const [pathVisibilityDraft, setPathVisibilityDraft] = useState<PathVisibility>('private');
  const [pathVisibilityReview, setPathVisibilityReview] = useState<Extract<PathVisibilityChangeReview, { kind: 'ready' }> | null>(null);
  const [pathVisibilityBusy, setPathVisibilityBusy] = useState(false);
  const [pathVisibilityErrorKey, setPathVisibilityErrorKey] = useState<MessageKey | null>(null);
  const [pathVisibilitySaved, setPathVisibilitySaved] = useState(false);
  const pathVisibilityTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const pathVisibilityConflict = useRef<PathAdministrationTarget<Session> | null>(null);
  const [manualPathID, setManualPathID] = useState<string | null>(null);
  const [manualForm, setManualForm] = useState<ManualActivityFormState | null>(null);
  const [manualDefaults, setManualDefaults] = useState<ManualActivityDefaults | null>(null);
  const [manualDefaultsLoadedAt, setManualDefaultsLoadedAt] = useState(0);
  const [manualNote, setManualNote] = useState('');
  const [manualBusy, setManualBusy] = useState(false);
  const [manualErrorKey, setManualErrorKey] = useState<MessageKey | null>(null);
  const [manualActivity, setManualActivity] = useState<{ id: string; version: number } | null>(null);
  const [manualSavedVersion, setManualSavedVersion] = useState<number | null>(null);
  const manualDraftBaseline = useRef<ManualActivityDraft | null>(null);
  const manualDraftDirty = manualForm !== null && manualDraftBaseline.current !== null &&
    manualActivityDraftChanged(manualDraftBaseline.current, manualActivityDraft(manualForm, manualNote));
  const [manualIdempotencyKey, setManualIdempotencyKey] = useState('');
  const [manualActivityPresentationOwner, setManualActivityPresentationOwner] = useState<ManualActivityPresentationOwner | null>(null);
  const manualActivityPresentationOwnerRef = useRef<ManualActivityPresentationOwner | null>(null);
  const manualOwnerID = useRef<string | null>(null);
  const [selectedPathID, setSelectedPathID] = useState<string | null>(null);
  const [activityHistoryOpen, setActivityHistoryOpen] = useState(false);
  const [activityHistory, setActivityHistory, activityHistoryRef] = useLatestState<ActivityDetail[]>([]);
  const [activityHistoryCursor, setActivityHistoryCursor] = useState<string | null>(null);
  const [activityHistoryErrorKey, setActivityHistoryErrorKey] = useState<MessageKey | null>(null);
  const [selectedActivity, setSelectedActivity] = useState<ActivityDetail | null>(null);
  const [activityRevisions, setActivityRevisions] = useState<ActivityRevision[]>([]);
  const [activityRevisionCursor, setActivityRevisionCursor] = useState<string | null>(null);
  const [activityRevisionErrorKey, setActivityRevisionErrorKey] = useState<MessageKey | null>(null);
  const [pathDetailBusy, setPathDetailBusy] = useState(false);
  const [pathDetailErrorKey, setPathDetailErrorKey] = useState<MessageKey | null>(null);
  const [activityDeletionBusy, setActivityDeletionBusy] = useState(false);
  const [activityDeletionErrorKey, setActivityDeletionErrorKey] = useState<MessageKey | null>(null);
  const [activityDeletionRetryable, setActivityDeletionRetryable] = useState(false);
  const activityDeletionTarget = useRef<ActivityDeletionIdentity | null>(null);
  const pathDetailOwnerID = useRef<string | null>(null);
  const pathDetailTarget = useRef<PathDetailTarget | null>(null);
  const pathRouteTarget = useRef<PathRouteTarget | null>(null);
  const [pathRouteRecovery, setPathRouteRecovery] = useState<'loading' | 'offline' | 'unavailable' | null>(null);
  const [pendingInvitationsBusy, setPendingInvitationsBusy] = useState(false);
  const [pendingInvitationsErrorKey, setPendingInvitationsErrorKey] = useState<MessageKey | null>(null);
  const [pendingInvitationBusy, setPendingInvitationBusy] = useState<Record<string, boolean | undefined>>({});
  const [pendingInvitationRejecting, setPendingInvitationRejecting] = useState<Record<string, boolean | undefined>>({});
  const [pendingInvitationErrors, setPendingInvitationErrors] = useState<Record<string, MessageKey | undefined>>({});
  const pendingInvitationsTarget = useRef<NotificationSessionTarget<Session> | null>(null);
  const [pendingInvitationAcceptanceReview, setPendingInvitationAcceptanceReview] =
    useState<Extract<PathInvitationAcceptanceReview, { kind: 'confirmation-required' }> | null>(null);
  const [acceptedInvitation, setAcceptedInvitation] = useState<{ role: PathInvitationRole } | null>(null);
  const [rejectedInvitationID, setRejectedInvitationID] = useState<string | null>(null);
  const [invitationsOpen, setInvitationsOpen] = useState(false);
  const [focusedInvitationID, setFocusedInvitationID] = useState<string | null>(null);
  const focusedInvitationIDRef = useRef<string | null>(null);
  focusedInvitationIDRef.current = focusedInvitationID;
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [notificationHistory, setNotificationHistory, notificationHistoryRef] =
    useLatestState<NotificationHistoryState>({ items: [], nextCursor: '', unreadCount: 0 });
  const [notificationUnreadCount, setNotificationUnreadCount] = useState<number | null>(null);
  const [notificationsBusy, setNotificationsBusy] = useState(false);
  const [notificationsRefreshing, setNotificationsRefreshing] = useState(false);
  const [notificationsErrorKey, setNotificationsErrorKey] = useState<MessageKey | null>(null);
  const [notificationTapFeedbackKey, setNotificationTapFeedbackKey] = useState<MessageKey | null>(null);
  const notificationTarget = useRef<NotificationSessionTarget<Session> | null>(null);
  const notificationMutationTarget = useRef<NotificationSessionTarget<Session> | null>(null);
  const notificationMutationAdmission = useRef(false);
  const [notificationMutationGeneration, setNotificationMutationGeneration] = useState(0);
  const notificationSettingsTarget = useRef<NotificationSessionTarget<Session> | null>(null);
  const notificationSettingsAdmission = useRef<symbol | null>(null);
  const settingsOperationTargets = useRef(new Map<string, NotificationSessionTarget<Session>>());
  const settingsMutationAdmissions = useRef(new Map<'interactions' | 'time-zone', symbol>());
  const [socialSearch, setSocialSearch] = useState<SocialProfileSearchState>({
    items: [], loadingMore: false, query: '', refreshing: false, status: 'idle',
  });
  const socialSearchPage = useRef<ProfileSearchState>({ query: '', items: [], nextCursor: '' });
  const [socialProfile, setSocialProfile] = useState<SocialProfileDetailState>({
    refreshing: false, status: 'loading',
  });
  const socialProfileUsernameRef = useRef<string | undefined>(undefined);
  socialProfileUsernameRef.current = socialProfile.username;
  const socialProfileStateRef = useRef(socialProfile);
  socialProfileStateRef.current = socialProfile;
  const socialPresentationGeneration = useRef(0);
  const socialRelationshipSubmitting = useRef(false);
  const socialFollowRequestSubmitting = useRef<string | null>(null);
  const [socialFollowRequests, setSocialFollowRequests] = useState<SocialFollowRequestState>({
    items: [], refreshing: false, status: 'idle',
  });
  const socialFollowRequestPage = useRef<CoreFollowRequestState>({ items: [], nextCursor: '' });
  const socialProfileSearchTarget = useRef<SocialSessionTarget<Session> | null>(null);
  const socialProfileDetailTarget = useRef<SocialSessionTarget<Session> | null>(null);
  const socialFollowRequestTarget = useRef<SocialSessionTarget<Session> | null>(null);
  const [socialActiveFollowing, setSocialActiveFollowing] = useState<ActiveFollowingState>({
    items: [], loadingMore: false, refreshing: false, status: 'idle',
  });
  const socialActiveFollowingPage = useRef<ActiveFollowingPage>({ items: [], nextCursor: '' });
  const socialActiveFollowingTarget = useRef<SocialSessionTarget<Session> | null>(null);
  const [socialFeed, setSocialFeed] = useState<SocialFeedState>({
    items: [], loadingMore: false, refreshing: false, status: 'idle',
  });
  const socialFeedPage = useRef<SocialFeedPage>({ items: [], nextCursor: '' });
  const socialFeedTarget = useRef<SocialSessionTarget<Session> | null>(null);
  const [socialDeepEvent, setSocialDeepEvent] = useState<{
    event?: SocialFeedEvent;
    eventID?: string;
    status: 'idle' | 'loading' | 'ready' | 'error';
  }>({ status: 'idle' });
  const socialDeepEventTarget = useRef<SocialSessionTarget<Session> | null>(null);
  const [practiceComments, setPracticeComments] = useState<{
    busy: boolean;
    errorKey?: MessageKey;
    eventID: string;
    eventOwnerID: string;
    focusedCommentID?: string;
    history?: { commentID: string; errorKey?: MessageKey; loading: boolean; nextCursor?: string; versions: readonly CommentVersionPresentation[] };
    loadingMore: boolean;
    refreshing: boolean;
    status: 'loading' | 'ready' | 'error';
  } | null>(null);
  const practiceCommentPage = useRef<PracticeCommentPage>({ items: [], nextCursor: '' });
  const practiceCommentHistoryPage = useRef<PracticeCommentHistoryPage | null>(null);
  const practiceCommentHistoryTarget = useRef<(SocialSessionTarget<Session> & {
    commentID: string;
    eventID: string;
  }) | null>(null);
  const practiceCommentTarget = useRef<(SocialSessionTarget<Session> & {
    eventID: string;
    eventOwnerID: string;
  }) | null>(null);
  const practiceCommentCreateAdmission = useRef<symbol | null>(null);
  const [practiceCommentHeartRoster, setPracticeCommentHeartRoster] = useState<{
    commentID: string;
    errorKey?: MessageKey;
    eventID: string;
    loadingMore: boolean;
    refreshing: boolean;
    status: 'loading' | 'ready' | 'error';
  } | null>(null);
  const practiceCommentHeartRosterPage = useRef<PracticeCommentHeartRosterPage | null>(null);
  const practiceCommentHeartRosterTarget = useRef<SocialSessionTarget<Session> & {
    commentID: string;
    eventID: string;
  } | null>(null);
  const [socialFeedActivity, setSocialFeedActivity] = useState<{
    busy: boolean;
    detail: ActivityDetail;
    errorKey?: MessageKey;
    event: PracticeSessionFeedEvent;
    nextCursor?: string;
    revisions: readonly ActivityRevision[];
  } | null>(null);
  const socialFeedActivityTarget = useRef<{
    activityID: string;
    ownerID: string;
    pathID: string;
    session: Session;
  } | null>(null);
  const notificationLifecycleState = useRef<{ destination: Destination | null; session: Session | null }>({
    destination: null,
    session: null,
  });
  const signOutTimerResolutions = useRef(createSignOutTimerResolutionCoordinator());
  const pushCredentials = useRef(new Map<string, string>());
  const pushRegistrationCoordinator = useRef<ReturnType<typeof createNativePushRegistrationCoordinator> | null>(null);
  const [sharingPath, setSharingPath] = useState(false);
  const [invitationUsername, setInvitationUsername] = useState('');
  const [invitationRole, setInvitationRole] = useState<PathInvitationRole>('participant');
  const [invitationReview, setInvitationReview] = useState<PathInvitationRecipientReview | null>(null);
  const [invitationReviewBusy, setInvitationReviewBusy] = useState(false);
  const [invitationSendBusy, setInvitationSendBusy] = useState(false);
  const [invitationErrorKey, setInvitationErrorKey] = useState<MessageKey | null>(null);
  const [invitationSent, setInvitationSent] = useState(false);
  const [managedInvitationState, setManagedInvitationState] = useState<ManagedPendingPathInvitationState>({ items: [], nextCursor: '' });
  const [managedInvitationsBusy, setManagedInvitationsBusy] = useState(false);
  const [managedInvitationsErrorKey, setManagedInvitationsErrorKey] = useState<MessageKey | undefined>();
  const [managedInvitationBusy, setManagedInvitationBusy] = useState<Record<string, boolean | undefined>>({});
  const [managedInvitationErrors, setManagedInvitationErrors] = useState<Record<string, MessageKey | undefined>>({});
  const invitationTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const invitationAcceptanceTargets = useRef(
    new Map<string, { ownerID: string; session: Session }>(),
  );
  const invitationRejectionTargets = useRef(
    new Map<string, { ownerID: string; session: Session }>(),
  );
  const [ownershipTransferOpen, setOwnershipTransferOpen] = useState(false);
  const [ownershipTransferPathID, setOwnershipTransferPathID] = useState<string | null>(null);
  const [ownershipTransferCandidates, setOwnershipTransferCandidates] = useState<OwnershipTransferCandidate[]>([]);
  const [ownershipTransferCandidatesCursor, setOwnershipTransferCandidatesCursor] = useState('');
  const [ownershipTransferCandidatesLoading, setOwnershipTransferCandidatesLoading] = useState(false);
  const [ownershipTransferCandidatesLoaded, setOwnershipTransferCandidatesLoaded] = useState(false);
  const [ownershipTransferCandidatesErrorKey, setOwnershipTransferCandidatesErrorKey] = useState<MessageKey | undefined>();
  const [ownershipTransferSelectedRecipient, setOwnershipTransferSelectedRecipient] = useState<OwnershipTransferCandidate | null>(null);
  const [ownershipTransferReview, setOwnershipTransferReview] = useState<FrozenOwnershipTransferReview | null>(null);
  const [ownershipTransferReviewBusy, setOwnershipTransferReviewBusy] = useState(false);
  const [pendingOwnershipTransfer, setPendingOwnershipTransfer] = useState<PendingOwnershipTransfer | undefined>();
  const [pendingOwnershipTransferLoading, setPendingOwnershipTransferLoading] = useState(false);
  const [ownershipTransferExpirationSummary, setOwnershipTransferExpirationSummary] = useState<string | undefined>();
  const [ownershipTransferBusyAction, setOwnershipTransferBusyAction] = useState<OwnershipTransferBusyAction | undefined>();
  const [ownershipTransferErrorKey, setOwnershipTransferErrorKey] = useState<MessageKey | null>(null);
  const ownershipTransferTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const pathManagementHandoffTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const pathVisibilityConfirmationTarget = useRef<PathAdministrationTarget<Session> | null>(null);
  const ownershipTransferIdempotencyKeys = useRef(new Map<string, string>());
  const providerSignIn = useProviderSignIn(issuer, clientId, 'hourpaths');
  useEffect(() => {
    const shellDestination = accountShellDestination({
      destinationKind: destination?.kind ?? null,
      pathname,
      ready,
    });
    if (shellDestination === 'home-tabs') router.replace('/(tabs)/home');
    if (shellDestination === 'account-entry') router.replace('/');
  }, [destination?.kind, pathname, ready]);
  useEffect(() => {
    if (ownershipTransferOpen && pendingOwnershipTransfer?.viewerRole === 'creator' &&
      ownershipTransferCandidates.length === 0 && !ownershipTransferCandidatesLoaded && !ownershipTransferCandidatesLoading &&
      !ownershipTransferCandidatesErrorKey) void loadOwnershipTransferCandidates();
  }, [ownershipTransferCandidates.length, ownershipTransferCandidatesErrorKey,
    ownershipTransferCandidatesLoaded, ownershipTransferCandidatesLoading,
    ownershipTransferOpen, pendingOwnershipTransfer?.viewerRole]);
  notificationLifecycleState.current = { destination, session };
  function currentAdministrationSession(
    target: PathAdministrationTarget<Session> | null,
    pathID: string,
  ): Session | null {
    const active = notificationLifecycleState.current;
    return currentPathAdministrationCredential(
      target,
      active.destination?.kind === 'home' ? active.destination.profile.id : null,
      pathID,
      active.session,
    );
  }
  if (!pushRegistrationCoordinator.current && pushProjectId) {
    pushRegistrationCoordinator.current = createNativePushRegistrationCoordinator({
      projectId: pushProjectId,
      channelName: i18n.t('notification.settings.channel'),
      register: async ({ accountID, installationID, pushToken }) => {
        const credential = pushCredentials.current.get(accountID);
        if (!credential) throw new Error('push_registration_session_missing');
        const platform = nativePushPlatform();
        if (!platform) throw new Error('push_registration_platform_unsupported');
        const result = generatedResponse(
          await createSessionApiClient(apiURL, () => credential).upsertPushInstallation(
            installationID,
            {
              provider: 'expo',
              platform,
              locale: i18n.locale,
              token: pushToken,
            },
          ),
        );
        if (!result.ok) throw new Error('push_registration_unavailable');
        const pending = await loadNativePushDeregistration();
        if (pending?.installationID === installationID) {
          await clearNativePushDeregistration();
        }
      },
      deregister: async ({ accountID, installationID }) => {
        const credential = pushCredentials.current.get(accountID);
        if (!credential) throw new Error('push_registration_session_missing');
        await stageNativePushDeregistration({ accountID, installationID, credential });
        const result = generatedResponse(
          await createSessionApiClient(apiURL, () => credential).deletePushInstallation(installationID),
        );
        if (!result.ok && result.status !== 404) throw new Error('push_deregistration_unavailable');
        await clearNativePushDeregistration();
        pushCredentials.current.delete(accountID);
      },
    });
  }

  function resetPathDeletion() {
    pathDeletionOperations.cancel(pathDeletionReview?.pathId);
    pathDeletionTarget.current = null;
    setPathDeletionReview(null);
    setPathDeletionBusy(false);
    setPathDeletionErrorKey(null);
  }

  function resetPathLeave() {
    pathLeaveOperations.cancel(pathLeaveReview?.pathId);
    pathLeaveTarget.current = null;
    setPathLeaveReview(null);
    setPathLeaveBusy(false);
    setPathLeaveErrorKey(null);
  }

  function resetNudgeComposer() {
    nudgeEligibilityOperations.invalidate();
    nudgeSendOperations.cancel();
    nudgeSendAdmission.current = null;
    setNudgeComposerOpen(false);
    setNudgeComposerPreset(null);
    setNudgeSendBusy(false);
    setNudgeSendErrorKey(null);
    setNudgeSendConfirmationKey(null);
  }

  function closePathNudgeSettings(force = false) {
    if (!force && pathNudgePreferenceAdmission.current) return;
    const target = pathNudgePreferenceTarget.current;
    nudgeAudienceLoadOperations.invalidate();
    nudgeAudienceOperations.cancel();
    pathNudgePreferenceAdmission.current = null;
    pathNudgePreferenceTarget.current = null;
    if (target && pathRouteTarget.current?.routeKey === pathNudgeSettingsRouteKey(target.pathID)) {
      pathRouteTarget.current = null;
      setPathRouteRecovery(null);
    }
    setPathNudgeSettingsOpen(false);
    setPathNudgePreference(null);
    setPathNudgePreferenceLoading(false);
    setPathNudgePreferenceBusy(false);
    setPathNudgePreferenceError(false);
  }

  function closePathMemberRemovalRoute() {
    pathMemberReviewOperations.invalidate();
    pathMemberActivityOperations.invalidate();
    pathMemberUnblockOperations.invalidate();
    resetNudgeComposer();
    if (!pathMemberRemovalBusy && !pathMemberRoleChangeBusy && selectedPathMember && pathMemberTarget.current) {
      pathMemberRemovalOperations.cancel(pathMemberTarget.current.pathID, selectedPathMember.userId);
      pathMemberRoleChangeOperations.cancel(pathMemberTarget.current.pathID, selectedPathMember.userId);
    }
    const target = pathMemberTarget.current;
    pathMemberTarget.current = target ? { ...target, userID: undefined } : null;
    setSelectedPathMember(null);
    setPathMemberRemovalReview(null);
    setPathMemberReviewLoading(false);
    setPathMemberRemovalBusy(false);
    setPathMemberRemovalErrorKey(null);
    setPathMemberPendingRole(null);
    setPathMemberRoleChangeBusy(false);
    setPathMemberRoleChangeErrorKey(null);
    setPathMemberUnblockBusy(false);
    setPathMemberUnblockErrorKey(null);
    setPathMemberActivities([]);
    setPathMemberActivitiesCursor(null);
    setPathMemberActivitiesLoading(false);
    setPathMemberActivitiesErrorKey(null);
  }

  function closePathMembersRoute(preserveComparison = false) {
    closePathMemberRemovalRoute();
    setPathMembersOpen(false);
    if (preserveComparison) return;
    pathMemberListOperations.invalidate();
    pathMemberTarget.current = null;
    setPathMembers([]);
    setPathMembersCursor(null);
    setPathMembersLoading(false);
    setPathMembersLoadingMore(false);
    setPathMembersError(false);
  }

  function resetGoalManagement() {
    pathManagementHandoffTarget.current = null;
    resetPathDeletion();
    goalManagementOperations.invalidate();
    resetPathVisibilityState();
    goalManagementTarget.current = null;
    goalManagementOwnerID.current = null;
    setGoalManagementPathID(null);
    setGoalManagementCurrent(null);
    setGoalManagementForm(null);
    setGoalManagementReview(null);
    setGoalManagementBusy(false);
    setGoalManagementErrorKey(null);
    setGoalManagementSaved(false);
  }

  function resetPathVisibilityState() {
    pathVisibilityConfirmationTarget.current = null;
    pathVisibilityOperations.cancel();
    pathVisibilityTarget.current = null;
    pathVisibilityConflict.current = null;
    setPathVisibilityDraft('private');
    setPathVisibilityReview(null);
    setPathVisibilityBusy(false);
    setPathVisibilityErrorKey(null);
    setPathVisibilitySaved(false);
  }

  function resetPathArchive() {
    pathArchiveOperations.cancel(pathArchiveReview?.pathId);
    pathArchiveTarget.current = null;
    setPathArchiveReview(null);
    setPathArchiveBusy(false);
    setPathArchiveErrorKey(null);
    setPathArchiveSavedKey(null);
  }

  function resetPathRename() {
    pathRenameOperations.cancel(pathRenamePathID ?? undefined);
    pathRenameTarget.current = null;
    setPathRenamePathID(null);
    setPathRenameName('');
    setPathRenameBusy(false);
    setPathRenameErrorKey(null);
    setPathRenameSavedName(null);
  }

  function resetInvitationShare() {
    invitationReviewOwner.cancel();
    invitationSendOwner.cancel(selectedPathID ?? undefined);
    invitationCancelOwner.cancel();
    managedInvitationListOperations.invalidate();
    invitationTarget.current = null;
    setSharingPath(false);
    setInvitationUsername('');
    setInvitationRole('participant');
    setInvitationReview(null);
    setInvitationReviewBusy(false);
    setInvitationSendBusy(false);
    setInvitationErrorKey(null);
    setInvitationSent(false);
    setManagedInvitationState({ items: [], nextCursor: '' });
    setManagedInvitationsBusy(false);
    setManagedInvitationsErrorKey(undefined);
    setManagedInvitationBusy({});
    setManagedInvitationErrors({});
    resetPathVisibilityState();
  }

  function resetInvitations() {
    resetInvitationShare();
    invitationAcceptOwner.cancel();
    invitationRejectOwner.cancel();
    invitationAcceptanceTargets.current.clear();
    invitationRejectionTargets.current.clear();
    pendingInvitationsTarget.current = null;
    setPendingInvitationsBusy(false);
    setPendingInvitationsErrorKey(null);
    setPendingInvitationBusy({});
    setPendingInvitationRejecting({});
    setPendingInvitationErrors({});
    setPendingInvitationAcceptanceReview(null);
    setAcceptedInvitation(null);
    setRejectedInvitationID(null);
    setInvitationsOpen(false);
    setFocusedInvitationID(null);
  }

  function resetOwnershipTransfer() {
    ownershipTransferOperations.invalidate();
    ownershipTransferTarget.current = null;
    ownershipTransferIdempotencyKeys.current.clear();
    setOwnershipTransferOpen(false);
    setOwnershipTransferPathID(null);
    setOwnershipTransferCandidates([]);
    setOwnershipTransferCandidatesCursor('');
    setOwnershipTransferCandidatesLoading(false);
    setOwnershipTransferCandidatesLoaded(false);
    setOwnershipTransferCandidatesErrorKey(undefined);
    setOwnershipTransferSelectedRecipient(null);
    setOwnershipTransferReview(null);
    setOwnershipTransferReviewBusy(false);
    ownershipTransferReviewOperations.invalidate();
    setPendingOwnershipTransfer(undefined);
    setPendingOwnershipTransferLoading(false);
    setOwnershipTransferExpirationSummary(undefined);
    setOwnershipTransferBusyAction(undefined);
    setOwnershipTransferErrorKey(null);
  }

  function resetNotifications() {
    notificationRefreshOperations.invalidate();
    notificationMutationOperations.invalidate();
    notificationSettingsOperations.invalidate();
    notificationTarget.current = null;
    notificationMutationTarget.current = null;
    notificationMutationAdmission.current = false;
    notificationSettingsTarget.current = null;
    notificationSettingsAdmission.current = null;
    setNotificationsOpen(false);
    setNotificationHistory({ items: [], nextCursor: '', unreadCount: 0 });
    setNotificationUnreadCount(null);
    setNotificationsBusy(false);
    setNotificationsRefreshing(false);
    setNotificationsErrorKey(null);
    setNotificationTapFeedbackKey(null);
  }

  function resetSettingsOperations() {
    settingsOperationTargets.current.clear();
    settingsMutationAdmissions.current.clear();
  }

  function resetSocialProfileDiscovery() {
    clearAllPracticeCommentDrafts();
    socialPresentationGeneration.current += 1;
    socialRelationshipSubmitting.current = false;
    socialFollowRequestSubmitting.current = null;
    socialProfileSearchOwner.cancel();
    socialProfileSearchOperations.invalidate();
    socialProfileDetailOperations.invalidate();
    socialFollowRequestOperations.invalidate();
    socialActiveFollowingOperations.invalidate();
    socialFeedOperations.invalidate();
    socialDeepEventOperations.invalidate();
    interactionDisabledEventOperations.invalidate();
    practiceCommentOperations.invalidate();
    practiceCommentHistoryOperations.invalidate();
    practiceCommentHeartRosterOperations.invalidate();
    for (const owner of practiceCommentMutationOperations.values()) owner.invalidate();
    practiceCommentMutationOperations.clear();
    practiceCommentMutationRetries.clear();
    practiceCommentMutationAdmissions.clear();
    socialFeedActivityOperations.invalidate();
    for (const owner of socialFeedReactionOperations.values()) owner.invalidate();
    socialFeedReactionOperations.clear();
    socialFeedReactionRevisions.clear();
    socialFeedReactionAdmissions.clear();
    socialFeedReactionRetries.clear();
    socialFeedReactionTargets.clear();
    socialProfileSearchTarget.current = null;
    socialProfileDetailTarget.current = null;
    socialFollowRequestTarget.current = null;
    socialActiveFollowingTarget.current = null;
    socialActiveFollowingPage.current = { items: [], nextCursor: '' };
    setSocialActiveFollowing({ items: [], loadingMore: false, refreshing: false, status: 'idle' });
    socialFeedTarget.current = null;
    socialFeedPage.current = { items: [], nextCursor: '' };
    setSocialFeed({ items: [], loadingMore: false, refreshing: false, status: 'idle' });
    socialDeepEventTarget.current = null;
    setSocialDeepEvent({ status: 'idle' });
    for (const comment of practiceCommentPage.current.items) practiceCommentHeartOperations.invalidate(comment.id);
    practiceCommentPage.current = { items: [], nextCursor: '' };
    practiceCommentHistoryPage.current = null;
    practiceCommentHistoryTarget.current = null;
    practiceCommentTarget.current = null;
    practiceCommentCreateAdmission.current = null;
    setPracticeComments(null);
    practiceCommentHeartRosterPage.current = null;
    practiceCommentHeartRosterTarget.current = null;
    setPracticeCommentHeartRoster(null);
    socialFeedActivityTarget.current = null;
    setSocialFeedActivity(null);
    socialSearchPage.current = { query: '', items: [], nextCursor: '' };
    setSocialSearch({ items: [], loadingMore: false, query: '', refreshing: false, status: 'idle' });
    setSocialProfile({ refreshing: false, status: 'loading' });
    socialFollowRequestPage.current = { items: [], nextCursor: '' };
    setSocialFollowRequests({ items: [], refreshing: false, status: 'idle' });
  }

  function hideBlockedUserFromSocialSurfaces(userId: string) {
    socialSearchPage.current = {
      ...socialSearchPage.current,
      items: socialSearchPage.current.items.filter((profile) => profile.userId !== userId),
    };
    setSocialSearch((current) => ({
      ...current,
      items: current.items.filter((profile) => profile.userId !== userId),
    }));
    setSocialProfile((current) => current.profile?.userId === userId
      ? { refreshing: false, status: 'error', errorKey: 'social.profileUnavailableDescription' }
      : current);

    socialFollowRequestPage.current = {
      ...socialFollowRequestPage.current,
      items: socialFollowRequestPage.current.items.filter(({ requester }) => requester.userId !== userId),
    };
    setSocialFollowRequests((current) => ({
      ...current,
      items: current.items.filter(({ requester }) => requester.userId !== userId),
    }));

    socialActiveFollowingPage.current = {
      ...socialActiveFollowingPage.current,
      items: socialActiveFollowingPage.current.items.filter(({ participant }) => participant.userId !== userId),
    };
    setSocialActiveFollowing((current) => ({
      ...current,
      items: current.items.filter(({ participant }) => participant.userId !== userId),
    }));
    socialFeedPage.current = {
      ...socialFeedPage.current,
      items: socialFeedPage.current.items.filter(({ participant }) => participant.userId !== userId),
    };
    setSocialFeed((current) => ({
      ...current,
      items: current.items.filter(({ participant }) => participant.userId !== userId),
    }));

    practiceCommentPage.current = {
      ...practiceCommentPage.current,
      items: practiceCommentPage.current.items.filter(({ authorUserId }) => authorUserId !== userId),
    };
    setPracticeComments((current) => current ? { ...current } : null);
    if (practiceCommentHeartRosterPage.current) {
      practiceCommentHeartRosterPage.current = {
        ...practiceCommentHeartRosterPage.current,
        items: practiceCommentHeartRosterPage.current.items.filter((person) => person.userId !== userId),
      };
      setPracticeCommentHeartRoster((current) => current ? { ...current } : null);
    }
    setSocialFeedActivity((current) => current?.event.participant.userId === userId ? null : current);

    setNotificationHistory((current) => {
      const removedUnread = current.items.filter((item) => item.actor.userId === userId && !item.read).length;
      return {
        ...current,
        items: current.items.filter((item) => item.actor.userId !== userId),
        unreadCount: Math.max(0, current.unreadCount - removedUnread),
      };
    });
    setNotificationUnreadCount((current) => current === null ? null : Math.max(0, current -
      notificationHistory.items.filter((item) => item.actor.userId === userId && !item.read).length));
  }

  function resetManualActivity() {
    manualOperations.invalidate();
    manualActivityPresentationOwnerRef.current = null;
    manualOwnerID.current = null;
    setManualActivityPresentationOwner(null);
    setManualPathID(null);
    setManualForm(null);
    setManualDefaults(null);
    setManualDefaultsLoadedAt(0);
    setManualNote('');
    setManualBusy(false);
    setManualErrorKey(null);
    setManualActivity(null);
    setManualSavedVersion(null);
    manualDraftBaseline.current = null;
    setManualIdempotencyKey('');
  }

  function resetTimerPresentation() {
    signOutTimerResolutions.current.invalidate();
    timerOperations.cancel();
    setTimerBusy({});
    setTimerErrorKeys({});
    setTimerNoticeKey(null);
  }

  function claimManualActivityPresentation(owner: ManualActivityPresentationOwner) {
    manualActivityPresentationOwnerRef.current = owner;
    setManualActivityPresentationOwner(owner);
  }

  function resetPathDetail() {
    resetPathLeave();
    closePathMembersRoute();
    closePathNudgeSettings(true);
    pathDetailOperations.invalidate();
    activityDeletionOperations.invalidate();
    activityDeletionTarget.current = null;
    pathDetailOwnerID.current = null;
    pathDetailTarget.current = null;
    pathRouteTarget.current = null;
    setPathRouteRecovery(null);
    setSelectedPathID(null);
    setActivityHistoryOpen(false);
    setActivityHistory([]);
    setActivityHistoryCursor(null);
    setActivityHistoryErrorKey(null);
    setSelectedActivity(null);
    setActivityRevisions([]);
    setActivityRevisionCursor(null);
    setActivityRevisionErrorKey(null);
    setPathDetailBusy(false);
    setPathDetailErrorKey(null);
    setActivityDeletionBusy(false);
    setActivityDeletionErrorKey(null);
    setActivityDeletionRetryable(false);
    resetPathRename();
    resetPathArchive();
    resetInvitationShare();
    resetOwnershipTransfer();
  }

  function closeActivityHistoryRoute() {
    pathDetailOperations.invalidate();
    const target = pathDetailTarget.current;
    pathDetailTarget.current = target ? { ...target, activityID: undefined } : null;
    setActivityHistoryOpen(false);
    setSelectedActivity(null);
    setActivityRevisions([]);
    setActivityRevisionCursor(null);
    setActivityRevisionErrorKey(null);
    setPathDetailBusy(false);
    setPathDetailErrorKey(null);
  }

  function closeActivityDetailRoute(activityID: string) {
    if (pathDetailTarget.current?.activityID !== activityID) return;
    if (manualActivityPresentationOwnerRef.current === 'activity-details') resetManualActivity();
    pathDetailOperations.invalidate();
    activityDeletionOperations.invalidate();
    activityDeletionTarget.current = null;
    const target = pathDetailTarget.current;
    pathDetailTarget.current = { ...target, activityID: undefined };
    setSelectedActivity(null);
    setActivityRevisions([]);
    setActivityRevisionCursor(null);
    setActivityRevisionErrorKey(null);
    setPathDetailBusy(false);
    setPathDetailErrorKey(null);
    setActivityDeletionBusy(false);
    setActivityDeletionErrorKey(null);
    setActivityDeletionRetryable(false);
    if (pathMembersOpen && selectedPathMember) setActivityHistoryOpen(false);
  }

  function beginActivityDeletion() {
    if (
      !session ||
      destination?.kind !== 'home' ||
      !selectedPath ||
      !selectedActivity ||
      activityDeletionBusy ||
      manualBusy ||
      selectedPath.archivedAt ||
      !effectivePathCapabilities(selectedPath).trackTime ||
      !activityBelongsToProfile(selectedActivity, destination.profile.id)
    ) return;
    const identity: ActivityDeletionIdentity = Object.freeze({
      activityId: selectedActivity.activity.id,
      ownerId: destination.profile.id,
      pathId: selectedPath.id,
      sessionToken: session.token,
    });
    activityDeletionOperations.intent(identity);
    activityDeletionTarget.current = identity;
    setActivityDeletionErrorKey(null);
    setActivityDeletionRetryable(false);
    presentNativeDestructiveConfirmation({
      title: i18n.t('pathDetails.deleteConfirmationHeading'),
      message: i18n.t('pathDetails.deleteConfirmation'),
      cancelLabel: i18n.t('common.cancel'),
      confirmLabel: i18n.t('pathDetails.confirmDelete'),
      onConfirm: () => void submitActivityDeletion(identity),
    });
  }

  async function submitActivityDeletion(identity: ActivityDeletionIdentity) {
    if (
      !session ||
      destination?.kind !== 'home' ||
      !selectedPath ||
      !selectedActivity ||
      activityDeletionBusy ||
      manualBusy ||
      activityDeletionTarget.current !== identity ||
      session.token !== identity.sessionToken ||
      destination.profile.id !== identity.ownerId ||
      selectedPath.id !== identity.pathId ||
      selectedActivity.activity.id !== identity.activityId ||
      selectedPath.archivedAt ||
      !effectivePathCapabilities(selectedPath).trackTime ||
      !activityBelongsToProfile(selectedActivity, destination.profile.id)
    ) return;
    const currentSession = session;
    const { activityId: activityID, ownerId: ownerID, pathId: pathID } = identity;
    setActivityDeletionBusy(true);
    setActivityDeletionErrorKey(null);
    setActivityDeletionRetryable(false);
    const mutation = await activityDeletionOperations.submit(identity, true, async (idempotencyKey) =>
      validateSessionCredential<ActivityDeletionResult>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).deleteActivity(pathID, activityID, idempotencyKey),
      )),
    );
    if (activityDeletionTarget.current !== identity) return;
    if (mutation.kind === 'busy') return;
    if (mutation.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(mutation.cause) ? mutation.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        await handleSessionFailure(failure, currentSession, 'profile', {
          current: () => activityDeletionTarget.current === identity,
        });
      } else {
        setActivityDeletionRetryable(classifySessionFailure(failure).retryable);
        setActivityDeletionErrorKey(classifySessionFailure(failure).retryable
          ? 'pathDetails.deleteRetryableError'
          : 'pathDetails.deleteFailed');
      }
      if (activityDeletionTarget.current === identity) setActivityDeletionBusy(false);
      return;
    }
    if (mutation.kind !== 'applied') {
      if (activityDeletionTarget.current === identity) setActivityDeletionBusy(false);
      return;
    }

    const latestDestination = notificationLifecycleState.current.destination;
    if (latestDestination?.kind !== 'home' || latestDestination.profile.id !== ownerID) return;
    const timer = latestDestination.profile.timers[pathID] ?? { running: false, accumulatedSeconds: 0 };
    const next = applyLoadedActivityDeletionResult(
      {
        activities: activityHistoryRef.current,
        comments: practiceCommentPage.current,
        feed: socialFeedPage.current,
        members: pathMembersRef.current,
        notifications: notificationHistoryRef.current,
        pathMemberActivities: pathMemberActivitiesRef.current,
        selectedMember: selectedPathMemberRef.current,
        timer,
      },
      mutation,
      { activityId: activityID, ownerId: ownerID, pathId: pathID },
    );
    const removedFeedEventIDs = new Set(next.removedFeedEventIds);
    pathMemberListOperations.invalidate();
    pathMemberActivityOperations.invalidate();
    pathDetailOperations.invalidate();
    socialFeedOperations.invalidate();
    socialFeedTarget.current = null;
    for (const eventID of removedFeedEventIDs) {
      socialFeedReactionOperations.get(eventID)?.invalidate();
      socialFeedReactionOperations.delete(eventID);
      socialFeedReactionRevisions.delete(eventID);
    }
    notificationRefreshOperations.invalidate();
    notificationTarget.current = null;
    setNotificationsBusy(false);
    setNotificationsRefreshing(false);
    for (const owner of practiceCommentMutationOperations.values()) owner.invalidate();
    practiceCommentMutationOperations.clear();
    practiceCommentMutationRetries.clear();
    practiceCommentMutationAdmissions.clear();
    setActivityHistory([...next.activities]);
    setPathMemberActivities([...next.pathMemberActivities]);
    setPathMembers([...next.members]);
    setSelectedPathMember(next.selectedMember);
    socialFeedPage.current = next.feed;
    setSocialFeed((current) => ({
      ...current,
      interactionNoticeEventID: current.interactionNoticeEventID && removedFeedEventIDs.has(current.interactionNoticeEventID)
        ? undefined
        : current.interactionNoticeEventID,
      interactionNoticeKey: current.interactionNoticeEventID && removedFeedEventIDs.has(current.interactionNoticeEventID)
        ? undefined
        : current.interactionNoticeKey,
      items: next.feed.items,
      loadingMore: false,
      refreshing: false,
    }));
    practiceCommentPage.current = next.comments;
    if (
      (practiceCommentTarget.current && removedFeedEventIDs.has(practiceCommentTarget.current.eventID)) ||
      (practiceCommentHistoryTarget.current && removedFeedEventIDs.has(practiceCommentHistoryTarget.current.eventID)) ||
      (practiceCommentHeartRosterTarget.current && removedFeedEventIDs.has(practiceCommentHeartRosterTarget.current.eventID))
    ) {
      practiceCommentOperations.invalidate();
      practiceCommentHistoryOperations.invalidate();
      practiceCommentHeartRosterOperations.invalidate();
      practiceCommentTarget.current = null;
      practiceCommentHistoryTarget.current = null;
      practiceCommentHeartRosterTarget.current = null;
      practiceCommentHistoryPage.current = null;
      practiceCommentHeartRosterPage.current = null;
      setPracticeComments(null);
      setPracticeCommentHeartRoster(null);
    }
    if (
      socialFeedActivityTarget.current?.activityID === activityID ||
      (socialFeedActivity && removedFeedEventIDs.has(socialFeedActivity.event.id))
    ) {
      socialFeedActivityOperations.invalidate();
      socialFeedActivityTarget.current = null;
      setSocialFeedActivity(null);
    }
    setNotificationHistory(next.notifications);
    const nextUnreadCount = next.notifications.unreadCount;
    setNotificationUnreadCount(nextUnreadCount);
    void setNativeNotificationBadge(nextUnreadCount).catch(() => undefined);
    setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
      ? {
          ...current,
          profile: {
            ...current.profile,
            timers: {
              ...current.profile.timers,
              [pathID]: {
                ...current.profile.timers[pathID],
                running: current.profile.timers[pathID]?.running ?? false,
                accumulatedSeconds: next.timer.accumulatedSeconds,
                intervalProgress: next.timer.intervalProgress,
              },
            },
          },
        }
      : current);
    await refreshHomeOrganizationPath(pathID, currentSession, ownerID);
    allowNativeChildRouteDismissal(activityDetailRouteKey(pathID, activityID));
    await AccessibilityInfo.announceForAccessibility(i18n.t('pathDetails.deleted'));
    router.back();
  }

  async function activate(
    next: Session,
    renewable: boolean,
    ticket: SessionOperationTicket,
    sourceSessionToken?: string,
  ) {
    const activeBeforeAdoption = notificationLifecycleState.current;
    const retainsOwnedHome = activeBeforeAdoption.destination?.kind === 'home' &&
      activeBeforeAdoption.session?.token === sourceSessionToken &&
      homeProjectionSessionToken.current === sourceSessionToken;
    await activatePersistedMobileSession({
      credential: next,
      renewable,
      current: ticket.current,
      adopt: (credential, canRenew) => {
        if (pathCreationTarget.current && !pathCreationTarget.current.sessionTokens.includes(credential.token)) {
          if (retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            activeBeforeAdoption.destination.profile.id === pathCreationTarget.current.ownerID) {
            pathCreationTarget.current = rotatePathCreationTarget(pathCreationTarget.current, credential.token);
          } else {
            cancelPathCreation();
          }
        }
        if (pathRouteTarget.current && !pathRouteTarget.current.sessionTokens.includes(credential.token)) {
          if (retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            activeBeforeAdoption.destination.profile.id === pathRouteTarget.current.ownerID) {
            pathRouteTarget.current = rotatePathRouteTarget(
              pathRouteTarget.current,
              activeBeforeAdoption.destination.profile.id,
              credential.token,
            );
          } else {
            resetPathDetail();
          }
        }
        if (pathDetailTarget.current && !pathDetailTarget.current.sessionTokens.includes(credential.token)) {
          if (retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            activeBeforeAdoption.destination.profile.id === pathDetailTarget.current.ownerID) {
            pathDetailTarget.current = rotatePathDetailTarget(
              pathDetailTarget.current,
              activeBeforeAdoption.destination.profile.id,
              credential.token,
            );
          } else {
            resetPathDetail();
          }
        }
        if (pathMemberTarget.current && !pathMemberTarget.current.sessionTokens.includes(credential.token)) {
          if (retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            activeBeforeAdoption.destination.profile.id === pathMemberTarget.current.ownerID) {
            pathMemberTarget.current = rotatePathMemberTarget(
              pathMemberTarget.current,
              activeBeforeAdoption.destination.profile.id,
              credential.token,
            );
          } else {
            closePathMembersRoute();
          }
        }
        if (pathNudgePreferenceTarget.current &&
          !pathNudgePreferenceTarget.current.sessionTokens.includes(credential.token)) {
          if (retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            activeBeforeAdoption.destination.profile.id === pathNudgePreferenceTarget.current.ownerID) {
            pathNudgePreferenceTarget.current = rotatePathNudgePreferenceTarget(
              pathNudgePreferenceTarget.current,
              activeBeforeAdoption.destination.profile.id,
              credential.token,
            );
          } else {
            closePathNudgeSettings(true);
          }
        }
        if (onboardingActivations.ownedBy(ticket) || onboardingHomeRecovery?.sessionToken === credential.token) {
          setOnboardingHomeRecovery({ sessionToken: credential.token, status: 'loading' });
        } else {
          invalidateOnboardingActivation();
        }
        if (credential.nextAction === 'home' && !retainsOwnedHome) {
          homeProjectionSessionToken.current = null;
          setDestination(null);
          setHomeRecovery({ sessionToken: credential.token, status: 'loading' });
        } else if (credential.nextAction === 'home') {
          homeProjectionSessionToken.current = credential.token;
        } else {
          homeProjectionSessionToken.current = null;
          setHomeRecovery(null);
        }
        if (activityDeletionTarget.current && activityDeletionTarget.current.sessionToken !== credential.token) {
          activityDeletionOperations.invalidate();
          activityDeletionTarget.current = null;
          setActivityDeletionBusy(false);
          setActivityDeletionErrorKey(null);
          setActivityDeletionRetryable(false);
        }
        if (pathLeaveTarget.current && pathLeaveTarget.current.session.token !== credential.token) resetPathLeave();
        if (pathArchiveTarget.current && !pathArchiveTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathArchiveTarget.current, activeBeforeAdoption.destination.profile.id, credential))) resetPathArchive();
        if (pathDeletionTarget.current && !pathDeletionTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathDeletionTarget.current, activeBeforeAdoption.destination.profile.id, credential))) resetPathDeletion();
        if (pathRenameTarget.current && !pathRenameTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathRenameTarget.current, activeBeforeAdoption.destination.profile.id, credential))) resetPathRename();
        if (goalManagementTarget.current && !goalManagementTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(goalManagementTarget.current, activeBeforeAdoption.destination.profile.id, credential))) resetGoalManagement();
        if (pathVisibilityTarget.current && !pathVisibilityTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathVisibilityTarget.current, activeBeforeAdoption.destination.profile.id, credential))) resetPathVisibilityState();
        if (pathVisibilityConflict.current && !pathVisibilityConflict.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathVisibilityConflict.current, activeBeforeAdoption.destination.profile.id, credential))) resetPathVisibilityState();
        if (ownershipTransferTarget.current && !ownershipTransferTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(ownershipTransferTarget.current, activeBeforeAdoption.destination.profile.id, credential))) resetOwnershipTransfer();
        if (pathManagementHandoffTarget.current && !pathManagementHandoffTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathManagementHandoffTarget.current, activeBeforeAdoption.destination.profile.id, credential))) {
          pathManagementHandoffTarget.current = null;
        }
        if (pathVisibilityConfirmationTarget.current && !pathVisibilityConfirmationTarget.current.sessionTokens.includes(credential.token) &&
          !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
            rotatePathAdministrationTarget(pathVisibilityConfirmationTarget.current, activeBeforeAdoption.destination.profile.id, credential))) {
          pathVisibilityConfirmationTarget.current = null;
        }
        const activeNotificationTargets = [notificationTarget, notificationMutationTarget, notificationSettingsTarget];
        if (activeNotificationTargets.some(({ current }) => current && !current.sessionTokens.includes(credential.token))) {
          const ownerID = activeBeforeAdoption.destination?.kind === 'home'
            ? activeBeforeAdoption.destination.profile.id
            : null;
          if (retainsOwnedHome && ownerID &&
            activeNotificationTargets.every(({ current }) => !current || current.ownerID === ownerID)) {
            for (const target of activeNotificationTargets) {
              if (target.current) {
                target.current = rotateNotificationSessionTarget(target.current, ownerID, credential);
              }
            }
          } else {
            resetNotifications();
          }
        }
        if ([...settingsOperationTargets.current.values()].some(
          (target) => !target.sessionTokens.includes(credential.token),
        )) {
          const ownerID = activeBeforeAdoption.destination?.kind === 'home'
            ? activeBeforeAdoption.destination.profile.id
            : null;
          if (retainsOwnedHome && ownerID && [...settingsOperationTargets.current.values()].every(
            (target) => target.ownerID === ownerID,
          )) {
            for (const [intentKey, target] of settingsOperationTargets.current) {
              settingsOperationTargets.current.set(
                intentKey,
                rotateNotificationSessionTarget(target, ownerID, credential),
              );
            }
          } else {
            resetSettingsOperations();
          }
        }
        if (pendingInvitationsTarget.current &&
          !pendingInvitationsTarget.current.sessionTokens.includes(credential.token)) {
          const ownerID = activeBeforeAdoption.destination?.kind === 'home'
            ? activeBeforeAdoption.destination.profile.id
            : null;
          if (retainsOwnedHome && ownerID && pendingInvitationsTarget.current.ownerID === ownerID) {
            pendingInvitationsTarget.current = rotateNotificationSessionTarget(
              pendingInvitationsTarget.current,
              ownerID,
              credential,
            );
          } else {
            resetInvitations();
          }
        }
        if (
          (invitationTarget.current && !invitationTarget.current.sessionTokens.includes(credential.token) &&
            !(retainsOwnedHome && activeBeforeAdoption.destination?.kind === 'home' &&
              rotatePathAdministrationTarget(invitationTarget.current, activeBeforeAdoption.destination.profile.id, credential))) ||
          [...invitationAcceptanceTargets.current.values()].some((target) => target.session.token !== credential.token) ||
          [...invitationRejectionTargets.current.values()].some((target) => target.session.token !== credential.token)
        ) resetInvitations();
        const socialTargets = [
          socialProfileSearchTarget,
          socialProfileDetailTarget,
          socialFollowRequestTarget,
          socialActiveFollowingTarget,
          socialFeedTarget,
          socialDeepEventTarget,
        ];
        if (socialTargets.some(({ current }) => current && !current.sessionTokens.includes(credential.token))) {
          const ownerID = activeBeforeAdoption.destination?.kind === 'home'
            ? activeBeforeAdoption.destination.profile.id
            : null;
          if (retainsOwnedHome && ownerID && socialTargets.every(({ current }) => !current || current.ownerID === ownerID)) {
            for (const target of socialTargets) {
              if (target.current) target.current = rotateSocialSessionTarget(target.current, ownerID, credential);
            }
            for (const [eventID, target] of socialFeedReactionTargets) {
              socialFeedReactionTargets.set(eventID, rotateSocialSessionTarget(target, ownerID, credential));
            }
          } else {
            resetSocialProfileDiscovery();
          }
        }
        const deepSocialTargets = [
          practiceCommentTarget,
          practiceCommentHistoryTarget,
          practiceCommentHeartRosterTarget,
          socialFeedActivityTarget,
        ];
        if (deepSocialTargets.some(({ current }) => current && (
          'sessionTokens' in current
            ? !current.sessionTokens.includes(credential.token)
            : current.session.token !== credential.token
        ))) {
          const ownerID = activeBeforeAdoption.destination?.kind === 'home'
            ? activeBeforeAdoption.destination.profile.id
            : null;
          if (retainsOwnedHome && ownerID && deepSocialTargets.every(({ current }) => !current || current.ownerID === ownerID)) {
            if (practiceCommentTarget.current) practiceCommentTarget.current = {
              ...practiceCommentTarget.current,
              ...rotateSocialSessionTarget(practiceCommentTarget.current, ownerID, credential),
            };
            if (practiceCommentHistoryTarget.current) practiceCommentHistoryTarget.current = {
              ...practiceCommentHistoryTarget.current,
              ...rotateSocialSessionTarget(practiceCommentHistoryTarget.current, ownerID, credential),
            };
            if (practiceCommentHeartRosterTarget.current) practiceCommentHeartRosterTarget.current = {
              ...practiceCommentHeartRosterTarget.current,
              ...rotateSocialSessionTarget(practiceCommentHeartRosterTarget.current, ownerID, credential),
            };
            if (socialFeedActivityTarget.current) socialFeedActivityTarget.current = { ...socialFeedActivityTarget.current, session: credential };
            socialFeedActivityOperations.invalidate();
            socialFeedActivityTarget.current = null;
            setSocialFeedActivity(null);
          } else {
            resetSocialProfileDiscovery();
          }
        }
        setSessionRenewable(canRenew); setSession(credential);
      },
      loadHome: (credential) => loadMobileHomeProfile(apiURL, credential),
      loadOnboarding: async (credential) => createMobileOnboardingDraft(
        await loadMobileOnboardingProfile(apiURL, credential),
        deviceOnboardingDefaults,
      ),
      online: (nextDestination) => {
        if (nextDestination.kind === 'home') {
          setOnboardingHomeRecovery(null);
          homeProjectionSessionToken.current = next.token;
        } else {
          homeProjectionSessionToken.current = null;
        }
        setHomeRecovery(null);
        const nextOwnerID = nextDestination.kind === 'home' ? nextDestination.profile.id : null;
        if (goalManagementOwnerID.current && goalManagementOwnerID.current !== nextOwnerID) resetGoalManagement();
        if (manualOwnerID.current && manualOwnerID.current !== nextOwnerID) resetManualActivity();
        if (pathDetailOwnerID.current && pathDetailOwnerID.current !== nextOwnerID) resetPathDetail();
        if (activityDeletionTarget.current && activityDeletionTarget.current.ownerId !== nextOwnerID) {
          activityDeletionOperations.invalidate();
          activityDeletionTarget.current = null;
          setActivityDeletionBusy(false);
          setActivityDeletionErrorKey(null);
          setActivityDeletionRetryable(false);
        }
        if (ownershipTransferTarget.current && ownershipTransferTarget.current.ownerID !== nextOwnerID) resetOwnershipTransfer();
        if (
          (invitationTarget.current && invitationTarget.current.ownerID !== nextOwnerID) ||
          (pendingInvitationsTarget.current && pendingInvitationsTarget.current.ownerID !== nextOwnerID) ||
          [...invitationAcceptanceTargets.current.values()].some((target) => target.ownerID !== nextOwnerID) ||
          [...invitationRejectionTargets.current.values()].some((target) => target.ownerID !== nextOwnerID)
        ) resetInvitations();
        if (notificationTarget.current && notificationTarget.current.ownerID !== nextOwnerID) resetNotifications();
        if ([...settingsOperationTargets.current.values()].some((target) => target.ownerID !== nextOwnerID)) {
          resetSettingsOperations();
        }
        if ((socialProfileSearchTarget.current && socialProfileSearchTarget.current.ownerID !== nextOwnerID) ||
          (socialProfileDetailTarget.current && socialProfileDetailTarget.current.ownerID !== nextOwnerID) ||
          (socialFollowRequestTarget.current && socialFollowRequestTarget.current.ownerID !== nextOwnerID) ||
          (socialActiveFollowingTarget.current && socialActiveFollowingTarget.current.ownerID !== nextOwnerID) ||
          (socialFeedTarget.current && socialFeedTarget.current.ownerID !== nextOwnerID) ||
          (socialFeedActivityTarget.current && socialFeedActivityTarget.current.ownerID !== nextOwnerID)) {
          resetSocialProfileDiscovery();
        }
        if (nextDestination.kind !== 'home') setCreatingPath(false);
        setDestination(nextDestination); setAccessState('authenticated_online'); setRetryAttempt(0); setErrorKey(null);
      },
    });
  }

  async function synchronizePushPermission(
    requestPermission: boolean,
  ): Promise<PushPermission> {
    const current = notificationLifecycleState.current;
    if (!current.session || current.destination?.kind !== 'home') {
      throw new Error('push_registration_session_missing');
    }
    const coordinator = pushRegistrationCoordinator.current;
    if (!coordinator) {
      return requestPermission
        ? requestNativePushPermission(i18n.t('notification.settings.channel'))
        : getNativePushPermission();
    }
    const ownerID = current.destination.profile.id;
    pushCredentials.current.set(ownerID, current.session.token);
    return coordinator.synchronize(ownerID, { requestPermission });
  }

  async function deregisterPushSession(currentSession: Session | null) {
    const coordinator = pushRegistrationCoordinator.current;
    const currentDestination = notificationLifecycleState.current.destination;
    if (!coordinator || !currentSession || currentDestination?.kind !== 'home') return;
    const ownerID = currentDestination.profile.id;
    pushCredentials.current.set(ownerID, currentSession.token);
    await coordinator.signOut(ownerID);
  }

  function ownsSignOutResolution(ownerID: string, currentSession: Session): boolean {
    const current = notificationLifecycleState.current;
    return current.session === currentSession &&
      current.destination?.kind === 'home' &&
      current.destination.profile.id === ownerID;
  }

  function applyOwnedTimerState(
    ownerID: string,
    currentSession: Session,
    pathID: string,
    state: TimerState,
  ): boolean {
    const current = notificationLifecycleState.current;
    if (current.session !== currentSession || current.destination?.kind !== 'home' ||
      current.destination.profile.id !== ownerID) return false;
    const nextDestination = {
      ...current.destination,
      profile: {
        ...current.destination.profile,
        timers: { ...current.destination.profile.timers, [pathID]: state },
      },
    };
    commitTimerProjectionBeforeRender(
      nextDestination,
      (next) => { notificationLifecycleState.current = { ...current, destination: next }; },
      setDestination,
    );
    return true;
  }

  async function resolveTimerSignOut(choice: SignOutTimerChoice): Promise<SignOutPresentationResult> {
    await timerMutationBarrier.blockAndDrain();
    let signOutCompleted = false;
    try {
      const current = notificationLifecycleState.current;
      if (!current.session || current.destination?.kind !== 'home') return { kind: 'superseded' };
      const currentSession = current.session;
      const ownerID = current.destination.profile.id;
      const runningEntries = Object.entries(current.destination.profile.timers)
        .filter(([, state]) => state.running);
      if (runningEntries.length === 0) {
        if (!ownsSignOutResolution(ownerID, currentSession)) return { kind: 'superseded' };
        signOutCompleted = await clearSession(null, 'authentication_required', { ownerID, session: currentSession });
        return signOutCompleted ? { kind: 'signed_out' } : { kind: 'superseded' };
      }
    if (choice === 'confirmed_no_timers') {
      return { kind: 'choice_required', runningTimerCount: runningEntries.length };
    }
    const snapshots: RunningTimerSnapshot[] = [];
    const timerIDs = new Map<string, string>();
    for (const [pathID, state] of runningEntries) {
      if (!state.timer || state.timer.pathId !== pathID || !Number.isFinite(Date.parse(state.timer.startedAt))) {
        return { kind: 'failed' };
      }
      snapshots.push({
        accumulatedSeconds: state.accumulatedSeconds,
        pathId: pathID,
        startedAt: state.timer.startedAt,
      });
      timerIDs.set(pathID, state.timer.id);
    }
    const resolution = signOutTimerResolutions.current.begin(snapshots);
    const decision = choice === 'keep_running'
      ? resolution.keepRunning()
      : await resolution.stopAndSave(async (timer) => {
          if (!ownsSignOutResolution(ownerID, currentSession)) {
            signOutTimerResolutions.current.invalidate();
            throw new Error('sign_out_resolution_superseded');
          }
          const timerID = timerIDs.get(timer.pathId);
          if (!timerID) throw new Error('sign_out_timer_missing');
          const result = await timerOperations.stop(timer.pathId, timerID, (idempotencyKey) =>
            validateSessionCredential<TimerStopResult>(currentSession, async (credential) => generatedResponse(
              await createSessionApiClient(apiURL, () => credential.token).stopTimer(
                timer.pathId,
                timerID,
                idempotencyKey,
              ),
            )),
          );
          if (result.kind === 'failed') throw result.cause;
          if (result.kind === 'superseded' || !ownsSignOutResolution(ownerID, currentSession)) {
            signOutTimerResolutions.current.invalidate();
            throw new Error('sign_out_resolution_superseded');
          }
          const presentation = timerMutationPresentation(result.state);
          if (!applyOwnedTimerState(ownerID, currentSession, timer.pathId, presentation.state)) {
            signOutTimerResolutions.current.invalidate();
            throw new Error('sign_out_resolution_superseded');
          }
          return { pathId: timer.pathId, running: presentation.state.running };
        });
    if (!decision.authorizeSignOut) {
      return { kind: decision.kind === 'stop_failed' ? 'failed' : 'superseded' };
    }
    if (!ownsSignOutResolution(ownerID, currentSession)) {
      signOutTimerResolutions.current.invalidate();
      return { kind: 'superseded' };
    }
      signOutCompleted = await clearSession(null, 'authentication_required', { ownerID, session: currentSession });
      return signOutCompleted ? { kind: 'signed_out' } : { kind: 'superseded' };
    } finally {
      if (!signOutCompleted) timerMutationBarrier.unblock();
    }
  }

  async function clearSession(
    message: MessageKey | null = null,
    state: SessionAccessState = 'authentication_required',
    expected?: Readonly<{ ownerID: string; session: Session }>,
  ): Promise<boolean> {
    const disposedSession = expected?.session ?? session;
    sessionOperations.invalidate();
    invalidateOnboardingActivation();
    await deregisterPushSession(disposedSession).catch(() => undefined);
    const active = notificationLifecycleState.current;
    if (expected
      ? !ownsSignOutResolution(expected.ownerID, expected.session)
      : active.session !== disposedSession) return false;
    cancelPathCreation();
    resetTimerPresentation();
    resetGoalManagement();
    resetManualActivity();
    resetPathDetail();
    resetInvitations();
    resetOwnershipTransfer();
    resetNotifications();
    resetSettingsOperations();
    void setNativeNotificationBadge(0).catch(() => false);
    resetSocialProfileDiscovery();
    await disposeMobileSession(disposedSession, state, {
      discardStored: () => serializedSessionStorage.discard(),
      transition: (next) => {
        homeProjectionSessionToken.current = null;
        setSession(null); setRetryAttempt(0); setDestination(null); setCreatingPath(false); setPathSubmitting(false); setActivatingOnboarding(false); setHomeRecovery(null); setAccessState(next.accessState);
        setErrorKey(next.storageUnreadable ? 'errors.localSessionUnreadable' : message);
      },
      revoke: async (token) => { await createSessionApiClient(apiURL, () => token).revoke(); },
    });
    timerMutationBarrier.unblock();
    return true;
  }
  async function beginSignIn() {
    setErrorKey(null);
    try {
      await serializedSessionStorage.ready();
    } catch (cause) {
      const failure: SessionFailure = isSessionFailure(cause)
        ? cause
        : { kind: 'local_storage', reason: 'malformed' };
      const decision = classifySessionFailure(failure);
      setAccessState(decision.state);
      setErrorKey(localizedFailure(failure, 'errors.signInFailed'));
      return;
    }
    try { await providerSignIn.begin(); }
    catch { setErrorKey('errors.signInFailed'); }
  }
  async function handleSessionFailure(
    cause: unknown,
    current: Session,
    operation: SessionRecoveryOperation = 'refresh',
    ticket?: SessionOperationTicket,
  ) {
    if (ticket && !ticket.current()) return;
    const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
    if (classifySessionFailure(failure).discardCredential) {
      sessionOperations.invalidate();
      invalidateOnboardingActivation();
      await deregisterPushSession(current).catch(() => undefined);
      resetTimerPresentation();
      resetManualActivity();
      resetPathDetail();
      resetInvitations();
      resetNotifications();
      resetSettingsOperations();
      resetSocialProfileDiscovery();
    }
    await applyMobileSessionFailure(failure, current, {
      discardStored: () => serializedSessionStorage.discard(),
      transition: (next) => {
        setSession(next.session); setAccessState(next.accessState);
        if (!next.session) { homeProjectionSessionToken.current = null; resetManualActivity(); resetPathDetail(); resetInvitations(); resetNotifications(); resetSettingsOperations(); resetSocialProfileDiscovery(); setRetryAttempt(0); setDestination(null); setActivatingOnboarding(false); setOnboardingHomeRecovery(null); setHomeRecovery(null); }
        else if (!next.retryable) { setSessionRenewable(false); setRetryAttempt(0); }
        else {
          setRetryOperation(operation);
          setRetryAttempt((attempt) => attempt + 1);
        }
        if (next.session) setOnboardingHomeRecovery((recovery) => recovery?.sessionToken === current.token
          ? { ...recovery, status: failure.kind === 'network' ? 'offline' : 'error' }
          : recovery);
        if (
          next.session?.nextAction === 'home' &&
          !(notificationLifecycleState.current.destination?.kind === 'home' &&
            homeProjectionSessionToken.current === current.token)
        ) {
          setHomeRecovery((recovery) => recovery?.sessionToken === current.token || current.token === next.session?.token
            ? { sessionToken: current.token, status: failure.kind === 'network' ? 'offline' : 'error' }
            : recovery);
        }
        if (next.retryable) setErrorKey(null);
        else setErrorKey(next.storageUnreadable ? 'errors.localSessionUnreadable' : failureMessage(failure));
      },
      revoke: async (token) => { await createSessionApiClient(apiURL, () => token).revoke(); },
    });
  }

  async function handleFeatureSessionFailure(
    cause: unknown,
    current: Session,
    ticket?: SessionOperationTicket,
  ) {
    const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
    if (!shouldTransitionMobileSessionForFeatureFailure(failure)) return;
    await handleSessionFailure(failure, current, 'profile', ticket);
  }

  useEffect(() => {
    if (destination?.kind !== 'home') return;
    const snapshot = destination.profile.homePreferences;
    const allPathIDs = [...destination.profile.paths, ...destination.profile.archivedPaths].map(({ id }) => id);
    const known = new Set(snapshot.manualPathIds);
    setHomePreferences({
      order: snapshot.orderMethod,
      pinnedPathIDs: snapshot.pinnedPathIds,
      manualPathIDs: [...snapshot.manualPathIds, ...allPathIDs.filter((id) => !known.has(id))],
    });
  }, [
    destination?.kind,
    destination?.kind === 'home' ? destination.profile.homePreferences.revision : null,
    destination?.kind === 'home'
      ? [...destination.profile.paths, ...destination.profile.archivedPaths].map(({ id }) => id).join('\u0000')
      : null,
  ]);

  useEffect(() => {
    setHomeFilter('all');
    setHomeArrangementOpen(false);
    setHomePreferenceErrorKey(null);
    setHomePreferenceBusy(false);
    homePreferenceOperations.cancel();
  }, [destination?.kind === 'home' ? destination.profile.id : null]);

  async function updateHomePreferences(requested: HomePreferences) {
    if (!session || destination?.kind !== 'home' || homePreferenceBusy) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const current = destination.profile.homePreferences;
    const requestedSnapshot: HomePreferenceSnapshot = {
      ...current,
      manualPathIds: requested.manualPathIDs,
      orderMethod: requested.order,
      pinnedPathIds: requested.pinnedPathIDs,
    };
    setHomePreferenceBusy(true);
    setHomePreferenceErrorKey(null);
    const result = await homePreferenceOperations.submit(current, requestedSnapshot, (body, idempotencyKey) =>
      validateSessionCredential<HomePreferenceSnapshot>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).updateHomePreferences(body, idempotencyKey),
      )),
    );
    if (result.kind === 'superseded') { setHomePreferenceBusy(false); return; }
    setHomePreferenceBusy(false);
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) await handleSessionFailure(failure, currentSession);
      else setHomePreferenceErrorKey(failureMessage(failure));
      return;
    }
    setHomePreferences({
      order: result.preferences.orderMethod,
      pinnedPathIDs: result.preferences.pinnedPathIds,
      manualPathIDs: result.preferences.manualPathIds,
    });
    setDestination((currentDestination) => currentDestination?.kind === 'home' && currentDestination.profile.id === ownerID
      ? { ...currentDestination, profile: { ...currentDestination.profile, homePreferences: result.preferences } }
      : currentDestination);
  }

  async function refreshHomeOrganizationPath(
    pathID: string,
    currentSession: Session,
    ownerID: string,
  ): Promise<void> {
    const isCurrent = () => {
      const active = notificationLifecycleState.current;
      return active.session?.token === currentSession.token &&
        active.destination?.kind === 'home' && active.destination.profile.id === ownerID;
    };
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).path(pathID),
      );
      if (!isCurrent()) return;
      if (response.status === 404) {
        setDestination((current) => isCurrent() && current?.kind === 'home' && current.profile.id === ownerID
          ? { ...current, profile: applyRefreshedMobilePath(current.profile, pathID, null) }
          : current);
        if (isCurrent() && foregroundTargetKey.current === `path:${pathID}`) {
          resetPathDetail();
          router.replace('/(tabs)/home');
        }
        return;
      }
      const projected = await validateSessionCredential<SessionPath>(
        currentSession,
        async () => response,
      );
      const admitted = projected.archivedAt
        ? admitMobileSessionPaths([], [projected])
        : admitMobileSessionPaths([projected], []);
      const refreshed = admitted.active[0] ?? admitted.archived[0];
      if (!refreshed) throw sessionFailureFromResponse(502);
      if (!isCurrent()) return;
      setDestination((current) => isCurrent() && current?.kind === 'home' && current.profile.id === ownerID
        ? { ...current, profile: applyRefreshedMobilePath(current.profile, pathID, refreshed) }
        : current);
    } catch (cause) {
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) await handleSessionFailure(failure, currentSession);
    }
  }

  useEffect(() => {
    if (accessState !== 'authenticated_offline') setOfflineStatusDismissed(false);
  }, [accessState]);

  useEffect(() => {
    (async () => {
      try {
        const ticket = sessionOperations.issue();
        await restoreStoredSession({
          read: () => serializedSessionStorage.read(),
          now: () => Date.now(),
          refreshLeadMs: sessionRefreshLeadMs,
          refresh: (current) => refreshSession(current, ticket),
          expiryAdvanced: sessionExpiryAdvanced,
          current: ticket.current,
          revokeSuperseded: revokeSupersededSession,
          activate: (current, renewable) => activate(current, renewable, ticket, current.token),
          handleFailure: (cause, current) => handleSessionFailure(cause, current, 'refresh', ticket),
          handleUnreadable: async () => {
            await clearSession('errors.localSessionUnreadable', 'local_session_unreadable');
          },
        });
      } finally { setReady(true); }
    })();
  }, []);

  useEffect(() => {
    const review = pendingInvitationAcceptanceReview;
    if (!review) return;
    const target = invitationAcceptanceTargets.current.get(review.invitationId);
    const pending = destination?.kind === 'home'
      ? destination.profile.pendingInvitations.items.find(
          (candidate) => candidate.invitation.id === review.invitationId,
        )
      : undefined;
    if (
      session &&
      destination?.kind === 'home' &&
      target?.session === session &&
      target.ownerID === destination.profile.id &&
      pending?.warning?.pathVisibility === review.warning.pathVisibility &&
      pending.warning.hasRetainedActivity === review.warning.hasRetainedActivity
    ) return;
    invitationAcceptOwner.cancel(review.invitationId);
    invitationAcceptanceTargets.current.delete(review.invitationId);
    setPendingInvitationAcceptanceReview(null);
  }, [destination, pendingInvitationAcceptanceReview, session]);

  useEffect(() => {
    let active = true;
    let liveElapsed: ReturnType<typeof setTimeout>;
    const tick = () => {
      liveElapsed = setTimeout(() => {
        if (!active) return;
        setNow(Date.now());
        tick();
      }, 1000);
    };
    tick();
    return () => { active = false; clearTimeout(liveElapsed); };
  }, []);

  useEffect(() => {
    if (!providerSignIn.identityToken && !providerSignIn.failed) return;
    (async () => {
      let acknowledged = false;
      const ticket = sessionOperations.issue();
      try {
        if (!providerSignIn.identityToken) throw new LocalizedError('errors.identityTokenMissing');
        await completeMobileSessionExchange({
          exchange: async () => {
            await serializedSessionStorage.ready();
            return exchangeSessionCredential(
              async () => generatedResponse(await createSessionApiClient(apiURL, () => null).exchange(providerSignIn.identityToken!)),
              async (credential) => serializedSessionStorage.persist(credential, ticket.current),
            );
          },
          current: ticket.current,
          acknowledge: () => { providerSignIn.acknowledge(); acknowledged = true; },
          activate: (credential, renewable) => activate(credential, renewable, ticket),
          handleFailure: (cause, credential, operation) => handleSessionFailure(cause, credential, operation, ticket),
          revokeSuperseded: revokeSupersededSession,
        });
      } catch (cause) {
        if (!ticket.current()) return;
        if (session) await handleSessionFailure(cause, session, 'refresh', ticket);
        else { setAccessState('authentication_required'); setErrorKey(localizedFailure(cause, 'errors.signInFailed')); }
      }
      finally {
        if (!acknowledged) providerSignIn.acknowledge();
        setReady(true);
      }
    })();
  }, [providerSignIn.identityToken, providerSignIn.failed]);

  useEffect(() => {
    if (!session) return;
    if (onboardingHomeRecovery?.status === 'loading') return;
    if (homeRecovery?.status === 'loading') return;
    const untilExpiry = Date.parse(session.expiresAt) - Date.now();
    if (untilExpiry <= 0) { void clearSession('errors.sessionExpired'); return; }
    const delay = accessState === 'authenticated_offline' && retryAttempt > 0 ? sessionRetryDelay(retryAttempt, session.expiresAt) : sessionRenewable ? sessionRefreshDelay(session.expiresAt) : untilExpiry;
    const timer = setTimeout(() => {
      if (!sessionRenewable && (accessState !== 'authenticated_offline' || retryAttempt === 0)) { void clearSession('errors.sessionExpired'); return; }
      const ticket = sessionOperations.issue();
      void recoverMobileSession({
        mode: accessState === 'authenticated_offline' && retryAttempt > 0 ? retryOperation : 'refresh',
        credential: session,
        renewable: sessionRenewable,
        current: ticket.current,
        refresh: (current) => refreshSession(current, ticket),
        expiryAdvanced: sessionExpiryAdvanced,
        activate: (credential, renewable) => activate(credential, renewable, ticket, session.token),
        handleFailure: (cause, credential, operation) => handleSessionFailure(cause, credential, operation, ticket),
        revokeSuperseded: revokeSupersededSession,
      });
    }, delay);
    return () => clearTimeout(timer);
  }, [session?.token, session?.expiresAt, sessionRenewable, accessState, retryAttempt, retryOperation, onboardingHomeRecovery?.status, homeRecovery?.status]);

  useEffect(() => {
    if (!session || destination?.kind !== 'home') return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    let current = true;
    const currentContext = (): ForegroundNotificationContext | null => {
      const latest = notificationLifecycleState.current;
      return current && latest.session?.token === currentSession.token &&
        latest.destination?.kind === 'home' && latest.destination.profile.id === ownerID
        ? { sessionId: currentSession.token, targetKey: foregroundTargetKey.current, userId: ownerID }
        : null;
    };
    const foreground = createForegroundNotificationCoordinator({
      current: currentContext,
      resolve: async (notificationID, context) => {
        if (context.sessionId !== currentSession.token || context.userId !== ownerID) return null;
        const resolved = await resolvePushNotificationRecord(currentSession, ownerID, notificationID);
        return resolved ? {
          presentation: resolved.presentation,
          recipientUserId: ownerID,
          targetKey: notificationDestinationTargetKey(resolved.destination),
        } : null;
      },
      refreshHistory: async () => { await refreshPushNotificationHistory(currentSession, ownerID); },
      refreshRelevantTarget: async (targetKey) => {
        await refreshForegroundNotificationTarget(targetKey, currentSession, ownerID);
      },
    });
    const dispose = installNativeNotificationLifecycle({
      foreground: (notificationID) => foreground.request({
        notificationId: notificationID,
        sessionId: currentSession.token,
        userId: ownerID,
      }),
      resolve: (notificationID) => current
        ? resolvePushNotification(currentSession, ownerID, notificationID)
        : Promise.resolve(null),
      markRead: async (notificationID) => {
        if (!current) throw new Error('notification_session_changed');
        await markPushNotificationRead(currentSession, ownerID, notificationID);
      },
      navigate: (next) => {
        if (!current) return;
        setNotificationTapFeedbackKey(null);
        navigateFromPush(next);
      },
      unavailable: () => {
        if (current) setNotificationTapFeedbackKey('notification.itemUnavailable');
      },
      failure: () => {
        if (current) setNotificationsErrorKey('notification.error');
      },
    });
    return () => {
      current = false;
      setNotificationTapFeedbackKey(null);
      foreground.dispose();
      dispose();
    };
  }, [session?.token, destination?.kind, destination?.kind === 'home' ? destination.profile.id : null]);

  useEffect(() => {
    if (!session || destination?.kind !== 'home' || !pushRegistrationCoordinator.current) return;
    const ownerID = destination.profile.id;
    pushCredentials.current.set(ownerID, session.token);
    let active = true;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    const synchronize = async () => {
      try {
        await retryStoredPushDeregistration().catch(() => undefined);
        await pushRegistrationCoordinator.current?.synchronize(ownerID);
      } catch {
        const current = notificationLifecycleState.current;
        if (
          active
          && current.session?.token === session.token
          && current.destination?.kind === 'home'
        ) {
          setNotificationsErrorKey('notification.error');
          retryTimer = setTimeout(() => { void synchronize(); }, 30_000);
        }
      }
    };
    void synchronize();
    return () => {
      active = false;
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, [session?.token, destination?.kind, destination?.kind === 'home' ? destination.profile.id : null]);

  useEffect(() => {
    if (!pushProjectId) return;
    let active = true;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    const retryPendingDeregistration = async () => {
      try {
        await retryStoredPushDeregistration();
      } catch { /* The bounded poll retries transient cleanup failures. */ }
      if (active) {
        retryTimer = setTimeout(() => { void retryPendingDeregistration(); }, 30_000);
      }
    };
    void retryPendingDeregistration();
    return () => {
      active = false;
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, []);

  const updateOnboardingDisplayName = (displayName: string) => {
    if (onboardingHomeRecovery || onboardingActivations.blocked()) return;
    setDestination((current) => current && current.kind === 'onboarding'
      ? { ...current, profile: { ...current.profile, displayName } }
      : current);
  };

  const updateOnboardingDraft = (patch: Partial<MobileOnboardingDraft>) => {
    if (onboardingHomeRecovery || onboardingActivations.blocked()) return;
    setDestination((current) => current && current.kind === 'onboarding'
      ? { ...current, profile: { ...current.profile, ...patch } }
      : current);
  };

  const updateOnboardingUsername = (usernameSuggestion: string) => {
    if (onboardingHomeRecovery || onboardingActivations.blocked()) return;
    setErrorKey((current) => current === 'onboarding.usernameUnavailable' ? null : current);
    setDestination((current) => current && current.kind === 'onboarding'
      ? { ...current, profile: replaceMobileOnboardingUsername(current.profile, usernameSuggestion) }
      : current);
  };

  const reviewOnboardingUsername = (reviewed: boolean) => {
    if (onboardingHomeRecovery || onboardingActivations.blocked()) return;
    setDestination((current) => current && current.kind === 'onboarding' &&
      (!reviewed || validProfileUsername(current.profile.usernameSuggestion))
        ? { ...current, profile: { ...current.profile, usernameReviewed: reviewed } }
        : current);
  };

  async function continueCreatingNewAccount() {
    if (!session || session.nextAction !== 'duplicate_email_recovery' || destination?.kind !== 'duplicate_email_recovery' || decliningRecovery) return;
    const onboardingProfile = destination.profile;
    setDecliningRecovery(true);
    setErrorKey(null);
    const ticket = sessionOperations.issue();
    let serverDeclined = false;
    try {
      await continueMobileNewAccount({
        credential: session,
        current: ticket.current,
        decline: async () => {
          await validateSessionMutation(async () => generatedResponse(
            await createSessionApiClient(apiURL, () => session.token).declineDuplicateEmailRecovery(),
          ));
          serverDeclined = true;
        },
        persist: (replacement) => serializedSessionStorage.persist(replacement, ticket.current),
        activate: async (replacement) => {
          invalidateOnboardingActivation();
          setSession(replacement);
          setSessionRenewable(false);
          setDestination({ kind: 'onboarding', profile: onboardingProfile });
          setAccessState('authenticated_online');
        },
      });
    } catch (cause) {
      if (ticket.current()) {
        const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
        if (!serverDeclined && classifySessionFailure(failure).discardCredential) {
          await clearSession(failureMessage(failure), classifySessionFailure(failure).state);
        } else {
          setErrorKey(localizedFailure(failure, 'errors.restoreFailed'));
        }
      }
    } finally {
      if (ticket.current()) setDecliningRecovery(false);
    }
  }

  async function completeOnboarding() {
    if (!session || session.nextAction !== 'onboarding' || destination?.kind !== 'onboarding' ||
      onboardingActivations.blocked() ||
      errorKey === 'onboarding.usernameUnavailable') return;
    const draft = destination.profile;
    if (!canCompleteMobileOnboarding(draft)) return;
    if (!draft.profileVisibility || !draft.timeZone || !draft.firstDayOfWeek) return;
    const body: OnboardingActivationInput = {
      username: draft.usernameSuggestion,
      displayName: draft.displayName.trim(),
      profileVisibility: draft.profileVisibility,
      timeZone: draft.timeZone,
      firstDayOfWeek: draft.firstDayOfWeek,
      atLeast16: draft.atLeast16,
      termsAccepted: draft.termsAccepted,
      privacyAcknowledged: draft.privacyAcknowledged,
      communityGuidelinesAccepted: draft.communityGuidelinesAccepted,
      policyReviewToken: draft.policyReviewToken,
    };
    const ticket = sessionOperations.issue();
    const attempt = onboardingActivations.begin(session.token, ticket);
    setActivatingOnboarding(true);
    setErrorKey(null);
    try {
      const outcome = await completeMobileOnboardingActivation({
        current: ticket.current,
        request: async () => generatedResponse(
          await createSessionApiClient(apiURL, () => session.token).activateOnboarding(body),
        ),
        persist: (credential) => serializedSessionStorage.persist(credential, ticket.current),
        adopt: (credential) => activate(credential, true, ticket, session.token),
        handleAdoptionFailure: async (cause, credential) => {
          if (!ticket.current()) return false;
          await handleSessionFailure(cause, credential, 'profile', ticket);
          return true;
        },
        refreshReview: async () => {
          const current = await loadMobileOnboardingProfile(apiURL, session);
          return refreshMobileOnboardingPolicy(draft, current);
        },
        revokeSuperseded: revokeSupersededSession,
      });
      if (!ticket.current()) return;
      if (outcome.kind === 'policy_set_changed') {
        setDestination({ kind: 'onboarding', profile: outcome.profile });
        setErrorKey('onboarding.policySetChanged');
      } else if (outcome.kind === 'username_unavailable') {
        setErrorKey('onboarding.usernameUnavailable');
      }
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const decision = classifySessionFailure(failure);
      if (decision.discardCredential) await clearSession(failureMessage(failure), decision.state);
      else setErrorKey(localizedFailure(failure, 'errors.validationFailed'));
    } finally {
      onboardingActivations.release(attempt);
      if (ticket.current() && !onboardingActivations.blocked()) setActivatingOnboarding(false);
    }
  }

  async function retryOnboardingHome() {
    if (!session || session.nextAction !== 'home' ||
      onboardingHomeRecovery?.sessionToken !== session.token ||
      onboardingHomeRecovery.status === 'loading') return;
    setOnboardingHomeRecovery({ sessionToken: session.token, status: 'loading' });
    const ticket = sessionOperations.issue();
    await recoverMobileSession({
      mode: 'profile',
      credential: session,
      renewable: sessionRenewable,
      current: ticket.current,
      refresh: (current) => refreshSession(current, ticket),
      expiryAdvanced: sessionExpiryAdvanced,
      activate: (credential, renewable) => activate(credential, renewable, ticket, session.token),
      handleFailure: (cause, credential, operation) => handleSessionFailure(cause, credential, operation, ticket),
      revokeSuperseded: revokeSupersededSession,
    });
  }

  async function retryAuthenticatedHome() {
    if (
      !session ||
      session.nextAction !== 'home' ||
      destination?.kind === 'home' ||
      homeRecovery?.sessionToken !== session.token ||
      homeRecovery.status === 'loading'
    ) return;
    setHomeRecovery({ sessionToken: session.token, status: 'loading' });
    const ticket = sessionOperations.issue();
    await recoverMobileSession({
      mode: 'profile',
      credential: session,
      renewable: sessionRenewable,
      current: ticket.current,
      refresh: (current) => refreshSession(current, ticket),
      expiryAdvanced: sessionExpiryAdvanced,
      activate: (credential, renewable) => activate(credential, renewable, ticket, session.token),
      handleFailure: (cause, credential, operation) => handleSessionFailure(cause, credential, operation, ticket),
      revokeSuperseded: revokeSupersededSession,
    });
  }

  async function openPolicyLink(url: string) {
    setErrorKey(null);
    await openNativePolicyLink(url, () => setErrorKey('errors.temporarilyUnavailable'));
  }

  function cancelPathCreation() {
    pathCreation.cancel();
    pathCreationTarget.current = null;
    pathCreationBusy.current = false;
    setCreatingPath(false);
    setPathSubmitting(false);
    setPathName('');
    setPathVisibility('private');
    setPathGoalForm(initialPathGoalForm());
    setPathErrorKey(null);
  }

  function beginPathCreation() {
    const active = notificationLifecycleState.current;
    const profileVisibility = active.destination?.kind === 'home'
      ? active.destination.profile.profileVisibility
      : null;
    setPathVisibility(defaultPathCreationVisibility(profileVisibility));
    setCreatingPath(true);
    setPathErrorKey(null);
    setPathCreated(false);
  }

  function updatePathName(name: string) {
    setPathName(name);
    setPathErrorKey(null);
  }

  function updatePathVisibility(visibility: PathVisibility) {
    setPathVisibility(visibility);
    setPathErrorKey(null);
  }

  function updatePathGoalForm(update: Partial<PathGoalForm>) {
    setPathGoalForm((current) => ({ ...current, ...update }));
    setPathErrorKey(null);
  }

  async function loadMorePendingInvitations(cursor: string) {
    if (!session || destination?.kind !== 'home' || pendingInvitationsBusy || pendingInvitationsTarget.current) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = `notifications:invitations:${cursor}`;
    pendingInvitationsTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    setPendingInvitationsBusy(true);
    setPendingInvitationsErrorKey(null);
    try {
      const page = await loadPendingInvitationPage(currentSession, cursor);
      if (
        !ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession)
      ) return;
      setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
        ? {
            ...current,
            profile: {
              ...current.profile,
              pendingInvitations: mergePendingInvitationPage(current.profile.pendingInvitations, page, cursor),
            },
          }
        : current);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, () =>
        ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession));
      if (
        ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession)
      ) {
        setPendingInvitationsErrorKey(invitationFailureKey(cause));
      }
    } finally {
      if (
        ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession)
      ) {
        pendingInvitationsTarget.current = null;
        setPendingInvitationsBusy(false);
      }
    }
  }

  function ownsCurrentNotificationOperation(
    target: NotificationSessionTarget<Session> | null,
    ownerID: string,
    intentKey: string,
    requestSession: Session,
  ): boolean {
    const current = notificationLifecycleState.current;
    const currentOwnerID = current.destination?.kind === 'home' ? current.destination.profile.id : null;
    return ownsNotificationSessionTarget(target, ownerID, requestSession.token, intentKey) &&
      currentNotificationSessionCredential(target, currentOwnerID, intentKey, current.session) !== null;
  }

  async function handleNotificationOperationFailure(
    cause: unknown,
    requestSession: Session,
    current: () => boolean,
  ) {
    if (!current()) return;
    if (!shouldHandleSettingsOperationFailure(
      requestSession.token,
      notificationLifecycleState.current.session?.token,
    )) return;
    await handleFeatureSessionFailure(cause, requestSession, { current });
  }

  async function handleSettingsOperationFailure(
    cause: unknown,
    requestSession: Session,
    current: () => boolean,
  ) {
    if (!current()) return;
    if (!shouldHandleSettingsOperationFailure(
      requestSession.token,
      notificationLifecycleState.current.session?.token,
    )) return;
    await handleFeatureSessionFailure(cause, requestSession, { current });
  }

  async function openNotifications(navigate = true) {
    if (!session || destination?.kind !== 'home' || notificationsBusy || notificationTarget.current ||
      notificationMutationAdmission.current) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = 'notifications:history';
    const empty: NotificationHistoryState = { items: [], nextCursor: '', unreadCount: 0 };
    const ticket = notificationRefreshOperations.issue();
    notificationTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    setNotificationsOpen(true);
    if (navigate) {
      prepareNotificationRoute();
      router.push('/notifications');
    }
    setNotificationsBusy(true);
    setNotificationsErrorKey(null);
    try {
      const page = await loadNotificationHistoryPage(currentSession);
      if (
        !ticket.current() ||
        !ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) return;
      const history = mergeNotificationHistoryPage(empty, page, '');
      setNotificationHistory(history);
      setNotificationUnreadCount(history.unreadCount);
      void setNativeNotificationBadge(history.unreadCount).catch(() => undefined);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, () =>
        ticket.current() && ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession));
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) {
        setNotificationsErrorKey('notification.error');
      }
    } finally {
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) {
        notificationTarget.current = null;
        setNotificationsBusy(false);
      }
    }
  }

  async function refreshNotifications() {
    if (!session || destination?.kind !== 'home' || !notificationsOpen || notificationsBusy || notificationTarget.current ||
      notificationMutationAdmission.current) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = 'notifications:history';
    const ticket = notificationRefreshOperations.issue();
    notificationTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    setNotificationsBusy(true);
    setNotificationsRefreshing(true);
    setNotificationsErrorKey(null);
    try {
      const page = await loadNotificationHistoryPage(currentSession);
      if (
        !ticket.current() ||
        !ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) return;
      const history = mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: notificationHistory.unreadCount },
        page,
        '',
      );
      setNotificationHistory(history);
      setNotificationUnreadCount(history.unreadCount);
      void setNativeNotificationBadge(history.unreadCount).catch(() => undefined);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, () =>
        ticket.current() && ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession));
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) setNotificationsErrorKey('notification.error');
    } finally {
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) {
        notificationTarget.current = null;
        setNotificationsBusy(false);
        setNotificationsRefreshing(false);
      }
    }
  }

  async function openPendingInvitations(invitationID?: string, navigate = true) {
    if (!session || destination?.kind !== 'home' || pendingInvitationsBusy || pendingInvitationsTarget.current) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const invitationTarget = invitationID ?? '*';
    const intentKey = `notifications:invitations:${invitationTarget}`;
    pendingInvitationsTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    setFocusedInvitationID(invitationID ?? null);
    setInvitationsOpen(true);
    setPendingInvitationsBusy(true);
    setPendingInvitationsErrorKey(null);
    if (navigate) {
      prepareInvitationRoute();
      router.push({
        pathname: '/invitations',
        params: invitationID ? { invitationID } : {},
      });
    }
    try {
      const pendingInvitations = await loadPendingInvitationsThroughTarget(
        currentSession,
        invitationID,
        () => ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession),
      );
      if (
        !ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession)
      ) return;
      setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
        ? { ...current, profile: { ...current.profile, pendingInvitations } }
        : current);
      if (
        invitationID &&
        !pendingInvitations.items.some((item) => item.invitation.id === invitationID)
      ) setFocusedInvitationID(null);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, () =>
        ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession));
      if (
        ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession)
      ) setPendingInvitationsErrorKey(invitationFailureKey(cause));
    } finally {
      if (
        ownsCurrentNotificationOperation(pendingInvitationsTarget.current, ownerID, intentKey, currentSession)
      ) {
        pendingInvitationsTarget.current = null;
        setPendingInvitationsBusy(false);
      }
    }
  }

  function retryNotificationJourney(intent: NotificationJourneyIntent) {
    if (intent.kind === 'notifications') {
      void openNotifications(false);
      return;
    }
    if (intent.kind === 'invitations') {
      void openPendingInvitations(undefined, false);
      return;
    }
    void retryAuthenticatedHome();
  }

  function closeNotificationsRoute() {
    notificationRefreshOperations.invalidate();
    notificationTarget.current = null;
    setNotificationsOpen(false);
    setNotificationsBusy(false);
    setNotificationsRefreshing(false);
  }

  function closeInvitationsRoute() {
    setInvitationsOpen(false);
    setFocusedInvitationID(null);
    setAcceptedInvitation(null);
    if (pendingInvitationAcceptanceReview) cancelPendingInvitationAcceptance();
  }

  async function refreshPushNotificationHistory(
    currentSession: Session,
    ownerID: string,
  ): Promise<NotificationHistoryPage | null> {
    if (notificationTarget.current || notificationMutationAdmission.current) return null;
    const ticket = notificationRefreshOperations.issue();
    const page = await loadNotificationHistoryPage(currentSession);
    const current = notificationLifecycleState.current;
    if (
      !ticket.current() ||
      current.session?.token !== currentSession.token ||
      current.destination?.kind !== 'home' ||
      current.destination.profile.id !== ownerID
    ) return null;
    const history = mergeNotificationHistoryPage(
      { items: [], nextCursor: '', unreadCount: 0 },
      page,
      '',
    );
    setNotificationHistory(history);
    setNotificationUnreadCount(history.unreadCount);
    void setNativeNotificationBadge(history.unreadCount).catch(() => undefined);
    return page;
  }

  async function resolvePushNotificationRecord(
    currentSession: Session,
    ownerID: string,
    notificationID: string,
  ): Promise<{ destination: NotificationDestination; presentation: 'actionable' | 'informational' } | null> {
    const result = generatedResponse(
      await createSessionApiClient(apiURL, () => currentSession.token).getNotification(notificationID),
    );
    if (!result.ok) {
      if (result.status === 404) return null;
      throw new Error('notification_resolution_unavailable');
    }
    const envelope = await result.json();
    const interactionDisabled = interactionDisabledDestinationFromAPI(envelope?.data);
    const notification = interactionDisabled ? undefined : mergeNotificationHistoryPage(
      { items: [], nextCursor: '', unreadCount: 0 },
      { items: envelope ? [envelope.data] : [], nextCursor: '', unreadCount: 0 },
      '',
    ).items[0];
    const current = notificationLifecycleState.current;
    if (
      current.session?.token !== currentSession.token ||
      current.destination?.kind !== 'home' ||
      current.destination.profile.id !== ownerID
    ) return null;
    if (interactionDisabled) {
      return { destination: interactionDisabled, presentation: 'informational' };
    }
    if (!notification) return null;
    if (notification.type === 'path_invitation_received') {
      const pendingInvitations = await loadPendingInvitationsThroughTarget(
        currentSession,
        notification.invitationId,
        () => {
          const active = notificationLifecycleState.current;
          return active.session?.token === currentSession.token
            && active.destination?.kind === 'home'
            && active.destination.profile.id === ownerID;
        },
      );
      const latest = notificationLifecycleState.current;
      if (
        latest.session?.token !== currentSession.token ||
        latest.destination?.kind !== 'home' ||
        latest.destination.profile.id !== ownerID
      ) return null;
      setDestination((destination) => destination?.kind === 'home' && destination.profile.id === ownerID
        ? {
            ...destination,
            profile: { ...destination.profile, pendingInvitations },
          }
        : destination);
      return pendingInvitations.items.some((item) => item.invitation.id === notification.invitationId)
        ? { destination: { kind: 'invitation', invitationID: notification.invitationId }, presentation: notification.presentation }
        : null;
    }
    if (notification.type === 'path_deleted') return null;
    if (notification.type === 'follow_request_received') {
      return { destination: { kind: 'follow-request', requestID: notification.followRequestId }, presentation: notification.presentation };
    }
    if (notification.type === 'new_follower' || notification.type === 'follow_request_accepted') {
      return { destination: { kind: 'profile', username: notification.actor.username }, presentation: notification.presentation };
    }
    if (notification.type === 'practice_comment' || notification.type === 'comment_heart') {
      return { destination: { kind: 'comments', eventID: notification.socialFeedEventId, commentID: notification.commentId }, presentation: notification.presentation };
    }
    if (notification.type === 'nudge_received') {
      const accessible = current.destination.profile.paths.some((path) => path.id === notification.pathId)
        || current.destination.profile.archivedPaths.some((path) => path.id === notification.pathId);
      return accessible ? { destination: { kind: 'path', pathID: notification.pathId }, presentation: notification.presentation } : null;
    }
    const accessible = current.destination.profile.paths.some((path) => path.id === notification.pathId)
      || current.destination.profile.archivedPaths.some((path) => path.id === notification.pathId);
    return accessible ? { destination: { kind: 'path', pathID: notification.pathId }, presentation: notification.presentation } : null;
  }

  async function resolvePushNotification(
    currentSession: Session,
    ownerID: string,
    notificationID: string,
  ): Promise<NotificationDestination | null> {
    return (await resolvePushNotificationRecord(currentSession, ownerID, notificationID))?.destination ?? null;
  }

  async function refreshForegroundNotificationTarget(
    targetKey: string,
    currentSession: Session,
    ownerID: string,
  ): Promise<void> {
    const active = notificationLifecycleState.current;
    if (
      active.session?.token !== currentSession.token ||
      active.destination?.kind !== 'home' ||
      active.destination.profile.id !== ownerID ||
      foregroundTargetKey.current !== targetKey
    ) return;
    if (targetKey.startsWith('comments:')) {
      const target = practiceCommentTarget.current;
      if (target && `comments:${target.eventID}` === targetKey) {
        await loadPracticeComments(target, '', true);
      }
      return;
    }
    if (targetKey.startsWith('path:')) {
      const pathID = targetKey.slice('path:'.length);
      if (pathID) await refreshHomeOrganizationPath(pathID, currentSession, ownerID);
      return;
    }
    if (targetKey === 'invitations') {
      await openPendingInvitations(focusedInvitationIDRef.current ?? undefined, false);
      return;
    }
    if (targetKey === 'follow-requests') {
      await loadSocialFollowRequests(true);
      return;
    }
    const profileUsername = socialProfileUsernameRef.current;
    if (targetKey.startsWith('profile:') && profileUsername) {
      await loadSocialProfile(profileUsername, true);
    }
  }

  async function markPushNotificationRead(
    currentSession: Session,
    ownerID: string,
    notificationID: string,
  ) {
    const result = generatedResponse(
      await createSessionApiClient(apiURL, () => currentSession.token).markNotificationRead(notificationID),
    );
    if (!result.ok) throw new Error('notification_mutation_unavailable');
    const envelope = await result.json();
    const current = notificationLifecycleState.current;
    if (
      !envelope ||
      current.session?.token !== currentSession.token ||
      current.destination?.kind !== 'home' ||
      current.destination.profile.id !== ownerID
    ) throw new Error('notification_session_changed');
    setNotificationHistory((history) => history.items.some(({ id }) => id === notificationID)
      ? applyNotificationMutation(history, { kind: 'read', notificationId: notificationID }, envelope.data).history
      : history);
    setNotificationUnreadCount(envelope.data.unreadCount);
    await setNativeNotificationBadge(envelope.data.unreadCount).catch(() => false);
  }

  async function openDisabledInteraction(
    destination: Extract<NotificationDestination, { kind: 'interaction-disabled' }>,
  ) {
    const current = notificationLifecycleState.current;
    if (!current.session || current.destination?.kind !== 'home') return;
    const currentSession = current.session;
    const ownerID = current.destination.profile.id;
    const ticket = interactionDisabledEventOperations.issue();
    const currentTarget = () => {
      const latest = notificationLifecycleState.current;
      return ticket.current() && latest.session === currentSession &&
        latest.destination?.kind === 'home' && latest.destination.profile.id === ownerID;
    };
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .getPracticeFeedEvent(destination.eventID));
      if (response.status === 404) return;
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_social_feed_event_response');
      const focused = socialFeedEventFromAPI(envelope.data);
      if (focused.id !== destination.eventID || !currentTarget()) return;
      const nextFeed = {
        ...socialFeedPage.current,
        items: [focused, ...socialFeedPage.current.items.filter(({ id }) => id !== focused.id)],
      };
      socialFeedPage.current = nextFeed;
      setSocialFeed((current) => ({ ...current, items: nextFeed.items, status: 'ready' }));
    } catch (cause) {
      if (currentTarget()) {
        await handleFeatureSessionFailure(cause, currentSession, { current: currentTarget });
      }
    }
  }

  function navigateFromPush(destination: NotificationDestination) {
    if (notificationLifecycleState.current.destination?.kind !== 'home') return;
    if (destination.kind === 'interaction-disabled') {
      setSocialFeed((current) => ({
        ...current,
        interactionNoticeEventID: destination.eventID,
        interactionNoticeKey: destination.interaction === 'comments'
          ? 'social.interactionDisabledComments'
          : 'social.interactionDisabledReactions',
      }));
      router.push('/(tabs)/following');
      void openDisabledInteraction(destination);
      return;
    }
    if (destination.kind === 'comments') {
      openPracticeComments(destination.eventID, notificationLifecycleState.current.destination.profile.id, destination.commentID);
      return;
    }
    if (destination.kind === 'path') {
      openPathDetail(destination.pathID);
      return;
    }
    if (destination.kind === 'follow-request') {
      void loadSocialFollowRequests(true);
      router.push('/follow-requests');
      return;
    }
    if (destination.kind === 'profile') {
      void loadSocialProfile(destination.username);
      router.push({ pathname: '/profile/[username]', params: { username: destination.username } });
      return;
    }
    void openPendingInvitations(destination.invitationID);
  }

  async function loadMoreNotifications(cursor: string) {
    if (!session || destination?.kind !== 'home' || notificationsBusy || notificationTarget.current ||
      notificationMutationAdmission.current ||
      cursor !== notificationHistory.nextCursor) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = `notifications:history:${cursor}`;
    const ticket = notificationRefreshOperations.issue();
    notificationTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    setNotificationsBusy(true);
    setNotificationsErrorKey(null);
    try {
      const page = await loadNotificationHistoryPage(currentSession, cursor);
      if (
        !ticket.current() ||
        !ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) return;
      setNotificationHistory((current) => {
        const history = mergeNotificationHistoryPage(current, page, cursor);
        setNotificationUnreadCount(history.unreadCount);
        void setNativeNotificationBadge(history.unreadCount).catch(() => undefined);
        return history;
      });
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, () =>
        ticket.current() && ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession));
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) {
        setNotificationsErrorKey('notification.error');
      }
    } finally {
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationTarget.current, ownerID, intentKey, currentSession)
      ) {
        notificationTarget.current = null;
        setNotificationsBusy(false);
      }
    }
  }

  async function mutateNotification(
    mutation: NotificationMutation,
    notification?: PathInvitationNotification,
  ): Promise<boolean> {
    if (!session || destination?.kind !== 'home' || notificationsBusy || notificationMutationAdmission.current) return false;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const history = notificationHistory;
    const intentKey = `notifications:mutation:${mutation.kind}:${'notificationId' in mutation ? mutation.notificationId : 'all'}`;
    notificationMutationAdmission.current = true;
    notificationRefreshOperations.invalidate();
    notificationTarget.current = null;
    setNotificationsRefreshing(false);
    const ticket = notificationMutationOperations.issue();
    notificationMutationTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    setNotificationsBusy(true);
    setNotificationsErrorKey(null);
    try {
      const api = createSessionApiClient(apiURL, () => currentSession.token);
      const result = mutation.kind === 'read'
        ? await api.markNotificationRead(mutation.notificationId)
        : mutation.kind === 'delete'
          ? await api.deleteNotification(mutation.notificationId)
          : await api.markAllNotificationsRead();
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_notification_mutation');
      if (
        !ticket.current() ||
        !ownsCurrentNotificationOperation(notificationMutationTarget.current, ownerID, intentKey, currentSession)
      ) return false;
      setNotificationHistory((current) => {
        const next = applyNotificationMutation(current, mutation, envelope.data);
        setNotificationUnreadCount(next.unreadCount);
        void setNativeNotificationBadge(next.unreadCount).catch(() => false);
        return next.history;
      });
      return notification === undefined || history.items.some(({ id }) => id === notification.id);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, () =>
        ticket.current() && ownsCurrentNotificationOperation(notificationMutationTarget.current, ownerID, intentKey, currentSession));
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationMutationTarget.current, ownerID, intentKey, currentSession)
      ) {
        setNotificationsErrorKey('notification.mutationError');
      }
      return false;
    } finally {
      if (
        ticket.current() &&
        ownsCurrentNotificationOperation(notificationMutationTarget.current, ownerID, intentKey, currentSession)
      ) {
        notificationMutationTarget.current = null;
        notificationMutationAdmission.current = false;
        setNotificationMutationGeneration((current) => current + 1);
        setNotificationsBusy(false);
      }
    }
  }

  function ownsCurrentSocialOperation(
    target: SocialSessionTarget<Session> | null,
    ownerID: string,
    intentKey: string,
    requestSession: Session,
  ): boolean {
    const current = notificationLifecycleState.current;
    const currentOwnerID = current.destination?.kind === 'home' ? current.destination.profile.id : null;
    return ownsSocialSessionTarget(target, ownerID, requestSession.token, intentKey) &&
      currentSocialSessionCredential(target, currentOwnerID, intentKey, current.session) !== null;
  }

  async function handleSocialOperationFailure(
    cause: unknown,
    requestSession: Session,
    current: () => boolean,
  ) {
    if (!current()) return;
    if (notificationLifecycleState.current.session?.token !== requestSession.token) return;
    await handleFeatureSessionFailure(cause, requestSession, { current });
  }

  async function requestSocialProfiles(
    currentSession: Session,
    query: string,
    cursor: string,
    current: () => boolean,
  ) {
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token)
          .searchProfiles(query, cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      return profileSearchPageFromAPI(await response.json());
    } catch (cause) {
      await handleSocialOperationFailure(cause, currentSession, current);
      throw cause;
    }
  }

  async function searchSocialProfiles(value: string, refreshing = false) {
    if (!session || destination?.kind !== 'home') return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const query = profileSearchQuery(value);
    const ticket = socialProfileSearchOperations.issue();
    if (!query) {
      socialProfileSearchTarget.current = null;
      const result = await socialProfileSearchOwner.search(value, async () => ({ items: [], nextCursor: '' }));
      if (result.kind === 'cleared' && ticket.current()) {
        socialSearchPage.current = result.state;
        setSocialSearch({ items: [], loadingMore: false, query: value, refreshing: false, status: 'idle' });
      }
      return;
    }
    const intentKey = `social:search:${query.toLowerCase()}`;
    socialProfileSearchTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialProfileSearchTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialSearch((current) => ({
      ...current,
      items: refreshing ? current.items : [],
      loadingMore: false,
      query,
      refreshing,
      status: refreshing && current.items.length > 0 ? 'ready' : 'loading',
      errorKey: undefined,
    }));
    const result = await socialProfileSearchOwner.search(
      query,
      (requestedQuery, cursor) => requestSocialProfiles(currentSession, requestedQuery, cursor, currentTarget),
    );
    if (!currentTarget()) return;
    if (result.kind === 'loaded') {
      socialSearchPage.current = result.state;
      setSocialSearch({
        items: result.state.items,
        loadingMore: false,
        nextCursor: result.state.nextCursor || undefined,
        query: result.state.query,
        refreshing: false,
        status: 'ready',
      });
    } else if (result.kind === 'failed') {
      setSocialSearch((current) => ({
        ...current,
        errorKey: 'social.searchUnavailableDescription',
        loadingMore: false,
        refreshing: false,
        status: 'error',
      }));
    }
  }

  async function loadMoreSocialProfiles() {
    if (!session || destination?.kind !== 'home' || !socialSearchPage.current.nextCursor || socialSearch.loadingMore) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = socialProfileSearchOperations.issue();
    const intentKey = `social:search:${socialSearchPage.current.query.toLowerCase()}`;
    socialProfileSearchTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialProfileSearchTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialSearch((current) => ({ ...current, loadingMore: true, errorKey: undefined }));
    const result = await socialProfileSearchOwner.loadMore(
      socialSearchPage.current,
      (query, cursor) => requestSocialProfiles(currentSession, query, cursor, currentTarget),
    );
    if (!currentTarget()) return;
    if (result.kind === 'loaded') {
      socialSearchPage.current = result.state;
      setSocialSearch({
        items: result.state.items,
        loadingMore: false,
        nextCursor: result.state.nextCursor || undefined,
        query: result.state.query,
        refreshing: false,
        status: 'ready',
      });
    } else if (result.kind === 'failed') {
      setSocialSearch((current) => ({
        ...current,
        errorKey: 'social.searchUnavailableDescription',
        loadingMore: false,
        refreshing: false,
        status: 'error',
      }));
    }
  }

  async function loadSocialProfile(username: string, refreshing = false) {
    if (!session || destination?.kind !== 'home') return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = socialProfileDetailOperations.issue();
    const intentKey = `social:profile:${username.toLowerCase()}`;
    socialProfileDetailTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialProfileDetailTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialProfile((current) => ({
      ...current,
      profile: refreshing ? current.profile : undefined,
      refreshing,
      status: refreshing && current.profile ? 'ready' : 'loading',
      username,
      errorKey: undefined,
    }));
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).profileByUsername(username),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const profile = publicProfileFromAPI(await response.json());
      if (!currentTarget() || profile.username.toLowerCase() !== username.toLowerCase()) return;
      setSocialProfile({ profile, refreshing: false, status: 'ready', username: profile.username });
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialProfile({
        errorKey: 'social.profileUnavailableDescription',
        refreshing: false,
        status: 'error',
        username,
      });
    }
  }

  async function mutateSocialRelationship(action: 'follow' | 'cancel-request' | 'unfollow', username: string, frozenIdempotencyKey?: string) {
    const latest = notificationLifecycleState.current;
    const currentProfile = socialProfileStateRef.current.profile;
    const expectedRelationship = action === 'follow' ? 'none' : action === 'cancel-request' ? 'requested' : 'following';
    if (!latest.session || latest.destination?.kind !== 'home' || socialRelationshipSubmitting.current || socialProfile.mutating ||
      currentProfile?.relationship !== expectedRelationship ||
      socialProfileUsernameRef.current?.toLowerCase() !== username.toLowerCase()) return;
    socialRelationshipSubmitting.current = true;
    const currentSession = latest.session;
    const ownerID = latest.destination.profile.id;
    const ticket = socialProfileDetailOperations.issue();
    const intentKey = `social:profile:${username.toLowerCase()}`;
    socialProfileDetailTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialProfileDetailTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialProfile((current) => current.username?.toLowerCase() === username.toLowerCase()
      ? { ...current, mutating: true, mutationErrorKey: undefined }
      : current);
    try {
      const api = createSessionApiClient(apiURL, () => currentSession.token);
      const idempotencyKey = frozenIdempotencyKey ?? Crypto.randomUUID();
      const result = action === 'follow'
        ? await api.followProfile(username, idempotencyKey)
        : action === 'cancel-request'
          ? await api.cancelFollowRequest(username, idempotencyKey)
          : await api.unfollowProfile(username, idempotencyKey);
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const mutation = relationshipMutationResultFromAPI(await response.json());
      if (!currentTarget() ||
        mutation.profile.username.toLowerCase() !== username.toLowerCase()) return;
      socialSearchPage.current = {
        ...socialSearchPage.current,
        items: socialSearchPage.current.items.map((profile) => profile.userId === mutation.profile.userId ? mutation.profile : profile),
      };
      setSocialSearch((current) => ({
        ...current,
        items: current.items.map((profile) => profile.userId === mutation.profile.userId ? mutation.profile : profile),
      }));
      setSocialProfile({ profile: mutation.profile, refreshing: false, status: 'ready', username: mutation.profile.username, mutating: false });
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialProfile((current) => ({
        ...current,
        mutating: false,
        mutationErrorKey: 'social.relationshipUnavailable',
      }));
    } finally {
      if (currentTarget()) socialRelationshipSubmitting.current = false;
    }
  }

  async function loadSocialFollowRequests(refreshing = false, cursor = '') {
    if (!session || destination?.kind !== 'home' || socialFollowRequests.busyRequestID) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = socialFollowRequestOperations.issue();
    const intentKey = 'social:follow-requests';
    socialFollowRequestTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialFollowRequestTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialFollowRequests((current) => ({
      ...current,
      errorKey: undefined,
      refreshing,
      status: cursor || (refreshing && current.items.length > 0) ? 'ready' : 'loading',
    }));
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).followRequests(cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const page = followRequestPageFromAPI(await response.json());
      const next = cursor
        ? mergeFollowRequestPage(socialFollowRequestPage.current, page, cursor)
        : page;
      if (!currentTarget()) return;
      socialFollowRequestPage.current = next;
      setSocialFollowRequests({ items: next.items, nextCursor: next.nextCursor || undefined, refreshing: false, status: 'ready' });
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialFollowRequests((current) => ({
        ...current,
        errorKey: 'social.followRequestsUnavailable',
        refreshing: false,
        status: 'error',
      }));
    }
  }

  async function reviewSocialFollowRequest(decision: 'accept' | 'reject', requestID: string, frozenIdempotencyKey?: string) {
    const latest = notificationLifecycleState.current;
    if (!latest.session || latest.destination?.kind !== 'home' || socialFollowRequestSubmitting.current || socialFollowRequests.busyRequestID ||
      !socialFollowRequestPage.current.items.some(({ id }) => id === requestID)) return;
    socialFollowRequestSubmitting.current = requestID;
    const currentSession = latest.session;
    const ownerID = latest.destination.profile.id;
    const ticket = socialFollowRequestOperations.issue();
    const intentKey = `social:follow-request:${requestID}`;
    socialFollowRequestTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialFollowRequestTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialFollowRequests((current) => ({ ...current, busyRequestID: requestID, errorKey: undefined }));
    try {
      const api = createSessionApiClient(apiURL, () => currentSession.token);
      const idempotencyKey = frozenIdempotencyKey ?? Crypto.randomUUID();
      const result = decision === 'accept'
        ? await api.acceptFollowRequest(requestID, idempotencyKey)
        : await api.rejectFollowRequest(requestID, idempotencyKey);
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const reviewed = followRequestReviewResultFromAPI(await response.json());
      if (reviewed.request.id !== requestID || reviewed.decision !== (decision === 'accept' ? 'accepted' : 'rejected')) {
        throw new Error('mismatched_follow_request_review');
      }
      if (!currentTarget()) return;
      const next = removeResolvedFollowRequest(socialFollowRequestPage.current, requestID);
      socialFollowRequestPage.current = next;
      setSocialFollowRequests({ items: next.items, nextCursor: next.nextCursor || undefined, refreshing: false, status: 'ready' });
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialFollowRequests((current) => ({
        ...current,
        busyRequestID: undefined,
        errorKey: 'social.reviewRequestUnavailable',
      }));
    } finally {
      if (currentTarget()) socialFollowRequestSubmitting.current = null;
    }
  }

  async function loadSocialFeed(cursor = '', refreshing = false) {
    if (!session || destination?.kind !== 'home' || socialFeed.loadingMore) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = socialFeedOperations.issue();
    const admittedReactionRevisions = new Map(socialFeedReactionRevisions);
    const intentKey = 'social:feed';
    socialFeedTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialFeedTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialFeed((current) => ({
      ...current,
      errorKey: undefined,
      loadingMore: Boolean(cursor),
      refreshing,
      status: cursor || (refreshing && current.items.length > 0) ? 'ready' : 'loading',
    }));
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).socialFeed(cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_social_feed_response');
      const incoming = preserveNewerSocialReactionSummaries({
        items: envelope.data.items.map(socialFeedEventFromAPI),
        nextCursor: envelope.meta.nextCursor ?? '',
      }, socialFeedPage.current, admittedReactionRevisions, socialFeedReactionRevisions);
      const next = mergeSocialFeedPage(socialFeedPage.current, incoming, cursor || undefined);
      if (!currentTarget()) return;
      socialFeedPage.current = next;
      setSocialFeed((current) => ({
        ...current,
        items: next.items,
        loadingMore: false,
        nextCursor: next.nextCursor || undefined,
        refreshing: false,
        status: 'ready',
      }));
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialFeed((current) => ({
        ...current,
        errorKey: 'social.feedUnavailableDescription',
        loadingMore: false,
        refreshing: false,
        status: 'error',
      }));
    }
  }

  async function loadSocialDeepEvent(eventID: string) {
    const latest = notificationLifecycleState.current;
    if (!latest.session || latest.destination?.kind !== 'home') return;
    const currentSession = latest.session;
    const ownerID = latest.destination.profile.id;
    const intentKey = `social:event:${eventID}`;
    const ticket = socialDeepEventOperations.issue();
    socialDeepEventTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialDeepEventTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialDeepEvent({ eventID, status: 'loading' });
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .getPracticeFeedEvent(eventID));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_social_feed_event_response');
      const event = socialFeedEventFromAPI(envelope.data);
      if (event.id !== eventID) throw new Error('mismatched_social_feed_event');
      if (!currentTarget()) return;
      setSocialDeepEvent({ event, eventID, status: 'ready' });
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialDeepEvent({ eventID, status: 'error' });
    }
  }

  async function mutateSocialFeedReaction(event: SocialFeedEvent, reaction: SocialReaction | null) {
    const currentEvent = socialFeedPage.current.items.find(({ id }) => id === event.id);
    if (!event.reactionsEnabled || !currentEvent?.reactionsEnabled || !session || destination?.kind !== 'home') return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    if (socialFeedReactionAdmissions.has(event.id)) throw new Error('social_reaction_in_progress');
    const admission = Symbol('social-feed-reaction');
    socialFeedReactionAdmissions.set(event.id, admission);
    const previousRetry = socialFeedReactionRetries.get(event.id);
    const idempotencyKey = previousRetry?.reaction === reaction
      ? previousRetry.key
      : Crypto.randomUUID();
    socialFeedReactionRetries.set(event.id, { key: idempotencyKey, reaction });
    const reactionIntentKey = `social:reaction:${event.id}`;
    socialFeedReactionTargets.set(
      event.id,
      createSocialSessionTarget(ownerID, currentSession, reactionIntentKey),
    );
    let operationOwner = socialFeedReactionOperations.get(event.id);
    if (!operationOwner) {
      operationOwner = createSessionOperationOwner();
      socialFeedReactionOperations.set(event.id, operationOwner);
    }
    const ticket = operationOwner.issue();
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialFeedReactionTargets.get(event.id) ?? null,
      ownerID,
      reactionIntentKey,
      currentSession,
    ) &&
      socialFeedPage.current.items.some(({ id }) => id === event.id);
    try {
      const api = createSessionApiClient(apiURL, () => currentSession.token);
      const result = reaction
        ? await api.setSocialFeedReaction(event.id, reaction, idempotencyKey)
        : await api.removeSocialFeedReaction(event.id, idempotencyKey);
      const response = generatedResponse(result);
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_social_reaction_response');
      if (!currentTarget()) return;
      const next = applySocialReactionSummary(
        socialFeedPage.current,
        event.id,
        socialReactionSummaryFromAPI(envelope.data),
      );
      socialFeedReactionRetries.delete(event.id);
      if (next === socialFeedPage.current) return;
      socialFeedReactionRevisions.set(
        event.id,
        (socialFeedReactionRevisions.get(event.id) ?? 0) + 1,
      );
      socialFeedPage.current = next;
      setSocialFeed((current) => ({ ...current, items: next.items }));
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) throw cause;
    } finally {
      if (socialFeedReactionAdmissions.get(event.id) === admission) {
        socialFeedReactionAdmissions.delete(event.id);
        socialFeedReactionTargets.delete(event.id);
      }
    }
  }

  async function getConfiguredTimeZone(): Promise<TimeZonePreference> {
    if (!session || destination?.kind !== 'home') throw new Error('time_zone_unavailable');
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = 'settings:time-zone:load';
    settingsOperationTargets.current.set(
      intentKey,
      createNotificationSessionTarget(ownerID, currentSession, intentKey),
    );
    const currentTarget = () => ownsNotificationSessionTarget(
      settingsOperationTargets.current.get(intentKey) ?? null,
      ownerID,
      currentSession.token,
      intentKey,
    );
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).configuredTimeZone(),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope || !currentTarget()) throw new Error('time_zone_superseded');
      return timeZonePreferenceFromAPI(envelope.data);
    } catch (cause) {
      await handleSettingsOperationFailure(cause, currentSession, currentTarget);
      throw cause;
    }
  }

  async function updateConfiguredTimeZone(intent: FrozenTimeZoneChangeIntent): Promise<TimeZonePreference> {
    if (!session || destination?.kind !== 'home') throw new Error('time_zone_unavailable');
    if (settingsMutationAdmissions.current.has('time-zone')) throw new Error('time_zone_unavailable');
    const currentSession = session;
    const ownerID = destination.profile.id;
    const admission = Symbol('settings-time-zone');
    settingsMutationAdmissions.current.set('time-zone', admission);
    const intentKey = `settings:time-zone:update:${intent.idempotencyKey}`;
    settingsOperationTargets.current.set(
      intentKey,
      createNotificationSessionTarget(ownerID, currentSession, intentKey),
    );
    const currentTarget = () => ownsNotificationSessionTarget(
      settingsOperationTargets.current.get(intentKey) ?? null,
      ownerID,
      currentSession.token,
      intentKey,
    );
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .updateConfiguredTimeZone({
          confirmed: intent.confirmed,
          proposedTimeZone: intent.proposedTimeZone,
          reviewedTimeZone: intent.reviewedTimeZone,
        }, intent.idempotencyKey));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope || !currentTarget()) throw new Error('time_zone_superseded');
      return timeZonePreferenceFromAPI(envelope.data);
    } catch (cause) {
      await handleSettingsOperationFailure(cause, currentSession, currentTarget);
      throw cause;
    } finally {
      if (settingsMutationAdmissions.current.get('time-zone') === admission) {
        settingsMutationAdmissions.current.delete('time-zone');
        settingsOperationTargets.current.delete(intentKey);
      }
    }
  }

  async function getInteractionSettings(): Promise<InteractionSettings> {
    if (!session || destination?.kind !== 'home') throw new Error('interaction_settings_unavailable');
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = 'settings:interactions:load';
    settingsOperationTargets.current.set(
      intentKey,
      createNotificationSessionTarget(ownerID, currentSession, intentKey),
    );
    const currentTarget = () => ownsNotificationSessionTarget(
      settingsOperationTargets.current.get(intentKey) ?? null,
      ownerID,
      currentSession.token,
      intentKey,
    );
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).getInteractionSettings(),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope || !currentTarget()) throw new Error('interaction_settings_superseded');
      return interactionSettingsFromAPI(envelope.data);
    } catch (cause) {
      await handleSettingsOperationFailure(cause, currentSession, currentTarget);
      throw cause;
    }
  }

  async function refreshNotificationsAfterInteractionSettings(
    currentSession: Session,
    ownerID: string,
    currentTarget: () => boolean,
  ) {
    const ticket = notificationRefreshOperations.issue();
    try {
      const page = await loadNotificationHistoryPage(currentSession);
      if (!ticket.current() || !currentTarget()) return;
      const history = mergeNotificationHistoryPage(
        { items: [], nextCursor: '', unreadCount: 0 },
        page,
        '',
      );
      setNotificationHistory(history);
      setNotificationUnreadCount(history.unreadCount);
      await setNativeNotificationBadge(history.unreadCount).catch(() => false);
    } catch {
      // The settings mutation is already authoritative; notification refresh can retry independently.
    }
  }

  async function updateInteractionSettings(
    settings: InteractionSettings,
    idempotencyKey: string,
  ): Promise<InteractionSettings> {
    if (!session || destination?.kind !== 'home') throw new Error('interaction_settings_unavailable');
    if (settingsMutationAdmissions.current.has('interactions')) throw new Error('interaction_settings_unavailable');
    const currentSession = session;
    const ownerID = destination.profile.id;
    const admission = Symbol('settings-interactions');
    settingsMutationAdmissions.current.set('interactions', admission);
    const intentKey = `settings:interactions:update:${idempotencyKey}`;
    settingsOperationTargets.current.set(
      intentKey,
      createNotificationSessionTarget(ownerID, currentSession, intentKey),
    );
    const currentTarget = () => ownsNotificationSessionTarget(
      settingsOperationTargets.current.get(intentKey) ?? null,
      ownerID,
      currentSession.token,
      intentKey,
    );
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .updateInteractionSettings(settings, idempotencyKey));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope || !currentTarget()) throw new Error('interaction_settings_superseded');
      const authoritative = interactionSettingsFromAPI(envelope.data);
      const nextFeed = {
        ...socialFeedPage.current,
        items: socialFeedPage.current.items.map((item) => item.participant.userId === ownerID
          ? {
              ...item,
              commentsEnabled: authoritative.commentsEnabled,
              reactionsEnabled: authoritative.reactionsEnabled,
            }
          : item),
      };
      socialFeedPage.current = nextFeed;
      setSocialFeed((current) => ({ ...current, items: nextFeed.items }));
      await refreshNotificationsAfterInteractionSettings(currentSession, ownerID, currentTarget);
      if (!currentTarget()) throw new Error('interaction_settings_superseded');
      return authoritative;
    } catch (cause) {
      await handleSettingsOperationFailure(cause, currentSession, currentTarget);
      throw cause;
    } finally {
      if (settingsMutationAdmissions.current.get('interactions') === admission) {
        settingsMutationAdmissions.current.delete('interactions');
        settingsOperationTargets.current.delete(intentKey);
      }
    }
  }

  async function getNudgeChannelPreference(): Promise<NudgeChannelPreference> {
    if (!session || destination?.kind !== 'home') throw new Error('nudge_channel_unavailable');
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = 'notification-settings:nudge:load';
    const ticket = notificationSettingsOperations.issue();
    notificationSettingsTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentNotificationOperation(
      notificationSettingsTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .getNudgeNotificationChannel());
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope || !currentTarget()) throw new Error('nudge_channel_superseded');
      return nudgeChannelPreferenceFromAPI(envelope.data);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, currentTarget);
      throw cause;
    } finally {
      if (currentTarget()) notificationSettingsTarget.current = null;
    }
  }

  async function updateNudgeChannelPreference(
    body: NudgeChannelUpdateBody,
    idempotencyKey: string,
  ): Promise<NudgeChannelPreference> {
    if (!session || destination?.kind !== 'home' || notificationSettingsAdmission.current) {
      throw new Error('nudge_channel_unavailable');
    }
    const currentSession = session;
    const ownerID = destination.profile.id;
    const intentKey = `notification-settings:nudge:update:${idempotencyKey}`;
    const admission = Symbol('notification-settings');
    notificationSettingsAdmission.current = admission;
    const ticket = notificationSettingsOperations.issue();
    notificationSettingsTarget.current = createNotificationSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentNotificationOperation(
      notificationSettingsTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .updateNudgeNotificationChannel(body, idempotencyKey));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope || !currentTarget()) throw new Error('nudge_channel_superseded');
      return nudgeChannelPreferenceFromAPI(envelope.data);
    } catch (cause) {
      await handleNotificationOperationFailure(cause, currentSession, currentTarget);
      throw cause;
    } finally {
      if (currentTarget()) notificationSettingsTarget.current = null;
      if (notificationSettingsAdmission.current === admission) notificationSettingsAdmission.current = null;
    }
  }

  async function loadSocialActiveFollowing(cursor = '', refreshing = false) {
    if (!session || destination?.kind !== 'home' || socialActiveFollowing.loadingMore) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = socialActiveFollowingOperations.issue();
    const intentKey = 'social:active';
    socialActiveFollowingTarget.current = createSocialSessionTarget(ownerID, currentSession, intentKey);
    const currentTarget = () => ticket.current() && ownsCurrentSocialOperation(
      socialActiveFollowingTarget.current,
      ownerID,
      intentKey,
      currentSession,
    );
    setSocialActiveFollowing((current) => ({
      ...current,
      errorKey: undefined,
      loadingMore: Boolean(cursor),
      refreshing,
      status: cursor || (refreshing && current.items.length > 0) ? 'ready' : 'loading',
    }));
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token)
          .socialActiveFollowing(cursor || undefined),
      );
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_social_active_following_response');
      const incoming: ActiveFollowingPage = {
        items: envelope.data.items.map(activeFollowingItemFromAPI),
        nextCursor: envelope.meta.nextCursor ?? '',
      };
      const next = mergeActiveFollowingPage(
        socialActiveFollowingPage.current,
        incoming,
        cursor || undefined,
      );
      if (!currentTarget()) return;
      socialActiveFollowingPage.current = next;
      setSocialActiveFollowing({
        items: next.items,
        loadingMore: false,
        nextCursor: next.nextCursor || undefined,
        refreshing: false,
        status: 'ready',
      });
    } catch (cause) {
      if (!currentTarget()) return;
      await handleSocialOperationFailure(cause, currentSession, currentTarget);
      if (currentTarget()) setSocialActiveFollowing((current) => ({
        ...current,
        errorKey: 'social.activeUnavailable',
        loadingMore: false,
        refreshing: false,
        status: 'error',
      }));
    }
  }

  function currentPracticeCommentTarget(target: NonNullable<typeof practiceCommentTarget.current>) {
    const current = notificationLifecycleState.current;
    const active = practiceCommentTarget.current;
    return active?.eventID === target.eventID && active.eventOwnerID === target.eventOwnerID &&
      active.ownerID === target.ownerID && active.intentKey === target.intentKey &&
      active.sessionTokens.includes(target.requestSession.token) &&
      current.session !== null && active.sessionTokens.includes(current.session.token) &&
      current.destination?.kind === 'home' && current.destination.profile.id === target.ownerID;
  }

  function currentPracticeCommentCredential(
    target: NonNullable<typeof practiceCommentTarget.current>,
  ): Session | null {
    const current = notificationLifecycleState.current.session;
    return current && currentPracticeCommentTarget(target) ? current : null;
  }

  function currentPracticeCommentHistoryTarget(
    target: NonNullable<typeof practiceCommentHistoryTarget.current>,
  ) {
    const active = practiceCommentHistoryTarget.current;
    const lifecycle = notificationLifecycleState.current;
    return active?.commentID === target.commentID && active.eventID === target.eventID &&
      active.ownerID === target.ownerID && active.intentKey === target.intentKey &&
      active.sessionTokens.includes(target.requestSession.token) &&
      lifecycle.session !== null && active.sessionTokens.includes(lifecycle.session.token) &&
      lifecycle.destination?.kind === 'home' && lifecycle.destination.profile.id === target.ownerID;
  }

  async function loadPracticeComments(
    target: NonNullable<typeof practiceCommentTarget.current>,
    cursor = '',
    refreshing = false,
  ) {
    const requestSession = currentPracticeCommentCredential(target);
    if (!requestSession) return;
    const ticket = practiceCommentOperations.issue();
    setPracticeComments((current) => current?.eventID === target.eventID ? {
      ...current,
      errorKey: undefined,
      loadingMore: Boolean(cursor),
      refreshing,
      status: cursor || (refreshing && practiceCommentPage.current.items.length > 0) ? 'ready' : 'loading',
    } : current);
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => requestSession.token)
        .socialFeedComments(target.eventID, cursor || undefined));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const incoming = practiceCommentPageFromAPI(await response.json());
      if (incoming.items.some((comment) => comment.eventId !== target.eventID)) throw new Error('mismatched_practice_comments');
      const next = mergePracticeCommentPage(practiceCommentPage.current, incoming, cursor);
      if (!ticket.current() || !currentPracticeCommentTarget(target)) return;
      practiceCommentPage.current = next;
      setPracticeComments((current) => current?.eventID === target.eventID ? {
        ...current,
        busy: false,
        loadingMore: false,
        refreshing: false,
        status: 'ready',
      } : current);
    } catch (cause) {
      if (!ticket.current() || !currentPracticeCommentTarget(target)) return;
      await handleSocialOperationFailure(cause, requestSession, () => ticket.current() && currentPracticeCommentTarget(target));
      if (ticket.current()) setPracticeComments((current) => current?.eventID === target.eventID ? {
        ...current,
        errorKey: 'social.commentsUnavailableDescription',
        loadingMore: false,
        refreshing: false,
        status: 'error',
      } : current);
    }
  }

  function openPracticeComments(eventID: string, eventOwnerID: string, focusedCommentID?: string, navigate = true) {
    if (!session || destination?.kind !== 'home') return;
    practiceCommentOperations.invalidate();
    practiceCommentHistoryOperations.invalidate();
    for (const owner of practiceCommentMutationOperations.values()) owner.invalidate();
    practiceCommentMutationOperations.clear();
    practiceCommentMutationRetries.clear();
    practiceCommentMutationAdmissions.clear();
    for (const comment of practiceCommentPage.current.items) practiceCommentHeartOperations.invalidate(comment.id);
    practiceCommentHeartRosterOperations.invalidate();
    practiceCommentHeartRosterPage.current = null;
    practiceCommentHeartRosterTarget.current = null;
    setPracticeCommentHeartRoster(null);
    const target = {
      ...createSocialSessionTarget(destination.profile.id, session, `social:comments:${eventID}`),
      eventID,
      eventOwnerID,
    };
    practiceCommentTarget.current = target;
    practiceCommentPage.current = { items: [], nextCursor: '' };
    practiceCommentHistoryPage.current = null;
    practiceCommentHistoryTarget.current = null;
    setPracticeComments({
      busy: false,
      eventID,
      eventOwnerID,
      focusedCommentID,
      loadingMore: false,
      refreshing: false,
      status: 'loading',
    });
    preparePracticeCommentsRoute();
    if (navigate) router.push({ pathname: '/following/comments/[eventID]', params: { eventID } });
    void loadPracticeComments(target);
  }

  function commentOperation(id: string) {
    let owner = practiceCommentMutationOperations.get(id);
    if (!owner) { owner = createSessionOperationOwner(); practiceCommentMutationOperations.set(id, owner); }
    return owner;
  }

  function practiceCommentMutationKey(scope: string, intent: string) {
    const previous = practiceCommentMutationRetries.get(scope);
    if (previous?.intent === intent) return previous.key;
    const key = Crypto.randomUUID();
    practiceCommentMutationRetries.set(scope, { intent, key });
    return key;
  }

  async function createPracticeComment(text: string) {
    const target = practiceCommentTarget.current;
    const profile = notificationLifecycleState.current.destination?.kind === 'home'
      ? notificationLifecycleState.current.destination.profile
      : undefined;
    const requestSession = target ? currentPracticeCommentCredential(target) : null;
    if (!target || !profile || !requestSession) throw new Error('comments_unavailable');
    if (practiceCommentCreateAdmission.current) throw new Error('comment_create_in_progress');
    const admission = Symbol('practice-comment-create');
    practiceCommentCreateAdmission.current = admission;
    const temporaryID = `pending:${Crypto.randomUUID()}`;
    const retryScope = `create:${target.eventID}`;
    const idempotencyKey = practiceCommentMutationKey(retryScope, text);
    const ticket = commentOperation(temporaryID).issue();
    const previous = practiceCommentPage.current;
    practiceCommentPage.current = appendOptimisticPracticeComment(previous, {
      author: { userId: profile.id, username: 'self', displayName: profile.displayName },
      createdAt: new Date().toISOString(), eventId: target.eventID, temporaryId: temporaryID, text,
    });
    setPracticeComments((current) => current && { ...current, busy: true, errorKey: undefined });
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => requestSession.token)
        .createSocialFeedComment(target.eventID, { text }, idempotencyKey));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_comment_response');
      const authoritative = practiceCommentMutationFromAPI(
        envelope.data,
        { userId: profile.id, username: 'self', displayName: profile.displayName },
      );
      if (!ticket.current() || !currentPracticeCommentTarget(target) || authoritative.eventId !== target.eventID) return;
      practiceCommentPage.current = replacePracticeComment(practiceCommentPage.current, temporaryID, authoritative);
      practiceCommentMutationRetries.delete(retryScope);
    } catch (cause) {
      if (!ticket.current() || !currentPracticeCommentTarget(target)) return;
      practiceCommentPage.current = removePracticeComment(practiceCommentPage.current, temporaryID);
      setPracticeComments((current) => current && { ...current, errorKey: 'social.commentsMutationUnavailable' });
      await handleSocialOperationFailure(cause, requestSession, () => ticket.current() && currentPracticeCommentTarget(target));
      throw cause;
    } finally {
      if (ticket.current() && currentPracticeCommentTarget(target)) setPracticeComments((current) => current && { ...current, busy: false });
      if (practiceCommentCreateAdmission.current === admission) practiceCommentCreateAdmission.current = null;
    }
  }

  async function editPracticeComment(comment: PracticeComment, text: string) {
    const target = practiceCommentTarget.current;
    const requestSession = target ? currentPracticeCommentCredential(target) : null;
    if (!target || !requestSession) throw new Error('comments_unavailable');
    const ticket = commentOperation(comment.id).issue();
    const previous = practiceCommentPage.current;
    const retryScope = `edit:${comment.id}`;
    if (practiceCommentMutationAdmissions.has(retryScope)) throw new Error('comment_edit_in_progress');
    const admission = Symbol('practice-comment-edit');
    practiceCommentMutationAdmissions.set(retryScope, admission);
    const idempotencyKey = practiceCommentMutationKey(retryScope, `${comment.version}\u0000${text}`);
    practiceCommentPage.current = updatePracticeCommentOptimistically(previous, comment.id, comment.version, text);
    setPracticeComments((current) => current && { ...current, errorKey: undefined });
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => requestSession.token)
        .editSocialFeedComment(target.eventID, comment.id, { text, expectedVersion: comment.version }, idempotencyKey));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_comment_response');
      const authoritative = practiceCommentMutationFromAPI(envelope.data, comment.author, comment);
      if (!ticket.current() || !currentPracticeCommentTarget(target) || authoritative.version <= comment.version) return;
      practiceCommentPage.current = replacePracticeComment(practiceCommentPage.current, comment.id, authoritative);
      practiceCommentMutationRetries.delete(retryScope);
    } catch (cause) {
      if (ticket.current() && currentPracticeCommentTarget(target)) {
        practiceCommentPage.current = rollbackPracticeCommentEdit(practiceCommentPage.current, comment, text);
        setPracticeComments((current) => current && { ...current, errorKey: 'social.commentsMutationUnavailable' });
        await handleSocialOperationFailure(cause, requestSession, () => ticket.current() && currentPracticeCommentTarget(target));
      }
      throw cause;
    } finally {
      if (ticket.current() && currentPracticeCommentTarget(target)) setPracticeComments((current) => current && { ...current });
      if (practiceCommentMutationAdmissions.get(retryScope) === admission) {
        practiceCommentMutationAdmissions.delete(retryScope);
      }
    }
  }

  async function deletePracticeComment(comment: PracticeComment) {
    const target = practiceCommentTarget.current;
    const requestSession = target ? currentPracticeCommentCredential(target) : null;
    if (!target || !requestSession) throw new Error('comments_unavailable');
    const ticket = commentOperation(comment.id).issue();
    const previous = practiceCommentPage.current;
    const retryScope = `delete:${comment.id}`;
    if (practiceCommentMutationAdmissions.has(retryScope)) throw new Error('comment_delete_in_progress');
    const admission = Symbol('practice-comment-delete');
    practiceCommentMutationAdmissions.set(retryScope, admission);
    const idempotencyKey = practiceCommentMutationKey(retryScope, String(comment.version));
    practiceCommentPage.current = removePracticeComment(previous, comment.id);
    setPracticeComments((current) => current && { ...current, errorKey: undefined });
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => requestSession.token)
        .deleteSocialFeedComment(target.eventID, comment.id, idempotencyKey));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      practiceCommentMutationRetries.delete(retryScope);
    } catch (cause) {
      if (ticket.current() && currentPracticeCommentTarget(target)) {
        practiceCommentPage.current = restoreDeletedPracticeComment(practiceCommentPage.current, comment);
        setPracticeComments((current) => current && { ...current, errorKey: 'social.commentsMutationUnavailable' });
        await handleSocialOperationFailure(cause, requestSession, () => ticket.current() && currentPracticeCommentTarget(target));
      }
      throw cause;
    } finally {
      if (ticket.current() && currentPracticeCommentTarget(target)) setPracticeComments((current) => current && { ...current });
      if (practiceCommentMutationAdmissions.get(retryScope) === admission) {
        practiceCommentMutationAdmissions.delete(retryScope);
      }
    }
  }

  async function loadPracticeCommentHistory(comment: PracticeComment, cursor = '') {
    const target = practiceCommentTarget.current;
    const requestSession = target ? currentPracticeCommentCredential(target) : null;
    if (!target || !requestSession) return;
    const baseline = cursor && practiceCommentHistoryPage.current?.commentId === comment.id
      ? practiceCommentHistoryPage.current
      : { commentId: comment.id, versions: [], nextCursor: '' };
    if (cursor !== baseline.nextCursor) return;
    const ticket = practiceCommentHistoryOperations.issue();
    const historyTarget = {
      ...createSocialSessionTarget(target.ownerID, requestSession, `social:comment-history:${target.eventID}:${comment.id}`),
      commentID: comment.id,
      eventID: target.eventID,
    };
    practiceCommentHistoryTarget.current = historyTarget;
    setPracticeComments((current) => current && { ...current, history: {
      commentID: comment.id, loading: true, versions: baseline.versions,
    } });
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => requestSession.token)
        .socialFeedCommentHistory(target.eventID, comment.id, cursor || undefined));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const incoming = practiceCommentHistoryPageFromAPI(await response.json(), comment.id);
      const next = mergePracticeCommentHistoryPage(baseline, incoming, cursor);
      if (!ticket.current() || !currentPracticeCommentHistoryTarget(historyTarget) || !currentPracticeCommentTarget(target)) return;
      practiceCommentHistoryPage.current = next;
      setPracticeComments((current) => current && { ...current, history: {
        commentID: comment.id,
        loading: false,
        nextCursor: next.nextCursor || undefined,
        versions: next.versions,
      } });
    } catch (cause) {
      const currentHistoryTarget = () => ticket.current() &&
        currentPracticeCommentHistoryTarget(historyTarget) && currentPracticeCommentTarget(target);
      if (currentHistoryTarget()) await handleSocialOperationFailure(cause, requestSession, currentHistoryTarget);
      if (currentHistoryTarget()) setPracticeComments((current) => current && { ...current, history: {
        commentID: comment.id, errorKey: 'social.commentsHistoryUnavailable', loading: false, versions: baseline.versions,
      } });
    }
  }

  async function mutatePracticeCommentHeart(comment: PracticeComment) {
    const target = practiceCommentTarget.current;
    const requestSession = target ? currentPracticeCommentCredential(target) : null;
    if (!target || !requestSession || comment.pending) return;
    const previous = practiceCommentPage.current.items.find(({ id }) => id === comment.id);
    if (!previous) return;
    const hearted = !comment.heartedByViewer;
    practiceCommentPage.current = updatePracticeCommentHeartOptimistically(
      practiceCommentPage.current,
      comment.id,
      hearted,
    );
    setPracticeComments((current) => current && { ...current, errorKey: undefined });
    const currentTarget = () => currentPracticeCommentTarget(target) &&
      practiceCommentPage.current.items.some(({ id }) => id === comment.id);
    const result = await practiceCommentHeartOperations.submit(
      comment.id,
      hearted,
      async (commentID, selected, idempotencyKey) => {
        const api = createSessionApiClient(apiURL, () => requestSession.token);
        const response = generatedResponse(selected
          ? await api.setPracticeCommentHeart(target.eventID, commentID, idempotencyKey)
          : await api.removePracticeCommentHeart(target.eventID, commentID, idempotencyKey));
        if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
        const envelope = await response.json();
        if (!envelope) throw new Error('invalid_comment_heart_response');
        return practiceCommentHeartStateFromAPI(envelope.data);
      },
    );
    if (result.kind === 'superseded' || !currentTarget()) return;
    if (result.kind === 'applied') {
      practiceCommentPage.current = applyPracticeCommentHeartState(practiceCommentPage.current, result.state);
      setPracticeComments((current) => current && { ...current });
      return;
    }
    await handleSocialOperationFailure(result.cause, requestSession, currentTarget);
    if (!currentTarget()) return;
    practiceCommentPage.current = rollbackPracticeCommentHeart(
      practiceCommentPage.current,
      previous,
      hearted,
    );
    setPracticeComments((current) => current && {
      ...current,
      errorKey: 'social.commentHeartUnavailable',
    });
  }

  function currentPracticeCommentHeartRosterTarget(
    target: NonNullable<typeof practiceCommentHeartRosterTarget.current>,
  ) {
    const active = practiceCommentHeartRosterTarget.current;
    const commentsTarget = practiceCommentTarget.current;
    const lifecycle = notificationLifecycleState.current;
    return active?.commentID === target.commentID && active.eventID === target.eventID &&
      active.ownerID === target.ownerID && active.intentKey === target.intentKey &&
      active.sessionTokens.includes(target.requestSession.token) &&
      commentsTarget?.eventID === target.eventID && commentsTarget.ownerID === target.ownerID &&
      lifecycle.session !== null && active.sessionTokens.includes(lifecycle.session.token) &&
      lifecycle.destination?.kind === 'home' && lifecycle.destination.profile.id === target.ownerID;
  }

  async function loadPracticeCommentHeartRoster(
    target: NonNullable<typeof practiceCommentHeartRosterTarget.current>,
    cursor = '',
    refreshing = false,
  ) {
    const requestSession = currentSocialSessionCredential(
      practiceCommentHeartRosterTarget.current,
      target.ownerID,
      target.intentKey,
      notificationLifecycleState.current.session,
    );
    if (!requestSession || !currentPracticeCommentHeartRosterTarget(target)) return;
    let baseline = practiceCommentHeartRosterPage.current;
    if (!baseline || baseline.commentId !== target.commentID) return;
    if (refreshing) {
      baseline = invalidatePracticeCommentHeartRoster(baseline);
      practiceCommentHeartRosterPage.current = baseline;
    }
    if (cursor !== baseline.nextCursor) return;
    const requestedRevision = baseline.revision;
    const ticket = practiceCommentHeartRosterOperations.issue();
    setPracticeCommentHeartRoster((current) => current?.commentID === target.commentID ? {
      ...current,
      errorKey: undefined,
      loadingMore: Boolean(cursor),
      refreshing,
      status: cursor || (refreshing && baseline.items.length > 0) ? 'ready' : 'loading',
    } : current);
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => requestSession.token)
        .practiceCommentHearts(target.eventID, target.commentID, cursor || undefined));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const incoming = practiceCommentHeartRosterPageFromAPI(await response.json(), target.commentID);
      if (!ticket.current() || !currentPracticeCommentHeartRosterTarget(target) ||
        practiceCommentHeartRosterPage.current?.revision !== requestedRevision) return;
      const next = mergePracticeCommentHeartRosterPage(baseline, incoming, cursor, requestedRevision);
      practiceCommentHeartRosterPage.current = next;
      setPracticeCommentHeartRoster((current) => current?.commentID === target.commentID ? {
        ...current,
        loadingMore: false,
        refreshing: false,
        status: 'ready',
      } : current);
    } catch (cause) {
      if (!ticket.current() || !currentPracticeCommentHeartRosterTarget(target) ||
        practiceCommentHeartRosterPage.current?.revision !== requestedRevision) return;
      await handleSocialOperationFailure(cause, requestSession, () => ticket.current() && currentPracticeCommentHeartRosterTarget(target));
      if (ticket.current()) setPracticeCommentHeartRoster((current) => current?.commentID === target.commentID ? {
        ...current,
        errorKey: 'social.commentHeartRosterUnavailableDescription',
        loadingMore: false,
        refreshing: false,
        status: baseline.items.length > 0 ? 'ready' : 'error',
      } : current);
    }
  }

  function openPracticeCommentHeartRoster(comment: PracticeComment, navigate = true) {
    const commentsTarget = practiceCommentTarget.current;
    const requestSession = commentsTarget ? currentPracticeCommentCredential(commentsTarget) : null;
    if (!commentsTarget || !requestSession || comment.heartCount <= 0 || comment.pending) return;
    practiceCommentHeartRosterOperations.invalidate();
    const target = {
      ...createSocialSessionTarget(
        commentsTarget.ownerID,
        requestSession,
        `social:comment-hearts:${commentsTarget.eventID}:${comment.id}`,
      ),
      commentID: comment.id,
      eventID: commentsTarget.eventID,
    };
    practiceCommentHeartRosterTarget.current = target;
    practiceCommentHeartRosterPage.current = {
      commentId: comment.id,
      items: [],
      nextCursor: '',
      revision: 0,
    };
    setPracticeCommentHeartRoster({
      commentID: comment.id,
      eventID: commentsTarget.eventID,
      loadingMore: false,
      refreshing: false,
      status: 'loading',
    });
    prepareCommentHeartRosterRoute();
    if (navigate) router.push({
        pathname: '/following/comments/[eventID]/hearts/[commentID]',
        params: { commentID: comment.id, eventID: commentsTarget.eventID },
      });
    void loadPracticeCommentHeartRoster(target);
  }

  async function openSocialFeedActivity(event: PracticeSessionFeedEvent, navigate = true) {
    if (!session || destination?.kind !== 'home') return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = socialFeedActivityOperations.issue();
    socialFeedActivityTarget.current = {
      activityID: event.activity.id,
      ownerID,
      pathID: event.path.id,
      session: currentSession,
    };
    setSocialFeedActivity(null);
    setSocialFeed((current) => ({ ...current, detailErrorKey: undefined }));

    const currentTarget = () => ticket.current() &&
      socialFeedActivityTarget.current?.activityID === event.activity.id &&
      socialFeedActivityTarget.current?.ownerID === ownerID &&
      socialFeedActivityTarget.current?.pathID === event.path.id &&
      socialFeedActivityTarget.current?.session === currentSession;
    const result = await prepareSocialFeedActivityDetail({
      event,
      load: (pathID, activityID) => validateSessionCredential<ActivityDetail>(
        currentSession,
        async (credential) => generatedResponse(
          await createSessionApiClient(apiURL, () => credential.token).activity(pathID, activityID),
        ),
      ),
      loadRevisions: (pathID, activityID) => loadRevisionPage(currentSession, pathID, activityID),
      commitRoute: (pathID, activityID) => {
        if (!currentTarget() || !navigate) return;
        router.push({
          pathname: '/following/activity/[pathID]/[activityID]',
          params: { activityID, pathID },
        });
      },
      publish: (detail, revisions, nextCursor) => {
        if (currentTarget()) {
          setSocialFeedActivity({
            busy: false,
            detail,
            event,
            nextCursor: nextCursor ?? undefined,
            revisions,
          });
          setSocialFeed((current) => ({ ...current, detailErrorKey: undefined }));
        }
      },
    });
    if (result.kind === 'failed' && currentTarget()) {
      await handleFeatureSessionFailure(result.cause, currentSession, ticket);
      if (currentTarget()) setSocialFeed((current) => ({
        ...current,
        detailErrorKey: 'social.feedUnavailableDescription',
      }));
    }
  }

  async function loadMoreSocialFeedActivityRevisions() {
    if (!session || destination?.kind !== 'home' || !socialFeedActivity?.nextCursor ||
      socialFeedActivity.busy) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const event = socialFeedActivity.event;
    const cursor = socialFeedActivity.nextCursor;
    const ticket = socialFeedActivityOperations.issue();
    socialFeedActivityTarget.current = {
      activityID: event.activity.id,
      ownerID,
      pathID: event.path.id,
      session: currentSession,
    };
    setSocialFeedActivity((current) => current ? { ...current, busy: true, errorKey: undefined } : current);
    try {
      const page = await loadRevisionPage(currentSession, event.path.id, event.activity.id, cursor);
      if (!ticket.current() || socialFeedActivityTarget.current?.activityID !== event.activity.id ||
        socialFeedActivityTarget.current?.ownerID !== ownerID ||
        socialFeedActivityTarget.current?.pathID !== event.path.id ||
        socialFeedActivityTarget.current?.session !== currentSession) return;
      setSocialFeedActivity((current) => current && current.event.id === event.id ? {
        ...current,
        busy: false,
        nextCursor: page.nextCursor ?? undefined,
        revisions: appendUniqueRevisions(current.revisions, page.items),
      } : current);
    } catch (cause) {
      if (!ticket.current()) return;
      await handleFeatureSessionFailure(cause, currentSession, ticket);
      if (ticket.current()) setSocialFeedActivity((current) => current && current.event.id === event.id ? {
        ...current,
        busy: false,
        errorKey: 'errors.temporarilyUnavailable',
      } : current);
    }
  }

  async function openNotification(notification: PathInvitationNotification) {
    if (await mutateNotification({ kind: 'read', notificationId: notification.id }, notification)) {
      await openNotificationContext(notification);
    }
  }

  async function openNotificationContext(notification: PathInvitationNotification) {
    if (destination?.kind !== 'home') return;
    if (notification.type === 'path_deleted' || notification.type === 'path_member_removed') return;
    if (notification.type === 'follow_request_received') {
      await loadSocialFollowRequests(true);
      router.push('/follow-requests');
      return;
    }
    if (notification.type === 'new_follower' || notification.type === 'follow_request_accepted') {
      await loadSocialProfile(notification.actor.username);
      router.push({ pathname: '/profile/[username]', params: { username: notification.actor.username } });
      return;
    }
    if (notification.type === 'path_invitation_received') {
      await openPendingInvitations(notification.invitationId);
      return;
    }
    if (notification.type === 'practice_comment' || notification.type === 'comment_heart') {
      openPracticeComments(notification.socialFeedEventId, destination.profile.id, notification.commentId);
      return;
    }
    if (notification.type === 'nudge_received') {
      const path = destination.profile.paths.find((item) => item.id === notification.pathId)
        ?? destination.profile.archivedPaths.find((item) => item.id === notification.pathId);
      if (path) openPathDetail(path.id);
      else {
        closeNotificationsRoute();
        setErrorKey('errors.notFound');
        router.replace('/(tabs)/home');
      }
      return;
    }
    if (notification.type === 'path_ownership_transfer_received') {
      const path = destination.profile.paths.find((item) => item.id === notification.pathId);
      if (path) openPathDetail(path.id, notification.ownershipTransferId);
      return;
    }
    const path = destination.profile.paths.find((item) => item.id === notification.pathId)
      ?? destination.profile.archivedPaths.find((item) => item.id === notification.pathId);
    if (path) {
      openPathDetail(path.id);
    }
  }

  function openPathSharing(path: SessionPath) {
    const capabilities = effectivePathCapabilities(path);
    if (!session || destination?.kind !== 'home' || pathRenamePathID === path.id || ownershipTransferPathID === path.id ||
      !(capabilities.inviteMembers || capabilities.manageMembers || capabilities.manageVisibility)) return;
    resetInvitationShare();
    invitationTarget.current = createPathAdministrationTarget(destination.profile.id, path.id, session);
    setPathVisibilityDraft(path.visibility);
    setPathVisibilityReview(null);
    setPathVisibilityErrorKey(null);
    setPathVisibilitySaved(false);
    setSharingPath(true);
    void openPathMembers(path.id, undefined, false, false);
    if (capabilities.inviteMembers) {
      void loadManagedInvitations(path.id, '', session, destination.profile.id);
    }
  }

  function closePathSharing(dirty = false) {
    if (invitationReviewBusy || invitationSendBusy || pathVisibilityBusy ||
      Object.values(managedInvitationBusy).some(Boolean)) return;
    if (dirty) {
      presentNativeDestructiveConfirmation({
        cancelLabel: i18n.t('common.cancel'),
        confirmLabel: i18n.t('pathInvitation.discard'),
        message: i18n.t('pathInvitation.discardDescription'),
        onConfirm: resetInvitationShare,
        title: i18n.t('pathInvitation.discardTitle'),
      });
      return;
    }
    resetInvitationShare();
  }

  function changeInvitationUsername(username: string) {
    invitationReviewOwner.cancel();
    invitationSendOwner.cancel(selectedPathID ?? undefined);
    setInvitationUsername(username);
    setInvitationReview(null);
    setInvitationErrorKey(null);
    setInvitationSent(false);
  }

  async function reviewInvitationRecipient() {
    if (!session || destination?.kind !== 'home' || !selectedPath || !sharingPath || invitationReviewBusy) return;
    const path = destination.profile.paths.find((candidate) => candidate.id === selectedPath.id);
    if (!path || !effectivePathCapabilities(path).inviteMembers) return;
    const exactUsername = invitationUsername;
    if (!exactUsername || exactUsername.trim() !== exactUsername) {
      setInvitationErrorKey('pathInvitation.unavailable');
      return;
    }
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = path.id;
    invitationTarget.current = createPathAdministrationTarget(ownerID, pathID, currentSession);
    setInvitationReviewBusy(true);
    setInvitationErrorKey(null);
    setInvitationSent(false);
    const result = await invitationReviewOwner.review(pathID, exactUsername, async () =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .reviewPathInvitationRecipient(pathID, exactUsername))),
    );
    if (!ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) return;
    setInvitationReviewBusy(false);
    const currentPath = destination.kind === 'home'
      ? destination.profile.paths.find((candidate) => candidate.id === pathID)
      : undefined;
    if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) {
      resetInvitationShare();
      return;
    }
    if (result.kind === 'reviewed') setInvitationReview(result.review);
    else if (result.kind === 'failed') setInvitationErrorKey(pathInvitationFailureMessageKey(result.failure));
  }

  function chooseInvitationRole(role: PathInvitationRole) {
    if (!selectedPath || !effectivePathCapabilities(selectedPath).inviteMembers || invitationSendBusy || role === invitationRole) return;
    invitationSendOwner.cancel(selectedPath.id);
    setInvitationRole(role);
    setInvitationErrorKey(null);
    setInvitationSent(false);
  }

  async function loadManagedInvitations(
    pathID: string,
    cursor = '',
    currentSession = session,
    ownerID = destination?.kind === 'home' ? destination.profile.id : undefined,
    replaceInFlight = false,
  ) {
    if (!currentSession || !ownerID || (managedInvitationsBusy && !replaceInFlight)) return;
    if (!ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) return;
    if (replaceInFlight) managedInvitationListOperations.invalidate();
    const ticket = managedInvitationListOperations.issue();
    setManagedInvitationsBusy(true);
    setManagedInvitationsErrorKey(undefined);
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .managedPathInvitations(pathID, cursor || undefined));
      if (!response.ok) throw pathInvitationFailureFromProblem(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw { kind: 'invalid_response' } as const;
      if (!ticket.current() ||
        !ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) return;
      const page: ManagedPendingPathInvitationPage = {
        items: envelope.data as ManagedPendingPathInvitation[],
        nextCursor: envelope.meta.nextCursor ?? '',
      };
      const next = mergeManagedPendingInvitationPage(managedInvitationState, page, cursor);
      setManagedInvitationState(next);
    } catch (cause) {
      if (ticket.current() &&
        ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) {
        setManagedInvitationsErrorKey(invitationFailureKey(cause));
      }
    } finally {
      if (ticket.current() &&
        ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) {
        setManagedInvitationsBusy(false);
      }
    }
  }

  async function cancelManagedInvitation(managed: ManagedPendingPathInvitation) {
    if (!session || destination?.kind !== 'home' || !sharingPath) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = managed.invitation.pathId;
    const invitationID = managed.invitation.id;
    const currentPath = destination.profile.paths.find((candidate) => candidate.id === pathID);
    if (
      !currentPath ||
      !effectivePathCapabilities(currentPath).inviteMembers ||
      !ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token) ||
      !managedInvitationState.items.some((candidate) => candidate.invitation.id === invitationID) ||
      managedInvitationBusy[invitationID]
    ) return;
    setManagedInvitationBusy((busy) => ({ ...busy, [invitationID]: true }));
    setManagedInvitationErrors((errors) => ({ ...errors, [invitationID]: undefined }));
    const result = await invitationCancelOwner.submit(
      pathID,
      invitationID,
      true,
      async (_pathID, _invitationID, idempotencyKey) => pathInvitationOutputData(
        await invitationResponse(await createSessionApiClient(apiURL, () => currentSession.token)
          .cancelPathInvitation(pathID, invitationID, idempotencyKey)),
      ),
    );
    if (!ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) return;
    setManagedInvitationBusy((busy) => ({ ...busy, [invitationID]: false }));
    if (result.kind === 'canceled') {
      managedInvitationListOperations.invalidate();
      setManagedInvitationsBusy(false);
      setManagedInvitationState((current) => ({
        ...current,
        items: current.items.filter((candidate) => candidate.invitation.id !== invitationID),
      }));
      void loadManagedInvitations(pathID, '', currentSession, ownerID, true);
    } else if (result.kind === 'failed') {
      setManagedInvitationErrors((errors) => ({
        ...errors,
        [invitationID]: pathInvitationFailureMessageKey(result.failure),
      }));
    }
  }

  async function sendReviewedInvitation() {
    if (!session || destination?.kind !== 'home' || !selectedPath || !invitationReview || !sharingPath || invitationSendBusy) return;
    const path = destination.profile.paths.find((candidate) => candidate.id === selectedPath.id);
    if (!path || !effectivePathCapabilities(path).inviteMembers || invitationReview.pathId !== path.id) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = path.id;
    const review = invitationReview;
    invitationTarget.current = createPathAdministrationTarget(ownerID, pathID, currentSession);
    setInvitationSendBusy(true);
    setInvitationErrorKey(null);
    const result = await invitationSendOwner.submit(review, invitationRole, true, async (_pathID, body, idempotencyKey) =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .sendPathInvitation(pathID, body, idempotencyKey))),
    );
    if (!ownsPathAdministrationTarget(invitationTarget.current, ownerID, pathID, currentSession.token)) return;
    setInvitationSendBusy(false);
    const currentPath = destination.profile.paths.find((candidate) => candidate.id === pathID);
    if (!currentPath || !effectivePathCapabilities(currentPath).inviteMembers) {
      resetInvitationShare();
      return;
    }
    if (result.kind === 'sent') {
      setInvitationSent(true);
      void loadManagedInvitations(pathID, '', currentSession, ownerID, true);
    }
    else if (result.kind === 'failed') setInvitationErrorKey(pathInvitationFailureMessageKey(result.failure));
  }

  function beginPendingInvitationAcceptance(pending: PendingPathInvitation) {
    if (!session || destination?.kind !== 'home' || pendingInvitationBusy[pending.invitation.id]) return;
    const current = destination.profile.pendingInvitations.items.find(
      (candidate) => candidate.invitation.id === pending.invitation.id,
    );
    if (current !== pending) return;
    setRejectedInvitationID(null);
    const review = reviewPendingPathInvitationAcceptance(pending);
    setPendingInvitationErrors((errors) => ({ ...errors, [review.invitationId]: undefined }));
    if (review.kind === 'confirmation-required') {
      invitationAcceptanceTargets.current.set(review.invitationId, {
        ownerID: destination.profile.id,
        session,
      });
      setPendingInvitationAcceptanceReview(review);
      return;
    }
    void submitPendingInvitationAcceptance(review);
  }

  function cancelPendingInvitationAcceptance() {
    const review = pendingInvitationAcceptanceReview;
    if (!review || pendingInvitationBusy[review.invitationId]) return;
    invitationAcceptOwner.cancel(review.invitationId);
    invitationAcceptanceTargets.current.delete(review.invitationId);
    setPendingInvitationAcceptanceReview(null);
  }

  function invitationDecisionTarget(invitationID: string) {
    return invitationAcceptanceTargets.current.get(invitationID) ??
      invitationRejectionTargets.current.get(invitationID);
  }

  async function refreshPendingInvitationProjection(
    currentSession: Session,
    ownerID: string,
    invitationID: string,
  ) {
    setPendingInvitationsBusy(true);
    setPendingInvitationsErrorKey(null);
    try {
      const page = await loadPendingInvitationPage(currentSession);
      const target = invitationDecisionTarget(invitationID);
      if (target?.session !== currentSession || target.ownerID !== ownerID) return;
      const pendingInvitations = mergePendingInvitationPage({ items: [], nextCursor: '' }, page, '');
      setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
        ? {
            ...current,
            profile: { ...current.profile, pendingInvitations },
          }
        : current);
    } catch (cause) {
      const target = invitationDecisionTarget(invitationID);
      if (target?.session === currentSession && target.ownerID === ownerID) {
        setPendingInvitationsErrorKey(invitationFailureKey(cause));
      }
    } finally {
      const target = invitationDecisionTarget(invitationID);
      if (target?.session === currentSession && target.ownerID === ownerID) {
        setPendingInvitationsBusy(false);
      }
    }
  }

  async function refreshUnavailableInvitationProjections(
    currentSession: Session,
    ownerID: string,
    invitationID: string,
  ) {
    await refreshPendingInvitationProjection(currentSession, ownerID, invitationID);
    if (!invitationDecisionTarget(invitationID)) return;
    try {
      await refreshPushNotificationHistory(currentSession, ownerID);
    } catch {
      const target = invitationDecisionTarget(invitationID);
      if (target?.session === currentSession && target.ownerID === ownerID) {
        setNotificationsErrorKey('notification.error');
      }
    }
  }

  async function submitPendingInvitationAcceptance(review: PathInvitationAcceptanceReview) {
    const invitationID = review.invitationId;
    if (!session || destination?.kind !== 'home' || pendingInvitationBusy[invitationID]) return;
    const invitation = destination.profile.pendingInvitations.items.find(
      (candidate) => candidate.invitation.id === invitationID,
    );
    if (!invitation) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    invitationAcceptanceTargets.current.set(invitationID, { ownerID, session: currentSession });
    setPendingInvitationBusy((current) => ({ ...current, [invitationID]: true }));
    setPendingInvitationErrors((current) => ({ ...current, [invitationID]: undefined }));
    const result = await invitationAcceptOwner.submit(review, true, async (id, idempotencyKey, body) =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .acceptPathInvitation(id, idempotencyKey, body))),
    );
    const target = invitationAcceptanceTargets.current.get(invitationID);
    if (target?.session !== currentSession || target.ownerID !== ownerID) return;
    setPendingInvitationBusy((current) => ({ ...current, [invitationID]: false }));
    if (result.kind === 'failed') {
      const errorKey = pathInvitationFailureMessageKey(result.failure);
      setPendingInvitationErrors((current) => ({
        ...current,
        [invitationID]: errorKey,
      }));
      if (errorKey === 'pathInvitation.warningRequired') {
        invitationAcceptOwner.cancel(invitationID);
        setPendingInvitationAcceptanceReview(null);
        await refreshPendingInvitationProjection(currentSession, ownerID, invitationID);
      } else if (result.failure.kind === 'opaque') {
        setPendingInvitationAcceptanceReview(null);
        await refreshUnavailableInvitationProjections(currentSession, ownerID, invitationID);
      }
      invitationAcceptanceTargets.current.delete(invitationID);
      return;
    }
    invitationAcceptanceTargets.current.delete(invitationID);
    if (result.kind !== 'accepted') return;
    setPendingInvitationAcceptanceReview(null);
    setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
      ? {
          ...current,
          profile: {
            ...current.profile,
            pendingInvitations: {
              ...current.profile.pendingInvitations,
              items: current.profile.pendingInvitations.items.filter((candidate) => candidate.invitation.id !== invitationID),
            },
          },
        }
      : current);
    setAcceptedInvitation({ role: result.invitation.offeredRole });
    const ticket = sessionOperations.issue();
    await activate(currentSession, sessionRenewable, ticket, currentSession.token);
  }

  function beginPendingInvitationRejection(pending: PendingPathInvitation) {
    const invitationID = pending.invitation.id;
    if (
      !session ||
      destination?.kind !== 'home' ||
      pendingInvitationBusy[invitationID] ||
      pendingInvitationAcceptanceReview?.invitationId === invitationID ||
      !destination.profile.pendingInvitations.items.some(
        (candidate) => candidate === pending,
      )
    ) return;
    setPendingInvitationErrors((current) => ({ ...current, [invitationID]: undefined }));
    setAcceptedInvitation(null);
	presentNativeDestructiveConfirmation({
		title: i18n.t('pathInvitation.rejectConfirmationHeading'),
		message: i18n.t('pathInvitation.rejectConfirmationBody', {
			displayName: pending.inviter.displayName,
			pathName: pending.pathName,
		}),
		cancelLabel: i18n.t('common.cancel'),
		confirmLabel: i18n.t('pathInvitation.reject'),
		onConfirm: () => void submitPendingInvitationRejection(invitationID),
	});
  }

  async function submitPendingInvitationRejection(invitationID: string) {
    if (!session || destination?.kind !== 'home' || pendingInvitationBusy[invitationID]) return;
    if (!destination.profile.pendingInvitations.items.some(
      (candidate) => candidate.invitation.id === invitationID,
    )) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    invitationRejectionTargets.current.set(invitationID, { ownerID, session: currentSession });
    setPendingInvitationBusy((current) => ({ ...current, [invitationID]: true }));
    setPendingInvitationRejecting((current) => ({ ...current, [invitationID]: true }));
    setPendingInvitationErrors((current) => ({ ...current, [invitationID]: undefined }));
    setRejectedInvitationID(null);
    const result = await invitationRejectOwner.submit(invitationID, true, async (id, idempotencyKey) =>
      pathInvitationOutputData(await invitationResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .rejectPathInvitation(id, idempotencyKey))),
    );
    const target = invitationRejectionTargets.current.get(invitationID);
    if (target?.session !== currentSession || target.ownerID !== ownerID) return;
    setPendingInvitationBusy((current) => ({ ...current, [invitationID]: false }));
    setPendingInvitationRejecting((current) => ({ ...current, [invitationID]: false }));
    if (result.kind === 'failed') {
      setPendingInvitationErrors((current) => ({
        ...current,
        [invitationID]: pathInvitationFailureMessageKey(result.failure),
      }));
      if (result.failure.kind === 'opaque') {
        await refreshUnavailableInvitationProjections(currentSession, ownerID, invitationID);
      }
      invitationRejectionTargets.current.delete(invitationID);
      return;
    }
    invitationRejectionTargets.current.delete(invitationID);
    if (result.kind !== 'rejected') return;
    setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
      ? {
          ...current,
          profile: {
            ...current.profile,
            pendingInvitations: {
              ...current.profile.pendingInvitations,
              items: current.profile.pendingInvitations.items.filter(
                (candidate) => candidate.invitation.id !== invitationID,
              ),
            },
          },
        }
      : current);
    setNotificationHistory((current) => ({
      ...current,
      items: current.items.filter((notification) =>
        notification.type !== 'path_invitation_received' ||
        notification.invitationId !== invitationID),
    }));
    setNotificationUnreadCount(result.rejection.unreadCount);
    setRejectedInvitationID(invitationID);
    await setNativeNotificationBadge(result.rejection.unreadCount).catch(() => false);
  }

  function ownershipTransferCandidate(candidate: GeneratedOwnershipTransferCandidate): OwnershipTransferCandidate {
    return {
      displayName: candidate.displayName,
      isAdministrator: candidate.administrator,
      userID: candidate.userId,
      username: candidate.username || undefined,
    };
  }

  function ownershipTransferKey(action: OwnershipTransferBusyAction, subjectID: string): string {
    const key = `${action}:${subjectID}`;
    const existing = ownershipTransferIdempotencyKeys.current.get(key);
    if (existing) return existing;
    const created = Crypto.randomUUID();
    ownershipTransferIdempotencyKeys.current.set(key, created);
    return created;
  }

  function pendingOwnershipTransferPresentation(
    transfer: OwnershipTransferResult,
  ): PendingOwnershipTransfer | undefined {
    const viewerRole = transfer.counterpartRole === 'recipient' ? 'creator' : 'recipient';
    const counterpart = {
      displayName: transfer.counterpart.displayName,
      isAdministrator: false,
      userID: transfer.counterpart.userId,
      username: transfer.counterpart.username || undefined,
    };
    const expiration = ownershipTransferExpirationPresentation(
      transfer.transfer.reviewedAt,
      transfer.transfer.expiresAt,
      transfer.viewerTimeZone,
      i18n,
    );
    if (!expiration) return undefined;
    return { counterpart, expirationSummary: expiration.summary, id: transfer.transfer.id, viewerRole };
  }

  async function fetchPendingOwnershipTransfer(
    currentSession: Session,
    pathID: string,
  ): Promise<{ transfer?: OwnershipTransferResult }> {
    return validateSessionCredential(currentSession, async (credential) => {
      const api = createSessionApiClient(apiURL, () => credential.token);
      const pendingResult = await api.pendingOwnershipTransfer(pathID);
      if (pendingResult.response.status === 404) return {
        ok: true,
        status: 200,
        json: async () => ({ data: {} }),
      };
      const pending = generatedResponse(pendingResult);
      if (!pending.ok) return pending;
      const pendingEnvelope = await pending.json();
      if (!pendingEnvelope || !validOwnershipTransferProjection(pendingEnvelope.data, pathID)) {
        throw { kind: 'invalid_response' } as const;
      }
      return {
        ok: true,
        status: 200,
        json: async () => ({
          data: { transfer: pendingEnvelope.data },
        }),
      };
    });
  }

  async function loadOwnershipTransferCandidates(cursor = '') {
    if (!session || destination?.kind !== 'home' || !ownershipTransferPathID || ownershipTransferCandidatesLoading) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = ownershipTransferPathID;
    const path = destination.profile.paths.find(({ id }) => id === pathID);
    if (!path || !effectivePathCapabilities(path).transferOwnership) return;
    const target = ownershipTransferTarget.current;
    if (!ownsPathAdministrationTarget(target, ownerID, pathID, currentSession.token)) return;
    setOwnershipTransferCandidatesLoading(true);
    setOwnershipTransferCandidatesErrorKey(undefined);
    try {
      const page = await validateSessionCredential<{ items: GeneratedOwnershipTransferCandidate[]; nextCursor: string }>(currentSession, async (credential) => {
        const result = await createSessionApiClient(apiURL, () => credential.token)
          .ownershipTransferCandidates(pathID, cursor || undefined);
        return generatedResponse({
          response: result.response,
          error: result.error,
          data: result.data ? { data: { items: result.data.data, nextCursor: result.data.meta.nextCursor ?? '' } } : undefined,
        });
      });
      if (ownershipTransferTarget.current !== target || !currentAdministrationSession(target, pathID)) return;
      const mapped = page.items.map(ownershipTransferCandidate);
      setOwnershipTransferCandidates((current) => cursor
        ? [...current, ...mapped.filter(({ userID }) => !current.some((item) => item.userID === userID))]
        : mapped);
      setOwnershipTransferCandidatesCursor(page.nextCursor);
      setOwnershipTransferCandidatesLoaded(true);
    } catch (cause) {
      if (ownershipTransferTarget.current !== target) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile');
      }
      else setOwnershipTransferCandidatesErrorKey(localizedFailure(failure, 'pathOwnership.unavailable'));
    } finally {
      if (ownershipTransferTarget.current === target) setOwnershipTransferCandidatesLoading(false);
    }
  }

  async function openOwnershipTransfer(path: SessionPath, showWhenEmpty = true, expectedTransferID?: string, fromManagement = false) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || ownershipTransferBusyAction ||
      (!fromManagement && (goalManagementPathID === path.id || pathRenamePathID === path.id)) ||
      manualPathID === path.id || pathArchiveReview?.pathId === path.id || timerBusy[path.id] || sharingPath) return;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const pathID = path.id;
    const ticket = ownershipTransferOperations.issue();
    const target = createPathAdministrationTarget(ownerID, pathID, currentSession);
    ownershipTransferTarget.current = target;
    setOwnershipTransferPathID(pathID);
    setOwnershipTransferOpen(showWhenEmpty);
    setPendingOwnershipTransferLoading(true);
    setOwnershipTransferErrorKey(null);
    try {
      const loaded = await fetchPendingOwnershipTransfer(currentSession, pathID);
      if (!ticket.current() || ownershipTransferTarget.current !== target) return;
      const pending = loaded.transfer && loaded.transfer.transfer.state === 'pending' &&
        (!expectedTransferID || loaded.transfer.transfer.id === expectedTransferID)
        ? pendingOwnershipTransferPresentation(loaded.transfer)
        : undefined;
      setPendingOwnershipTransfer(pending);
      setOwnershipTransferExpirationSummary(pending?.expirationSummary);
      if (pending) setOwnershipTransferOpen(true);
      else if (!showWhenEmpty) {
        ownershipTransferTarget.current = null;
        setOwnershipTransferPathID(null);
      }
    } catch (cause) {
      if (!ticket.current() || ownershipTransferTarget.current !== target) return;
      if (showWhenEmpty) setOwnershipTransferErrorKey(localizedFailure(cause, 'pathOwnership.unavailable'));
      else {
        ownershipTransferTarget.current = null;
        setOwnershipTransferPathID(null);
      }
    } finally {
      if (ticket.current()) setPendingOwnershipTransferLoading(false);
    }
  }

  async function refreshAfterOwnershipTransfer(
    target: PathAdministrationTarget<Session>,
    ownerID: string,
    pathID: string,
  ) {
    const currentSession = currentAdministrationSession(target, pathID);
    if (!currentSession) return false;
    const [profile, notificationPage] = await Promise.all([
      loadMobileHomeProfile(apiURL, currentSession),
      loadNotificationHistoryPage(currentSession),
    ]);
    if (ownershipTransferTarget.current !== target || !currentAdministrationSession(target, pathID)) return false;
    setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
      ? { ...current, profile }
      : current);
    const history = mergeNotificationHistoryPage(
      { items: [], nextCursor: '', unreadCount: notificationPage.unreadCount },
      notificationPage,
      '',
    );
    setNotificationHistory(history);
    setNotificationUnreadCount(history.unreadCount);
    void setNativeNotificationBadge(history.unreadCount).catch(() => undefined);
    return true;
  }

  async function reviewOwnershipTransferRecipient(recipient: OwnershipTransferCandidate) {
    if (!session || destination?.kind !== 'home' || !ownershipTransferOpen || !ownershipTransferPathID ||
      ownershipTransferBusyAction || ownershipTransferReviewBusy || pendingOwnershipTransferLoading ||
      pendingOwnershipTransfer) return;
    const path = destination.profile.paths.find(({ id }) => id === ownershipTransferPathID);
    if (!path || !effectivePathCapabilities(path).transferOwnership) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = path.id;
    const target = ownershipTransferTarget.current;
    if (!ownsPathAdministrationTarget(target, ownerID, pathID, currentSession.token)) return;
    const ticket = ownershipTransferReviewOperations.issue();
    setOwnershipTransferSelectedRecipient(recipient);
    setOwnershipTransferReview(null);
    setOwnershipTransferExpirationSummary(undefined);
    setOwnershipTransferReviewBusy(true);
    setOwnershipTransferErrorKey(null);
    try {
      const review = await validateSessionCredential<OwnershipTransferReview>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).reviewOwnershipTransfer(
          pathID,
          { recipientUserId: recipient.userID },
        ),
      ));
      if (!ticket.current() || ownershipTransferTarget.current !== target || !currentAdministrationSession(target, pathID)) return;
      if (!validReviewedOwnershipTransfer(review)) throw { kind: 'invalid_response' } as const;
      const canonicalRecipient: OwnershipTransferCandidate = {
        displayName: review.recipient.displayName,
        isAdministrator: false,
        userID: review.recipient.userId,
        username: review.recipient.username || undefined,
      };
      const expiration = ownershipTransferExpirationPresentation(
        review.reviewedAt,
        review.expiresAt,
        review.viewerTimeZone,
        i18n,
      );
      if (!expiration) throw { kind: 'invalid_response' } as const;
      setOwnershipTransferCandidates((current) => [
        canonicalRecipient,
        ...current.filter(({ userID }) => userID !== canonicalRecipient.userID),
      ]);
      setOwnershipTransferSelectedRecipient(canonicalRecipient);
      setOwnershipTransferReview({ ...review, idempotencyKey: Crypto.randomUUID() });
      setOwnershipTransferExpirationSummary(expiration.summary);
    } catch (cause) {
      if (!ticket.current() || ownershipTransferTarget.current !== target) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile', ticket);
      }
      else setOwnershipTransferErrorKey(localizedFailure(failure, 'pathOwnership.unavailable'));
    } finally {
      if (ticket.current() && ownershipTransferTarget.current === target) setOwnershipTransferReviewBusy(false);
    }
  }

  async function confirmOwnershipTransfer(recipient: OwnershipTransferCandidate) {
    if (!session || destination?.kind !== 'home' || !ownershipTransferOpen || !ownershipTransferPathID ||
      ownershipTransferBusyAction || ownershipTransferReviewBusy ||
      ownershipTransferSelectedRecipient?.userID !== recipient.userID ||
      ownershipTransferReview?.recipient.userId !== recipient.userID) return;
    const path = destination.profile.paths.find(({ id }) => id === ownershipTransferPathID);
    if (!path || !effectivePathCapabilities(path).transferOwnership) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = path.id;
    const target = ownershipTransferTarget.current;
    if (!ownsPathAdministrationTarget(target, ownerID, pathID, currentSession.token)) return;
    const review = ownershipTransferReview;
    const idempotencyKey = review.idempotencyKey;
    setOwnershipTransferBusyAction('confirm');
    setOwnershipTransferErrorKey(null);
    try {
      const result = await validateSessionCredential<OwnershipTransferResult>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).initiateOwnershipTransfer(
          pathID,
          { reservationToken: review.reservationToken },
          idempotencyKey,
        ),
      ));
      if (ownershipTransferTarget.current !== target || !currentAdministrationSession(target, pathID)) return;
      if (!validOwnershipTransferProjection(result, pathID) || result.transfer.state !== 'pending') {
        throw { kind: 'invalid_response' } as const;
      }
      const pending = pendingOwnershipTransferPresentation(result);
      if (!pending) throw { kind: 'invalid_response' } as const;
      if (!await refreshAfterOwnershipTransfer(target, ownerID, pathID)) return;
      setOwnershipTransferReview(null);
      setPendingOwnershipTransfer(pending);
      setOwnershipTransferExpirationSummary(pending.expirationSummary);
    } catch (cause) {
      if (ownershipTransferTarget.current !== target) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile');
      }
      else setOwnershipTransferErrorKey(localizedFailure(failure, 'pathOwnership.unavailable'));
    } finally {
      if (ownershipTransferTarget.current === target) setOwnershipTransferBusyAction(undefined);
    }
  }

  async function mutatePendingOwnershipTransfer(action: Exclude<OwnershipTransferBusyAction, 'confirm'>) {
    if (!session || destination?.kind !== 'home' || !pendingOwnershipTransfer || !ownershipTransferPathID || ownershipTransferBusyAction) return;
    if ((action === 'cancel') !== (pendingOwnershipTransfer.viewerRole === 'creator')) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = ownershipTransferPathID;
    if (!destination.profile.paths.some(({ id }) => id === pathID)) return;
    const transferID = pendingOwnershipTransfer.id;
    const target = ownershipTransferTarget.current;
    if (!ownsPathAdministrationTarget(target, ownerID, pathID, currentSession.token)) return;
    const idempotencyKey = ownershipTransferKey(action, transferID);
    setOwnershipTransferBusyAction(action);
    setOwnershipTransferErrorKey(null);
    try {
      const api = createSessionApiClient(apiURL, () => currentSession.token);
      const result = await validateSessionCredential<OwnershipTransferResult>(currentSession, async () => generatedResponse(
        action === 'accept'
          ? await api.acceptOwnershipTransfer(transferID, idempotencyKey)
          : action === 'decline'
            ? await api.declineOwnershipTransfer(transferID, idempotencyKey)
            : await api.cancelOwnershipTransfer(transferID, idempotencyKey),
      ));
      const expectedState = action === 'accept' ? 'accepted' : action === 'decline' ? 'declined' : 'canceled';
      if (!validOwnershipTransferProjection(result, pathID) ||
        result.transfer.id !== transferID || result.transfer.state !== expectedState) {
        throw { kind: 'invalid_response' } as const;
      }
      if (ownershipTransferTarget.current !== target || !currentAdministrationSession(target, pathID) ||
        !await refreshAfterOwnershipTransfer(target, ownerID, pathID)) return;
      ownershipTransferIdempotencyKeys.current.delete(`${action}:${transferID}`);
      setPendingOwnershipTransfer(undefined);
      setOwnershipTransferExpirationSummary(undefined);
      setOwnershipTransferOpen(false);
      ownershipTransferTarget.current = null;
      setOwnershipTransferPathID(null);
    } catch (cause) {
      if (ownershipTransferTarget.current !== target) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile');
      }
      else setOwnershipTransferErrorKey(localizedFailure(failure, 'pathOwnership.unavailable'));
    } finally {
      if (ownershipTransferTarget.current === target) setOwnershipTransferBusyAction(undefined);
    }
  }

  function closeOwnershipTransfer() {
    if (ownershipTransferBusyAction || ownershipTransferReviewBusy || pendingOwnershipTransferLoading) return;
    ownershipTransferOperations.invalidate();
    ownershipTransferReviewOperations.invalidate();
    setOwnershipTransferOpen(false);
    setOwnershipTransferErrorKey(null);
  }

  function openPathManagement(path: SessionPath) {
    if (destination?.kind !== 'home') return;
    const capabilities = effectivePathCapabilities(path);
    const hasPendingTransfer = ownershipTransferPathID === path.id && Boolean(pendingOwnershipTransfer);
    if (!(capabilities.manageGoals || capabilities.renamePath || capabilities.manageLifecycle || capabilities.transferOwnership || hasPendingTransfer) ||
      manualBusy || timerBusy[path.id] || pathArchiveReview?.pathId === path.id ||
      (ownershipTransferPathID === path.id && !hasPendingTransfer)) return;
    resetGoalManagement();
    resetPathRename();
    goalManagementOwnerID.current = destination.profile.id;
    setGoalManagementPathID(path.id);
    setGoalManagementCurrent(path);
    setGoalManagementForm(pathGoalFormFromPath(path));
    if (capabilities.renamePath) {
      setPathRenamePathID(path.id);
      setPathRenameName(path.name);
    }
  }

  function updateGoalManagementForm(update: Partial<PathGoalForm>) {
    if (goalManagementBusy || pathRenameBusy || pathDeletionBusy) return;
    setGoalManagementForm((current) => current ? { ...current, ...update } : current);
    setGoalManagementReview(null);
    setGoalManagementErrorKey(null);
    setGoalManagementSaved(false);
  }

  function reviewPathGoalChanges() {
    if (!goalManagementForm || !goalManagementCurrent || !effectivePathCapabilities(goalManagementCurrent).manageGoals || goalManagementBusy) return;
    const prepared = buildPathGoalUpdateDraft(goalManagementForm);
    if (prepared.kind === 'invalid_duration') { setGoalManagementErrorKey('pathCreate.durationInvalid'); return; }
    if (prepared.kind === 'invalid_alignment') { setGoalManagementErrorKey('pathCreate.alignmentInvalid'); return; }
    const comparison = compareGoalConfigurations(goalManagementCurrent, prepared.draft);
    if (comparison.current.intervalGoal && !comparison.current.intervalGoal.alignment) {
      setGoalManagementErrorKey('pathCreate.alignmentInvalid');
      return;
    }
    {
      const goalManagementCurrent: GeneratedPathGoalUpdateDraft['expectedGoals'] = {
        ...(comparison.current.intervalGoal ? {
          intervalGoal: {
            ...comparison.current.intervalGoal,
            alignment: comparison.current.intervalGoal.alignment!,
          },
        } : {}),
        ...(comparison.current.overallTarget ? {
          overallTarget: { ...comparison.current.overallTarget },
        } : {}),
      };
      setGoalManagementReview({
        current: goalManagementCurrent,
        proposed: prepared.draft,
        changed: comparison.changed,
        idempotencyKey: Crypto.randomUUID(),
      });
    }
    setGoalManagementErrorKey(null);
    setGoalManagementSaved(false);
  }

  function cancelPathGoalReview() {
    if (goalManagementBusy) return;
    setGoalManagementReview(null);
    setGoalManagementErrorKey(null);
  }

  async function confirmPathGoalChanges() {
    if (!session || destination?.kind !== 'home' || !goalManagementPathID || !goalManagementReview || !goalManagementReview.changed || goalManagementBusy || pathRenameBusy || ownershipTransferPathID === goalManagementPathID) return;
    const managedPath = destination.profile.paths.find((path) => path.id === goalManagementPathID);
    if (!managedPath || !effectivePathCapabilities(managedPath).manageGoals) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = goalManagementPathID;
    const review = goalManagementReview;
    const idempotencyKey = review.idempotencyKey;
    const ticket = goalManagementOperations.issue();
    goalManagementTarget.current = createPathAdministrationTarget(ownerID, pathID, currentSession);
    setGoalManagementBusy(true);
    setGoalManagementErrorKey(null);
    try {
      const result = await validateSessionCredential<PathGoalMutationResult>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).updatePathGoals(pathID, { expectedGoals: review.current, ...review.proposed, confirmed: true }, idempotencyKey),
      ));
      if (!ticket.current() ||
        !ownsPathAdministrationTarget(goalManagementTarget.current, ownerID, pathID, currentSession.token)) return;
      if (result.path.id !== pathID) {
        setGoalManagementErrorKey('errors.apiRejected');
        return;
      }
      setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
        ? {
            ...current,
            profile: {
              ...current.profile,
              paths: current.profile.paths.map((path) => path.id === pathID ? { ...result.path, home: path.home } : path),
              timers: {
                ...current.profile.timers,
                [pathID]: {
                  ...(current.profile.timers[pathID] ?? { running: false }),
                  accumulatedSeconds: result.accumulatedSeconds,
                  intervalProgress: result.intervalProgress,
                },
              },
            },
          }
        : current);
      setGoalManagementCurrent(result.path);
      setGoalManagementForm(pathGoalFormFromPath(result.path));
      setPathMembers((current) => current.map((member) => ({ ...member, intervalProgress: undefined, overallProgress: undefined })));
      void openPathMembers(pathID, undefined, false);
      setGoalManagementReview(null);
      setGoalManagementSaved(true);
      goalManagementTarget.current = null;
    } catch (cause) {
      if (!ticket.current() ||
        !ownsPathAdministrationTarget(goalManagementTarget.current, ownerID, pathID, currentSession.token)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile', ticket);
      }
      else setGoalManagementErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally {
      if (ticket.current()) setGoalManagementBusy(false);
    }
  }

  function changePathVisibility(visibility: PathVisibility) {
    if (pathVisibilityBusy || destination?.kind !== 'home' ||
      !pathVisibilityOptions(destination.profile.profileVisibility).includes(visibility)) return;
    setPathVisibilityDraft(visibility);
    setPathVisibilityReview(null);
    setPathVisibilityErrorKey(null);
    setPathVisibilitySaved(false);
    if (sharingPath) reviewPathVisibility(visibility);
  }

  function reviewPathVisibility(visibility = pathVisibilityDraft) {
    if (!session || !destination || destination.kind !== 'home' || !selectedPath || pathVisibilityBusy ||
      !sharingPath || !effectivePathCapabilities(selectedPath).manageVisibility) return;
    const conflict = pathVisibilityConflict.current;
    if (conflict && ownsPathAdministrationTarget(
      conflict,
      destination.profile.id,
      selectedPath.id,
      session.token,
    )) {
      void reloadPathVisibilityConflict(session, conflict.ownerID, conflict.pathID);
      return;
    }
    pathVisibilityConflict.current = null;
    const review = reviewPathVisibilityChange(
      selectedPath,
      visibility,
      destination.profile.profileVisibility,
    );
    if (review.kind === 'unchanged') return;
    if (review.kind === 'not-permitted') {
      setPathVisibilityErrorKey('pathVisibility.unavailable');
      return;
    }
    setPathVisibilityReview(review);
    setPathVisibilityErrorKey(null);
    setPathVisibilitySaved(false);
    const confirmationTarget = createPathAdministrationTarget(
      destination.profile.id,
      selectedPath.id,
      session,
    );
    pathVisibilityConfirmationTarget.current = confirmationTarget;
    presentNativeDestructiveConfirmation({
      cancelLabel: i18n.t('common.cancel'),
      confirmLabel: i18n.t('pathVisibility.confirmation.confirm'),
      message: [
        i18n.t('pathVisibility.confirmation.transition', {
          current: i18n.t(`pathVisibility.option.${review.current}`),
          pathName: review.pathName,
          proposed: i18n.t(`pathVisibility.option.${review.proposed}`),
        }),
        i18n.t(review.broader
          ? 'pathVisibility.confirmation.historyExposure'
          : 'pathVisibility.confirmation.narrowingAccess'),
        i18n.t('pathVisibility.confirmation.unchangedScope'),
      ].join('\n\n'),
      onCancel: () => {
        if (pathVisibilityConfirmationTarget.current === confirmationTarget) {
          pathVisibilityConfirmationTarget.current = null;
        }
        setPathVisibilityDraft(review.current);
        cancelPathVisibilityReview();
      },
      onConfirm: () => void confirmPathVisibility(review, confirmationTarget),
      title: i18n.t(review.broader
        ? 'pathVisibility.confirmation.heading'
        : 'pathVisibility.confirmation.narrowingHeading'),
    });
  }

  function cancelPathVisibilityReview() {
    if (pathVisibilityBusy) return;
    setPathVisibilityReview(null);
    setPathVisibilityErrorKey(null);
  }

  async function reloadPathVisibilityConflict(currentSession: Session, ownerID: string, pathID: string) {
    const conflict = pathVisibilityConflict.current;
    if (!ownsPathAdministrationTarget(conflict, ownerID, pathID, currentSession.token)) return;
    const recoverySession = currentAdministrationSession(conflict, pathID);
    if (!recoverySession) return;
    setPathVisibilityBusy(true);
    setPathVisibilityErrorKey(null);
    try {
      const profile = await loadMobileHomeProfile(apiURL, recoverySession);
      const latest = notificationLifecycleState.current;
      if (pathVisibilityConflict.current !== conflict || !currentAdministrationSession(conflict, pathID) ||
        latest.destination?.kind !== 'home' || latest.destination.profile.id !== ownerID) return;
      if (profile.id !== ownerID) throw sessionFailureFromResponse(502);
      const activeAuthoritative = profile.paths.find((candidate) => candidate.id === pathID);
      const archivedAuthoritative = profile.archivedPaths.find((candidate) => candidate.id === pathID);
      if (activeAuthoritative && !activeAuthoritative.home) {
        throw sessionFailureFromResponse(502);
      }
      const activeHomePath = activeAuthoritative as HomePath | undefined;
      const authoritative = activeAuthoritative ?? archivedAuthoritative;
      setDestination((current) => {
        if (current?.kind !== 'home' || current.profile.id !== ownerID) return current;
        function replaceTarget<P extends SessionPath>(paths: readonly P[], replacement?: P): P[] {
          const retained = paths.filter((candidate) => candidate.id !== pathID);
          if (!replacement) return retained;
          const index = paths.findIndex((candidate) => candidate.id === pathID);
          if (index < 0) return [...retained, replacement];
          return [...retained.slice(0, index), replacement, ...retained.slice(index)];
        }
        const timers = { ...current.profile.timers };
        if (profile.timers[pathID]) timers[pathID] = profile.timers[pathID];
        else delete timers[pathID];
        return {
          ...current,
          profile: {
            ...current.profile,
            paths: replaceTarget(current.profile.paths, activeHomePath),
            archivedPaths: replaceTarget(current.profile.archivedPaths, archivedAuthoritative),
            timers,
          },
        };
      });
      pathVisibilityConflict.current = null;
      pathVisibilityTarget.current = null;
      if (!authoritative || authoritative.archivedAt || !effectivePathCapabilities(authoritative).manageVisibility) {
        if (!authoritative && selectedPathID === pathID) {
          setSelectedPathID(null);
          resetPathDetail();
        }
        resetInvitationShare();
        return;
      }
      setPathVisibilityDraft(authoritative.visibility);
      setPathVisibilityReview(null);
      setPathVisibilityErrorKey('errors.conflict');
      setPathVisibilityBusy(false);
    } catch (cause) {
      if (pathVisibilityConflict.current !== conflict) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === recoverySession) {
        pathVisibilityConflict.current = null;
        pathVisibilityOperations.cancel(pathID);
        setPathVisibilityBusy(false);
        await handleSessionFailure(failure, recoverySession);
      } else {
        setPathVisibilityErrorKey(localizedFailure(failure, 'pathVisibility.unavailable'));
      }
    } finally {
      if (pathVisibilityConflict.current === conflict) setPathVisibilityBusy(false);
    }
  }

  async function confirmPathVisibility(
    reviewOverride?: Extract<PathVisibilityChangeReview, { kind: 'ready' }>,
    confirmationTarget?: PathAdministrationTarget<Session>,
  ) {
    const requestedReview = reviewOverride ?? pathVisibilityReview;
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || !requestedReview || pathVisibilityBusy || !sharingPath ||
      invitationReviewBusy || invitationSendBusy) return;
    const review = requestedReview;
    const admittedTarget = confirmationTarget ?? createPathAdministrationTarget(
      active.destination.profile.id,
      requestedReview.pathId,
      active.session,
    );
    if ((confirmationTarget && pathVisibilityConfirmationTarget.current !== admittedTarget) ||
      !ownsPathAdministrationTarget(
        admittedTarget,
        active.destination.profile.id,
        review.pathId,
        active.session.token,
      )) return;
    const path = active.destination.profile.paths.find((candidate) => candidate.id === review.pathId);
    const latestReview = path
      ? reviewPathVisibilityChange(path, review.proposed, active.destination.profile.profileVisibility)
      : null;
    if (!path || !effectivePathCapabilities(path).manageVisibility || path.visibility !== review.current ||
      latestReview?.kind !== 'ready' ||
      latestReview.current !== review.current || latestReview.proposed !== review.proposed) return;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const pathID = path.id;
    pathVisibilityConfirmationTarget.current = null;
    pathVisibilityTarget.current = admittedTarget;
    setPathVisibilityBusy(true);
    setPathVisibilityErrorKey(null);
    setPathVisibilitySaved(false);
    const result = await pathVisibilityOperations.submit(review, true, (requestedPathID, body, idempotencyKey) =>
      validateSessionCredential<SessionPath>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token)
          .setPathVisibility(requestedPathID, body, idempotencyKey),
      )),
    );
    if (!ownsPathAdministrationTarget(pathVisibilityTarget.current, ownerID, pathID, currentSession.token)) return;
    const latest = notificationLifecycleState.current;
    const latestPath = latest.destination?.kind === 'home' && latest.destination.profile.id === ownerID
      ? latest.destination.profile.paths.find((candidate) => candidate.id === pathID)
      : undefined;
    if (!latest.session || !ownsPathAdministrationTarget(pathVisibilityTarget.current, ownerID, pathID, latest.session.token) ||
      !latestPath || latestPath.visibility !== review.current ||
      !effectivePathCapabilities(latestPath).manageVisibility) {
      pathVisibilityOperations.cancel(pathID);
      pathVisibilityTarget.current = null;
      setPathVisibilityBusy(false);
      return;
    }
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (failure.kind === 'http' && failure.status === 409) {
        pathVisibilityOperations.cancel(pathID);
        pathVisibilityConflict.current = pathVisibilityTarget.current;
        setPathVisibilityReview(null);
        const recoverySession = currentAdministrationSession(pathVisibilityConflict.current, pathID);
        if (recoverySession) await reloadPathVisibilityConflict(recoverySession, ownerID, pathID);
        return;
      }
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        pathVisibilityOperations.cancel(pathID);
        await handleSessionFailure(failure, currentSession);
      } else {
        setPathVisibilityErrorKey(localizedFailure(failure, 'pathVisibility.unavailable'));
      }
      setPathVisibilityBusy(false);
      return;
    }
    if (result.kind !== 'applied') {
      setPathVisibilityBusy(false);
      return;
    }
    setDestination((current) => {
      if (current?.kind !== 'home' || current.profile.id !== ownerID) return current;
      const existing = current.profile.paths.find((candidate) => candidate.id === pathID);
      if (!existing) return current;
      const selected = current.profile.paths.find((candidate) => candidate.id === selectedPathID) ?? null;
      const applied = applyPathVisibilityResult<SessionPath>({
        activePaths: current.profile.paths,
        archivedPaths: current.profile.archivedPaths,
        selectedPath: selected,
      }, result.path);
      const activePaths: HomePath[] = applied.activePaths.map((authoritativePath) => ({
        ...authoritativePath,
        home: current.profile.paths.find(({ id }) => id === authoritativePath.id)!.home,
      }));
      return {
        ...current,
        profile: {
          ...current.profile,
          paths: activePaths,
          archivedPaths: applied.archivedPaths,
        },
      };
    });
    setPathVisibilityDraft(result.path.visibility);
    setPathVisibilityReview(null);
    setPathVisibilitySaved(true);
    setPathVisibilityBusy(false);
    pathVisibilityTarget.current = null;
  }

  function closePathManagement(dirty = false) {
    if (goalManagementBusy || pathRenameBusy || pathDeletionBusy || pathVisibilityBusy) return;
    if (dirty) {
      presentNativeDestructiveConfirmation({
        cancelLabel: i18n.t('common.cancel'),
        confirmLabel: i18n.t('pathManage.discard'),
        message: i18n.t('pathManage.discardDescription'),
        onConfirm: () => {
          resetGoalManagement();
          resetPathRename();
        },
        title: i18n.t('pathManage.discardTitle'),
      });
      return;
    }
    resetGoalManagement();
    resetPathRename();
  }

  function handoffPathManagement(dirty: boolean, kind: 'archive' | 'ownership-transfer') {
    const active = notificationLifecycleState.current;
    const pathID = goalManagementPathID;
    const path = active.destination?.kind === 'home' && pathID
      ? [...active.destination.profile.paths, ...active.destination.profile.archivedPaths]
        .find((candidate) => candidate.id === pathID)
      : undefined;
    const permitted = path && (kind === 'archive'
      ? effectivePathCapabilities(path).manageLifecycle
      : effectivePathCapabilities(path).transferOwnership ||
        (ownershipTransferPathID === path.id && Boolean(pendingOwnershipTransfer)));
    if (!active.session || active.destination?.kind !== 'home' || !path || !permitted ||
      goalManagementBusy || pathRenameBusy || pathDeletionBusy || pathVisibilityBusy) return;
    const target = createPathAdministrationTarget(active.destination.profile.id, path.id, active.session);
    pathManagementHandoffTarget.current = target;
    const handoff = () => {
      const latest = notificationLifecycleState.current;
      const currentSession = currentPathAdministrationCredential(
        target,
        latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
        target.pathID,
        latest.session,
      );
      const authoritativePath = latest.destination?.kind === 'home'
        ? [...latest.destination.profile.paths, ...latest.destination.profile.archivedPaths]
          .find((candidate) => candidate.id === target.pathID)
        : undefined;
      const stillPermitted = authoritativePath && (kind === 'archive'
        ? effectivePathCapabilities(authoritativePath).manageLifecycle
        : effectivePathCapabilities(authoritativePath).transferOwnership ||
          (ownershipTransferPathID === authoritativePath.id && Boolean(pendingOwnershipTransfer)));
      if (pathManagementHandoffTarget.current !== target || !currentSession || !authoritativePath || !stillPermitted) {
        if (pathManagementHandoffTarget.current === target) pathManagementHandoffTarget.current = null;
        return;
      }
      resetGoalManagement();
      resetPathRename();
      if (kind === 'archive') reviewPathArchive(authoritativePath, true);
      else if (pendingOwnershipTransfer && ownershipTransferPathID === authoritativePath.id) {
        setOwnershipTransferOpen(true);
      } else {
        void openOwnershipTransfer(authoritativePath, true, undefined, true);
      }
    };
    if (dirty) {
      presentNativeDestructiveConfirmation({
        cancelLabel: i18n.t('common.cancel'),
        confirmLabel: i18n.t('pathManage.discard'),
        message: i18n.t('pathManage.discardDescription'),
        onConfirm: handoff,
        onCancel: () => {
          if (pathManagementHandoffTarget.current === target) pathManagementHandoffTarget.current = null;
        },
        title: i18n.t('pathManage.discardTitle'),
      });
      return;
    }
    handoff();
  }

  function reviewPathDeletionIntent() {
    if (!goalManagementCurrent || pathDeletionBusy || !effectivePathCapabilities(goalManagementCurrent).manageLifecycle) return;
    setPathDeletionReview(reviewPathDeletion(goalManagementCurrent));
    setPathDeletionErrorKey(null);
  }

  function cancelPathDeletion() {
    if (pathDeletionBusy || !pathDeletionReview) return;
    pathDeletionOperations.cancel(pathDeletionReview.pathId);
    pathDeletionTarget.current = null;
    setPathDeletionReview(null);
    setPathDeletionErrorKey(null);
  }

  async function confirmPathDeletion() {
    if (!session || destination?.kind !== 'home' || !pathDeletionReview || pathDeletionBusy || goalManagementBusy || pathRenameBusy) return;
    const review = pathDeletionReview;
    const path = [...destination.profile.paths, ...destination.profile.archivedPaths]
      .find((candidate) => candidate.id === review.pathId);
    if (!path || path.name !== review.expectedName || !effectivePathCapabilities(path).manageLifecycle) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = review.pathId;
    pathDeletionTarget.current = createPathAdministrationTarget(ownerID, pathID, currentSession);
    setPathDeletionBusy(true);
    setPathDeletionErrorKey(null);
    const result = await pathDeletionOperations.submit(review, true, (requestedPathID, body, idempotencyKey) =>
      validateSessionCredential<{ pathId: string; deleted: true }>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).deletePath(requestedPathID, body, idempotencyKey),
      )),
    );
    if (!ownsPathAdministrationTarget(pathDeletionTarget.current, ownerID, pathID, currentSession.token)) return;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        pathDeletionOperations.cancel(pathID);
        await handleSessionFailure(failure, currentSession);
      } else {
        setPathDeletionErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
      }
      setPathDeletionBusy(false);
      return;
    }
    if (result.kind !== 'applied') {
      setPathDeletionBusy(false);
      return;
    }
    setDestination((current) => {
      if (current?.kind !== 'home' || current.profile.id !== ownerID) return current;
      const selected = [...current.profile.paths, ...current.profile.archivedPaths]
        .find((candidate) => candidate.id === selectedPathID) ?? null;
      const applied = applyPathDeletionResult({
        activePaths: current.profile.paths,
        archivedPaths: current.profile.archivedPaths,
        selectedPath: selected,
        timerStates: current.profile.timers,
      }, result);
      return {
        ...current,
        profile: {
          ...current.profile,
          paths: applied.activePaths,
          archivedPaths: applied.archivedPaths,
          timers: applied.timerStates,
        },
      };
    });
    setSelectedPathID(null);
    resetPathDetail();
    resetGoalManagement();
    resetPathRename();
  }

  function changePathRenameName(name: string) {
    if (pathRenameBusy || goalManagementBusy || pathDeletionBusy || goalManagementReview) return;
    setPathRenameName(name);
    setPathRenameErrorKey(null);
    setPathRenameSavedName(null);
  }

  async function submitPathRename() {
    if (!session || destination?.kind !== 'home' || !pathRenamePathID || pathRenameBusy || goalManagementBusy || goalManagementReview || ownershipTransferPathID === pathRenamePathID) return;
    const path = destination.profile.paths.find((candidate) => candidate.id === pathRenamePathID);
    if (!path ||
      !effectivePathCapabilities(path).renamePath ||
      goalManagementPathID !== path.id ||
      manualPathID === path.id ||
      pathArchiveReview?.pathId === path.id ||
      timerBusy[path.id]) return;
    let review: ReturnType<typeof reviewPathRename>;
    try {
      review = reviewPathRename(path, pathRenameName);
    } catch {
      setPathRenameErrorKey('errors.validationFailed');
      return;
    }
    if (!review.changed) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const pathID = path.id;
    pathRenameTarget.current = createPathAdministrationTarget(ownerID, pathID, currentSession);
    setPathRenameBusy(true);
    setPathRenameErrorKey(null);
    setPathRenameSavedName(null);
    const result = await pathRenameOperations.submit(review, (requestedPathID, body, idempotencyKey) =>
      validateSessionCredential<SessionPath>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token)
          .renamePath(requestedPathID, body, idempotencyKey),
      )),
    );
    if (!ownsPathAdministrationTarget(pathRenameTarget.current, ownerID, pathID, currentSession.token)) return;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        pathRenameOperations.cancel(pathID);
        await handleSessionFailure(failure, currentSession);
      } else {
        setPathRenameErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
      }
      setPathRenameBusy(false);
      return;
    }
    if (result.kind !== 'applied') {
      setPathRenameBusy(false);
      return;
    }
    setDestination((current) => {
      if (current?.kind !== 'home' || current.profile.id !== ownerID) return current;
      const selected = current.profile.paths.find((candidate) => candidate.id === selectedPathID) ?? null;
      const applied = applyPathRenameResult({
        paths: current.profile.paths,
        selectedPath: selected,
      }, result.path);
      return {
        ...current,
        profile: {
          ...current.profile,
          paths: applied.paths,
        },
      };
    });
    setGoalManagementCurrent(result.path);
    setPathRenameName(result.path.name);
    setPathRenameSavedName(result.path.name);
    setPathRenameBusy(false);
    pathRenameTarget.current = null;
  }

  function reviewPathArchive(path: SessionPath, fromManagement = false) {
    if (!effectivePathCapabilities(path).manageLifecycle || pathArchiveBusy ||
      (!fromManagement && (pathRenamePathID === path.id || goalManagementPathID === path.id)) ||
      manualPathID === path.id || timerBusy[path.id] || ownershipTransferPathID === path.id) return;
    setPathArchiveReview(reviewPathArchiveChange(path));
    setPathArchiveErrorKey(null);
    setPathArchiveSavedKey(null);
  }

  async function confirmPathArchiveChange() {
    if (!session || destination?.kind !== 'home' || !pathArchiveReview || pathArchiveBusy || ownershipTransferPathID === pathArchiveReview.pathId) return;
    const lifecyclePath = [...destination.profile.paths, ...destination.profile.archivedPaths]
      .find((path) => path.id === pathArchiveReview.pathId);
    if (!lifecyclePath || !effectivePathCapabilities(lifecyclePath).manageLifecycle) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const review = pathArchiveReview;
    const pathID = review.pathId;
    pathArchiveTarget.current = createPathAdministrationTarget(ownerID, pathID, currentSession);
    setPathArchiveBusy(true);
    setPathArchiveErrorKey(null);
    let unarchivedTimer: TimerState | undefined;
    const result = await pathArchiveOperations.submit(review, true, async (requestedPathID, body, idempotencyKey) => {
      const path = await validateSessionCredential<SessionPath>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).setPathArchiveState(requestedPathID, body, idempotencyKey),
      ));
      if (!body.archived && effectivePathCapabilities(path).trackTime) {
        unarchivedTimer = await validateSessionCredential<TimerState>(currentSession, async (credential) => generatedResponse(
          await createSessionApiClient(apiURL, () => credential.token).currentTimer(requestedPathID),
        ));
      }
      return path;
    });
    if (!ownsPathAdministrationTarget(pathArchiveTarget.current, ownerID, pathID, currentSession.token)) return;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        pathArchiveOperations.cancel(pathID);
        await handleSessionFailure(failure, currentSession);
      } else {
        setPathArchiveErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
      }
      setPathArchiveBusy(false);
      return;
    }
    if (result.kind !== 'applied') {
      setPathArchiveBusy(false);
      return;
    }
    setDestination((current) => {
      if (current?.kind !== 'home' || current.profile.id !== ownerID) return current;
      const selected = [...current.profile.paths, ...current.profile.archivedPaths]
        .find((path) => path.id === selectedPathID) ?? null;
      const applied = applyPathArchiveResult({
        activePaths: current.profile.paths,
        archivedPaths: current.profile.archivedPaths,
        selectedPath: selected,
        timerStates: unarchivedTimer
          ? {
              ...current.profile.timers,
              [pathID]: { ...unarchivedTimer, running: false, timer: undefined },
            }
          : current.profile.timers,
      }, result.path);
      return {
        ...current,
        profile: {
          ...current.profile,
          paths: applied.activePaths,
          archivedPaths: applied.archivedPaths,
          timers: applied.timerStates,
        },
      };
    });
    setPathArchiveReview(null);
    setPathArchiveSavedKey(review.archived ? 'pathArchive.archived' : 'pathArchive.unarchived');
    setPathArchiveBusy(false);
    pathArchiveTarget.current = null;
  }

  function cancelPathArchiveChange() {
    if (pathArchiveBusy || !pathArchiveReview) return;
    pathArchiveOperations.cancel(pathArchiveReview.pathId);
    pathArchiveTarget.current = null;
    setPathArchiveReview(null);
    setPathArchiveErrorKey(null);
  }

  async function createPath() {
    if (!session || destination?.kind !== 'home' || pathCreationBusy.current) return;
    const prepared = buildPathCreateDraft(pathName, pathGoalForm, pathVisibility);
    if (prepared.kind === 'invalid_duration') { setPathErrorKey('pathCreate.durationInvalid'); return; }
    if (prepared.kind === 'invalid_alignment') { setPathErrorKey('pathCreate.alignmentInvalid'); return; }
    pathCreationBusy.current = true;
    setPathSubmitting(true);
    setPathErrorKey(null);
    setPathCreated(false);
    const currentSession = session;
    const ownerID = destination.profile.id;
    pathCreationTarget.current = {
      ownerID,
      requestSessionToken: currentSession.token,
      sessionTokens: [currentSession.token],
    };
    const result = await pathCreation.submit(prepared.draft, (body, idempotencyKey) =>
      validateSessionCredential<SessionPath>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).createPath(body, idempotencyKey),
      )),
    );
    if (result.kind === 'superseded') return;
    const active = notificationLifecycleState.current;
    const activeOwnerID = active.destination?.kind === 'home' ? active.destination.profile.id : null;
    if (!ownsPathCreationTarget(pathCreationTarget.current, active.session?.token, activeOwnerID)) return;
    pathCreationBusy.current = false;
    setPathSubmitting(false);
    if (result.kind === 'invalid_name') { setPathErrorKey('pathCreate.nameRequired'); return; }
    if (result.kind === 'invalid_goal') { setPathErrorKey('errors.validationFailed'); return; }
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential &&
        pathCreationTarget.current?.sessionTokens.at(-1) === currentSession.token) {
        pathCreation.cancel();
        pathCreationTarget.current = null;
        await handleSessionFailure(failure, currentSession);
      } else setPathErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
      return;
    }
    const createdPath: HomePath = { ...result.path, home: { classification: 'solo', pinned: false } };
    setHomePreferences((current) => current.manualPathIDs.includes(createdPath.id)
      ? current
      : { ...current, manualPathIDs: [...current.manualPathIDs, createdPath.id] });
    setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
      ? {
          ...current,
          profile: {
            ...current.profile,
            paths: [...current.profile.paths, createdPath],
            timers: {
              ...current.profile.timers,
              [createdPath.id]: {
                running: false,
                accumulatedSeconds: 0,
                intervalProgress: createdPath.intervalGoal
                  ? { accumulatedSeconds: 0, targetSeconds: createdPath.intervalGoal.targetSeconds }
                  : undefined,
              },
            },
          },
        }
      : current);
    setPathName('');
    setPathVisibility('private');
    setPathGoalForm(initialPathGoalForm());
    setCreatingPath(false);
    setPathCreated(true);
    pathCreationTarget.current = null;
  }

  async function toggleTimer(pathID: string) {
    if (!session || destination?.kind !== 'home' || timerBusy[pathID] || pathRenamePathID === pathID || goalManagementPathID === pathID || pathArchiveReview?.pathId === pathID || ownershipTransferPathID === pathID) return;
    const mutationLease = timerMutationBarrier.enter();
    if (!mutationLease) return;
    try {
    const path = destination.profile.paths.find((candidate) => candidate.id === pathID);
    if (!path || !effectivePathCapabilities(path).trackTime) return;
    const state = destination.profile.timers[pathID];
    if (!state) return;
    const ownerID = destination.profile.id;
    const timerID = state.timer?.id;
    setTimerBusy((current) => ({ ...current, [pathID]: true }));
    setTimerErrorKeys((current) => ({ ...current, [pathID]: undefined }));
    setTimerNoticeKey(null);
    const currentSession = session;
    const result = state.running && timerID
      ? await timerOperations.stop(pathID, timerID, (idempotencyKey) =>
          validateSessionCredential<TimerStopResult>(currentSession, async (credential) => generatedResponse(
            await createSessionApiClient(apiURL, () => credential.token).stopTimer(pathID, timerID, idempotencyKey),
          )),
        )
      : await timerOperations.start(pathID, (idempotencyKey) =>
          validateSessionCredential<TimerState>(currentSession, async (credential) => generatedResponse(
            await createSessionApiClient(apiURL, () => credential.token).startTimer(pathID, idempotencyKey),
          )),
        );
    if (result.kind === 'superseded') return;
    setTimerBusy((current) => ({ ...current, [pathID]: false }));
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        timerOperations.cancel();
        await handleSessionFailure(failure, currentSession);
      } else {
        setTimerErrorKeys((current) => ({ ...current, [pathID]: localizedFailure(failure, 'errors.temporarilyUnavailable') }));
      }
      return;
    }
    const presentation = timerMutationPresentation(result.state);
    applyOwnedTimerState(ownerID, currentSession, pathID, presentation.state);
    if (state.running && result.state.running === false && 'saved' in result.state && result.state.saved === true) {
      await refreshHomeOrganizationPath(pathID, currentSession, ownerID);
    }
    if (presentation.notice === 'subsecond') setTimerNoticeKey('timer.subsecondNotice');
    } finally {
      mutationLease.release();
    }
  }

  function bindPathDetailTarget(ownerID: string, sessionToken: string, pathID: string, activityID?: string) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || active.destination.profile.id !== ownerID) return false;
    const current = pathDetailTarget.current;
    if (active.session.token !== sessionToken && !(current && current.ownerID === ownerID &&
      current.pathID === pathID && current.sessionTokens.includes(sessionToken) &&
      current.sessionTokens.includes(active.session.token))) return false;
    pathDetailTarget.current = current && current.ownerID === ownerID && current.pathID === pathID &&
      current.sessionTokens.includes(sessionToken) && current.sessionTokens.includes(active.session.token)
      ? { ...current, activityID }
      : createPathDetailTarget(ownerID, sessionToken, pathID, activityID);
    return true;
  }

  function ownsActivePathDetail(pathID: string, activityID?: string) {
    const current = notificationLifecycleState.current;
    return ownsPathDetailTarget(
      pathDetailTarget.current,
      current.destination?.kind === 'home' ? current.destination.profile.id : null,
      current.session?.token,
      pathID,
      activityID,
    );
  }

  function bindPathMemberTarget(ownerID: string, sessionToken: string, pathID: string, userID?: string) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || active.destination.profile.id !== ownerID) return false;
    const current = pathMemberTarget.current;
    if (active.session.token !== sessionToken && !(current && current.ownerID === ownerID &&
      current.pathID === pathID && current.sessionTokens.includes(sessionToken) &&
      current.sessionTokens.includes(active.session.token))) return false;
    pathMemberTarget.current = current && current.ownerID === ownerID && current.pathID === pathID &&
      current.sessionTokens.includes(sessionToken) && current.sessionTokens.includes(active.session.token)
      ? { ...current, userID }
      : createPathMemberTarget(ownerID, sessionToken, pathID, userID);
    return true;
  }

  function ownsActivePathMember(pathID: string, userID?: string) {
    const current = notificationLifecycleState.current;
    return ownsPathMemberTarget(
      pathMemberTarget.current,
      current.destination?.kind === 'home' ? current.destination.profile.id : null,
      current.session?.token,
      pathID,
      userID,
    );
  }

  function openPathDetail(pathID: string, focusedOwnershipTransferID?: string, navigate = true, loadMembers = true) {
    const current = notificationLifecycleState.current;
    const currentDestination = current.destination;
    if (!current.session || currentDestination?.kind !== 'home') return;
    resetGoalManagement();
    resetPathRename();
    resetPathArchive();
    resetManualActivity();
    resetOwnershipTransfer();
    pathDetailOperations.invalidate();
    pathDetailOwnerID.current = currentDestination.profile.id;
    bindPathDetailTarget(currentDestination.profile.id, current.session.token, pathID);
    setSelectedPathID(pathID);
    setActivityHistoryOpen(false);
    setActivityHistory([]);
    setActivityHistoryCursor(null);
    setActivityHistoryErrorKey(null);
    setSelectedActivity(null);
    setActivityRevisions([]);
    setActivityRevisionCursor(null);
    setActivityRevisionErrorKey(null);
    setPathDetailErrorKey(null);
    const path = currentDestination.profile.paths.find(({ id }) => id === pathID)
      ?? currentDestination.profile.archivedPaths.find(({ id }) => id === pathID);
    if (path) void openOwnershipTransfer(path, Boolean(focusedOwnershipTransferID), focusedOwnershipTransferID);
    if (loadMembers) void openPathMembers(pathID, undefined, false);
    if (navigate) router.push({ pathname: '/path/[pathID]', params: { pathID } });
  }

  async function recoverPathRoute(intent: PathRouteIntent) {
    const current = notificationLifecycleState.current;
    if (!current.session || current.destination?.kind !== 'home') return;
    const currentSession = current.session;
    const ownerID = current.destination.profile.id;
    const path = [...current.destination.profile.paths, ...current.destination.profile.archivedPaths]
      .find((candidate) => candidate.id === intent.pathID);
    if (!path) {
      await retryPathRoute(intent);
      return;
    }

    openPathDetail(intent.pathID, undefined, false, intent.kind !== 'members' && intent.kind !== 'member');
    pathRouteTarget.current = createPathRouteTarget(ownerID, currentSession.token, intent.routeKey);
    setPathRouteRecovery('loading');
    let routeReady = true;
    if (intent.kind === 'history') {
      await openActivityHistory(intent.pathID, undefined, false);
    } else if (intent.kind === 'activity') {
      setActivityHistoryOpen(true);
      await inspectActivity(intent.pathID, intent.activityID, false);
    } else if (intent.kind === 'members') {
      const page = await openPathMembers(intent.pathID, undefined, false, true);
      if (!page || typeof page === 'string') {
        routeReady = false;
        setPathRouteRecovery(page || 'offline');
      }
    } else if (intent.kind === 'member') {
      const recovered = await recoverPathMember(intent.pathID, intent.userID);
      if (recovered !== 'found') {
        routeReady = false;
        setPathRouteRecovery(recovered);
      }
    } else if (intent.kind === 'nudge-settings') {
      const recovered = await loadPathNudgePreference(intent.pathID, false);
      if (recovered !== 'found') {
        routeReady = false;
        setPathRouteRecovery(recovered ?? 'offline');
      }
    }
    const latest = notificationLifecycleState.current;
    if (ownsPathRouteTarget(pathRouteTarget.current, latest.destination?.kind === 'home'
      ? latest.destination.profile.id
      : null, latest.session?.token, intent.routeKey)) {
      if (routeReady && (intent.kind !== 'member' || selectedPathMemberRef.current?.userId === intent.userID)) {
        setPathRouteRecovery(null);
      }
    }
  }

  async function retryPathRoute(intent: PathRouteIntent) {
    const current = notificationLifecycleState.current;
    if (!current.session || current.destination?.kind !== 'home') return;
    const currentSession = current.session;
    const ownerID = current.destination.profile.id;
    const target = createPathRouteTarget(ownerID, currentSession.token, intent.routeKey);
    pathRouteTarget.current = target;
    setPathRouteRecovery('loading');
    try {
      const response = generatedResponse(
        await createSessionApiClient(apiURL, () => currentSession.token).path(intent.pathID),
      );
      const latest = notificationLifecycleState.current;
      if (!ownsPathRouteTarget(pathRouteTarget.current, latest.destination?.kind === 'home'
        ? latest.destination.profile.id
        : null, latest.session?.token, intent.routeKey)) return;
      if (response.status === 404) {
        setPathRouteRecovery('unavailable');
        return;
      }
      const projected = await validateSessionCredential<SessionPath>(currentSession, async () => response);
      const admitted = projected.archivedAt
        ? admitMobileSessionPaths([], [projected])
        : admitMobileSessionPaths([projected], []);
      const refreshed = admitted.active[0] ?? admitted.archived[0];
      if (!refreshed) {
        setPathRouteRecovery('unavailable');
        return;
      }
      setDestination((active) => active?.kind === 'home' && active.profile.id === ownerID
        ? { ...active, profile: applyRefreshedMobilePath(active.profile, intent.pathID, refreshed) }
        : active);
      pathRouteTarget.current = null;
      setPathRouteRecovery('loading');
    } catch (cause) {
      const latest = notificationLifecycleState.current;
      if (!ownsPathRouteTarget(pathRouteTarget.current, latest.destination?.kind === 'home'
        ? latest.destination.profile.id
        : null, latest.session?.token, intent.routeKey)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        if (latest.session !== currentSession && latest.session && latest.destination?.kind === 'home' &&
          latest.destination.profile.id === ownerID) {
          pathRouteTarget.current = createPathRouteTarget(
            latest.destination.profile.id,
            latest.session.token,
            intent.routeKey,
          );
          setPathRouteRecovery('offline');
        } else if (latest.session === currentSession) {
          await handleSessionFailure(failure, currentSession);
        }
        return;
      }
      setPathRouteRecovery(failure.kind === 'network' ? 'offline' : 'unavailable');
    }
  }

  function leavePathRouteToHome() {
    resetPathDetail();
    router.replace('/(tabs)/home');
  }

  async function openPathMembers(pathID: string, cursor?: string, navigate = true, present = navigate): Promise<PathMemberPage | 'offline' | 'unavailable' | undefined> {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home') return;
    const path = [...active.destination.profile.paths, ...active.destination.profile.archivedPaths]
      .find((candidate) => candidate.id === pathID);
    if (!path) return;
    if (navigate && !cursor && !pathMembersOpen) {
      router.push({ pathname: '/path/[pathID]/members', params: { pathID } });
    }
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const ticket = pathMemberListOperations.issue();
    if (!bindPathMemberTarget(ownerID, currentSession.token, pathID)) return;
    if (present) setPathMembersOpen(true);
    if (cursor) setPathMembersLoadingMore(true);
    else setPathMembersLoading(true);
    setPathMembersError(false);
    try {
      const page = await validateSessionCredential<PathMemberPage>(currentSession, async (credential) => {
        const result = await createSessionApiClient(apiURL, () => credential.token).pathMembers(pathID, cursor);
        return generatedResponse({
          response: result.response,
          error: result.error,
          data: result.data ? { data: pathMemberPageFromAPI(pathID, ownerID, result.data) } : undefined,
        });
      });
      if (!ticket.current() || !ownsActivePathMember(pathID)) return;
      setPathMembers((current) => cursor
        ? [...current, ...page.items.filter((candidate) => !current.some(({ userId }) => userId === candidate.userId))]
        : page.items);
      setPathMembersCursor(page.nextCursor);
      return page;
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathMember(pathID)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const decision = classifySessionFailure(failure);
      if (decision.discardCredential) {
        if (notificationLifecycleState.current.session === currentSession) {
          await handleSessionFailure(failure, currentSession, 'profile', ticket);
          return 'unavailable';
        }
        setPathMembersError(true);
        return 'offline';
      }
      setPathMembersError(true);
      return decision.retryable ? 'offline' : 'unavailable';
    } finally {
      if (ticket.current()) {
        setPathMembersLoading(false);
        setPathMembersLoadingMore(false);
      }
    }
  }

  async function recoverPathMember(pathID: string, userID: string) {
    let cursor: string | undefined;
    const seenCursors = new Set<string>();
    for (;;) {
      const page = await openPathMembers(pathID, cursor, false, true);
      if (!page) return 'offline' as const;
      if (typeof page === 'string') return page;
      const member = page.items.find((candidate) => candidate.userId === userID);
      if (member) {
        await inspectPathMember(member, false);
        return selectedPathMemberRef.current?.pathId === pathID && selectedPathMemberRef.current.userId === userID
          ? 'found' as const
          : 'offline' as const;
      }
      if (!page.nextCursor || seenCursors.has(page.nextCursor)) return 'unavailable' as const;
      seenCursors.add(page.nextCursor);
      cursor = page.nextCursor;
    }
  }

  async function loadPathMemberNudgeEligibility(member: PathMemberSummary) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || member.isViewer || member.role === 'supporter') return;
    const path = active.destination.profile.paths.find(({ id }) => id === member.pathId);
    if (!path) return;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const ticket = nudgeEligibilityOperations.issue();
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .getPathMemberNudgeEligibility(member.pathId, member.userId));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_nudge_eligibility_response');
      const eligibility = nudgeEligibilityFromAPI(envelope.data);
      const current = notificationLifecycleState.current;
      if (!ticket.current() || !ownsPathMemberTarget(pathMemberTarget.current,
        current.destination?.kind === 'home' ? current.destination.profile.id : null,
        current.session?.token, member.pathId, member.userId)) return;
      setSelectedPathMember((selected) => selected?.pathId === member.pathId && selected.userId === member.userId
        ? { ...selected, nudgeEligibility: eligibility }
        : selected);
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession);
      }
      // Eligibility failures remain hidden and disclose no relationship or audience detail.
    }
  }

  function openNudgeComposer(member: PathMemberSummary) {
    if (nudgeActionState(member).kind !== 'send' || nudgeSendBusy) return;
    nudgeSendOperations.cancel();
    setNudgeComposerPreset(null);
    setNudgeSendErrorKey(null);
    setNudgeSendConfirmationKey(null);
    setNudgeComposerOpen(true);
  }

  function closeNudgeComposer() {
    if (nudgeSendBusy || nudgeSendAdmission.current) return;
    nudgeSendOperations.cancel();
    setNudgeComposerOpen(false);
    setNudgeComposerPreset(null);
    setNudgeSendErrorKey(null);
  }

  function selectNudgeComposerPreset(preset: NudgePreset) {
    if (nudgeSendBusy || nudgeSendAdmission.current || preset === nudgeComposerPreset) return;
    setNudgeComposerPreset(preset);
    setNudgeSendErrorKey(null);
  }

  async function sendSelectedNudge() {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || !selectedPathMember || !nudgeComposerOpen
      || !nudgeComposerPreset || nudgeSendBusy || nudgeSendAdmission.current ||
      nudgeActionState(selectedPathMember).kind !== 'send') return;
    const member = selectedPathMember;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const preset = nudgeComposerPreset;
    const review = reviewNudgeSend(member.pathId, member.userId, preset);
    const admission = Symbol('nudge-send');
    nudgeSendAdmission.current = admission;
    setNudgeSendBusy(true);
    setNudgeSendErrorKey(null);
    const result = await nudgeSendOperations.submit(review, (pathId, recipientUserId, body, idempotencyKey) =>
      validateSessionCredential(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token)
          .sendPathMemberNudge(pathId, recipientUserId, body, idempotencyKey),
      )),
    );
    if (nudgeSendAdmission.current !== admission) return;
    const current = notificationLifecycleState.current;
    if (!ownsPathMemberTarget(pathMemberTarget.current,
      current.destination?.kind === 'home' ? current.destination.profile.id : null,
      current.session?.token, member.pathId, member.userId)) {
      if (nudgeSendAdmission.current === admission) {
        nudgeSendAdmission.current = null;
        setNudgeSendBusy(false);
      }
      return;
    }
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && current.session === currentSession) {
        await handleSessionFailure(failure, currentSession);
      } else setNudgeSendErrorKey('nudge.compose.sendError');
      if (nudgeSendAdmission.current === admission) {
        nudgeSendAdmission.current = null;
        setNudgeSendBusy(false);
      }
      return;
    }
    if (result.kind !== 'applied') {
      if (nudgeSendAdmission.current === admission) {
        nudgeSendAdmission.current = null;
        setNudgeSendBusy(false);
      }
      return;
    }
    setSelectedPathMember((selected) => selected?.pathId === member.pathId && selected.userId === member.userId
      ? {
          ...selected,
          nudgeEligibility: {
            eligible: false,
            pathId: member.pathId,
            reason: 'rate_limited',
            recipientUserId: member.userId,
          },
        }
      : selected);
    setNudgeComposerOpen(false);
    setNudgeComposerPreset(null);
    nudgeSendAdmission.current = null;
    setNudgeSendBusy(false);
    setNudgeSendConfirmationKey('nudge.compose.sent');
    await AccessibilityInfo.announceForAccessibility(i18n.t('nudge.compose.sent'));
    void loadPathMemberNudgeEligibility(member);
  }

  async function loadPathNudgePreference(pathId: string, navigate = true) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || pathNudgePreferenceBusy) return;
    const path = [...active.destination.profile.paths, ...active.destination.profile.archivedPaths]
      .find(({ id }) => id === pathId);
    if (!path || !effectivePathCapabilities(path).trackTime) return 'unavailable' as const;
    if (navigate && !pathNudgeSettingsOpen) {
      router.push({ pathname: '/path/[pathID]/nudge-settings', params: { pathID: pathId } });
    }
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const routeKey = pathNudgeSettingsRouteKey(pathId);
    if (!ownsPathNudgePreferenceTarget(
      pathNudgePreferenceTarget.current,
      ownerID,
      currentSession.token,
      pathId,
    )) {
      pathNudgePreferenceTarget.current = createPathNudgePreferenceTarget(ownerID, currentSession.token, pathId);
    }
    if (!ownsPathRouteTarget(pathRouteTarget.current, ownerID, currentSession.token, routeKey)) {
      pathRouteTarget.current = createPathRouteTarget(ownerID, currentSession.token, routeKey);
    }
    const ticket = nudgeAudienceLoadOperations.issue();
    setPathNudgeSettingsOpen(true);
    setPathNudgePreferenceLoading(true);
    setPathNudgePreferenceError(false);
    setPathRouteRecovery('loading');
    try {
      const response = generatedResponse(await createSessionApiClient(apiURL, () => currentSession.token)
        .getPathNudgePreference(pathId));
      if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
      const envelope = await response.json();
      if (!envelope) throw new Error('invalid_nudge_preference_response');
      const preference = nudgeAudiencePreferenceFromAPI(envelope.data);
      const latest = notificationLifecycleState.current;
      if (!ticket.current() || !ownsPathNudgePreferenceTarget(
        pathNudgePreferenceTarget.current,
        latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
        latest.session?.token,
        pathId,
      )) return;
      setPathNudgePreference(preference);
      setPathRouteRecovery(null);
      return 'found' as const;
    } catch (cause) {
      const latest = notificationLifecycleState.current;
      if (!ticket.current() || !ownsPathNudgePreferenceTarget(
        pathNudgePreferenceTarget.current,
        latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
        latest.session?.token,
        pathId,
      )) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      const decision = classifySessionFailure(failure);
      const disposition = pathNudgeFailureDisposition({
        discardCredential: decision.discardCredential,
        latestOwnerID: latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
        latestSessionToken: latest.session?.token,
        pathID: pathId,
        requestSessionToken: currentSession.token,
        retryable: decision.retryable,
        target: pathNudgePreferenceTarget.current,
      });
      if (disposition === 'ignore') return;
      if (disposition === 'discard-current') {
        await handleSessionFailure(failure, currentSession);
        return 'unavailable' as const;
      }
      setPathNudgePreferenceError(true);
      const recovery = disposition;
      setPathRouteRecovery(recovery);
      return recovery;
    } finally {
      const latest = notificationLifecycleState.current;
      if (ticket.current() && ownsPathNudgePreferenceTarget(
        pathNudgePreferenceTarget.current,
        latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
        latest.session?.token,
        pathId,
      )) setPathNudgePreferenceLoading(false);
    }
  }

  async function updatePathNudgeAudience(audience: NudgeAudience) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || !pathNudgePreference || pathNudgePreferenceBusy
      || pathNudgePreferenceAdmission.current
      || audience === pathNudgePreference.audience || !ownsPathNudgePreferenceTarget(
        pathNudgePreferenceTarget.current,
        active.destination.profile.id,
        active.session.token,
        pathNudgePreference.pathId,
      )) return;
    const currentSession = active.session;
    const review = reviewNudgeAudienceChange(pathNudgePreference, audience);
    const admission = Symbol('path-nudge-preference');
    pathNudgePreferenceAdmission.current = admission;
    setPathNudgePreferenceBusy(true);
    setPathNudgePreferenceError(false);
    const result = await nudgeAudienceOperations.submit(review.pathId, review, (pathId, body, idempotencyKey) =>
      validateSessionCredential(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token)
          .updatePathNudgePreference(pathId, body, idempotencyKey),
      )),
    );
    if (pathNudgePreferenceAdmission.current !== admission) return;
    const latest = notificationLifecycleState.current;
    if (!ownsPathNudgePreferenceTarget(
      pathNudgePreferenceTarget.current,
      latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
      latest.session?.token,
      review.pathId,
    )) {
      if (pathNudgePreferenceAdmission.current === admission) {
        pathNudgePreferenceAdmission.current = null;
        setPathNudgePreferenceBusy(false);
      }
      return;
    }
    if (result.kind === 'applied') {
      setPathNudgePreference(result.preference);
      pathNudgePreferenceAdmission.current = null;
      setPathNudgePreferenceBusy(false);
      await AccessibilityInfo.announceForAccessibility(i18n.t('nudge.audience.saved'));
      return;
    }
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      const decision = classifySessionFailure(failure);
      const disposition = pathNudgeFailureDisposition({
        discardCredential: decision.discardCredential,
        latestOwnerID: latest.destination?.kind === 'home' ? latest.destination.profile.id : null,
        latestSessionToken: latest.session?.token,
        pathID: review.pathId,
        requestSessionToken: currentSession.token,
        retryable: decision.retryable,
        target: pathNudgePreferenceTarget.current,
      });
      if (disposition === 'ignore') {
        pathNudgePreferenceAdmission.current = null;
        setPathNudgePreferenceBusy(false);
        return;
      }
      if (disposition === 'discard-current') {
        await handleSessionFailure(failure, currentSession);
      }
      else setPathNudgePreferenceError(true);
    }
    if (pathNudgePreferenceAdmission.current === admission) {
      pathNudgePreferenceAdmission.current = null;
      setPathNudgePreferenceBusy(false);
    }
  }

  async function loadPathMemberActivities(member: PathMemberSummary, cursor?: string) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home') return;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const ticket = pathMemberActivityOperations.issue();
    setPathMemberActivitiesLoading(true);
    setPathMemberActivitiesErrorKey(null);
    try {
      const page = await loadActivityPage(currentSession, member.pathId, cursor, member.userId);
      if (!ticket.current() || !ownsActivePathMember(member.pathId, member.userId)) return;
      setPathMemberActivities((current) => cursor ? appendUniqueActivities(current, page.items) : newestActivitiesFirst(page.items));
      setPathMemberActivitiesCursor(page.nextCursor);
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathMember(member.pathId, member.userId)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile', ticket);
      }
      else setPathMemberActivitiesErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally {
      if (ticket.current()) setPathMemberActivitiesLoading(false);
    }
  }

  async function inspectPathMember(member: PathMemberSummary, navigate = true) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home') return;
    const path = [...active.destination.profile.paths, ...active.destination.profile.archivedPaths]
      .find((candidate) => candidate.id === member.pathId);
    if (!path) return;
    if (navigate) router.push({
      pathname: '/path/[pathID]/members/[userID]',
      params: { pathID: member.pathId, userID: member.userId },
    });
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const ticket = pathMemberReviewOperations.issue();
    if (!bindPathMemberTarget(ownerID, currentSession.token, member.pathId, member.userId)) return;
    setSelectedPathMember(member);
    setPathMemberRemovalReview(null);
    setPathMemberReviewLoading(true);
    setPathMemberRemovalErrorKey(null);
    setPathMemberPendingRole(null);
    setPathMemberRoleChangeErrorKey(null);
    setPathMemberActivities([]);
    setPathMemberActivitiesCursor(null);
    setPathMemberActivitiesErrorKey(null);
    void loadPathMemberActivities(member);
    void loadPathMemberNudgeEligibility(member);
    if (!member.canRemove) {
      setPathMemberReviewLoading(false);
      return;
    }
    try {
      const response = await validateSessionCredential<GeneratedMemberRemovalReview>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).reviewPathMemberRemoval(member.pathId, member.userId),
      ));
      const review = pathMemberRemovalReviewFromAPI(member.pathId, response);
      if (!ticket.current() || !ownsActivePathMember(member.pathId, member.userId)) return;
      setPathMemberRemovalReview(review);
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathMember(member.pathId, member.userId)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession, 'profile', ticket);
      }
    } finally { if (ticket.current()) setPathMemberReviewLoading(false); }
  }

  async function removeSelectedPathMember() {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || !pathMemberRemovalReview || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy) return;
    const review = pathMemberRemovalReview;
    const path = [...active.destination.profile.paths, ...active.destination.profile.archivedPaths]
      .find((candidate) => candidate.id === review.pathId);
    if (!path || effectivePathCapabilities(path).manageMembers !== true) return;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    setPathMemberRemovalBusy(true);
    setPathMemberRemovalErrorKey(null);
    const result = await pathMemberRemovalOperations.submit(review, true, (pathID, userID, body, idempotencyKey) =>
      validateSessionCredential<MemberRemovalReceipt>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).removePathMember(pathID, userID, body, idempotencyKey),
      )),
    );
    const currentTarget = pathMemberTarget.current;
    if (!ownsActivePathMember(review.pathId, review.userId)) {
      if (result.kind === 'applied' && currentTarget?.ownerID === ownerID && currentTarget.pathID === review.pathId) {
        void openPathMembers(review.pathId);
      }
      return;
    }
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession);
      } else setPathMemberRemovalErrorKey(localizedFailure(failure, 'pathMembers.removalUnavailable'));
      setPathMemberRemovalBusy(false);
      return;
    }
    if (result.kind !== 'applied') {
      setPathMemberRemovalBusy(false);
      return;
    }
    const applied = applyPathMemberRemovalResult({ members: pathMembers, selected: review }, result);
    setPathMembers(applied.members);
    setPathMemberRemovalBusy(false);
    allowNativeChildRouteDismissal(pathMemberRemovalRouteKey(review.pathId, review.userId));
    router.back();
  }

  function choosePathMemberRole(role: PathMemberAccessRole) {
    if (!selectedPathMember || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy) return;
    if (role === selectedPathMember.role) return;
    const ordinaryChange = selectedPathMember.canChangeRole
      && pathMemberRemovalReview !== null
      && ((selectedPathMember.role === 'participant' && role === 'supporter')
        || (selectedPathMember.role === 'supporter' && role === 'participant'));
    const administratorChange = (selectedPathMember.canGrantAdministrator
        && selectedPathMember.role === 'participant' && role === 'administrator')
      || (((selectedPathMember.canRevokeAdministrator && !selectedPathMember.isViewer)
          || (selectedPathMember.canStepDownAdministrator && selectedPathMember.isViewer))
        && selectedPathMember.role === 'administrator' && role === 'participant');
    if (!ordinaryChange && !administratorChange) return;
    setPathMemberRoleChangeErrorKey(null);
    if (role === 'supporter' || administratorChange) setPathMemberPendingRole(role);
    else void changeSelectedPathMemberRole(role);
  }

  function cancelPathMemberRoleChange() {
    if (pathMemberRoleChangeBusy || !selectedPathMember) return;
    pathMemberRoleChangeOperations.cancel(selectedPathMember.pathId, selectedPathMember.userId);
    setPathMemberPendingRole(null);
    setPathMemberRoleChangeErrorKey(null);
  }

  async function changeSelectedPathMemberRole(role = pathMemberPendingRole) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || !selectedPathMember || !role
      || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy) return;
    const member = selectedPathMember;
    const ordinaryChange = member.canChangeRole
      && pathMemberRemovalReview !== null
      && ((member.role === 'participant' && role === 'supporter')
        || (member.role === 'supporter' && role === 'participant'));
    const administratorChange = (member.canGrantAdministrator && member.role === 'participant' && role === 'administrator')
      || (((member.canRevokeAdministrator && !member.isViewer)
          || (member.canStepDownAdministrator && member.isViewer))
        && member.role === 'administrator' && role === 'participant');
    if (!ordinaryChange && !administratorChange) return;
    const currentSession = active.session;
    const ownerID = active.destination.profile.id;
    const review = reviewPathMemberRoleChange(pathMemberRemovalReview ?? {
      displayName: member.displayName,
      pathId: member.pathId,
      role: member.role,
      runningTimer: false,
      sessionCount: member.sessionCount,
      totalTrackedSeconds: member.totalTrackedSeconds,
      userId: member.userId,
      username: member.username,
    }, role);
    setPathMemberRoleChangeBusy(true);
    setPathMemberRoleChangeErrorKey(null);
    const result = await pathMemberRoleChangeOperations.submit(review, true, (pathID, userID, body, idempotencyKey) =>
      validateSessionCredential<PathMemberRoleChangeReceipt>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).changePathMemberRole(pathID, userID, body, idempotencyKey),
      )),
    );
    if (!ownsActivePathMember(member.pathId, member.userId)) return;
    if (result.kind === 'failed') {
      setPathMemberRoleChangeBusy(false);
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession);
      } else setPathMemberRoleChangeErrorKey(localizedFailure(failure, 'pathMembers.roleChangeUnavailable'));
      return;
    }
    if (result.kind !== 'applied') {
      setPathMemberRoleChangeBusy(false);
      return;
    }
    const applied = applyPathMemberRoleChangeResult({ members: pathMembers }, result);
    const safelyInvalidatedMembers = applied.members.map((candidate) => candidate.pathId === member.pathId && candidate.userId === member.userId
      ? {
        ...candidate,
        canChangeRole: false,
        canGrantAdministrator: false,
        canRemove: false,
        canRevokeAdministrator: false,
        canStepDownAdministrator: false,
      }
      : candidate);
    setPathMembers(safelyInvalidatedMembers);
    const changed = safelyInvalidatedMembers.find((candidate) => candidate.pathId === member.pathId && candidate.userId === member.userId) ?? null;
    setSelectedPathMember(changed);
    setPathMemberRemovalReview((current) => current && result.receipt.role !== 'administrator' ? {
      ...current,
      role: result.receipt.role,
      runningTimer: result.receipt.activityDeleted ? false : current.runningTimer,
      sessionCount: result.receipt.activityDeleted ? 0 : current.sessionCount,
      totalTrackedSeconds: result.receipt.activityDeleted ? 0 : current.totalTrackedSeconds,
    } : null);
    if (result.receipt.activityDeleted) {
      setPathMemberActivities([]);
      setPathMemberActivitiesCursor(null);
    }
    const refreshedPage = await openPathMembers(member.pathId, undefined, false);
    const current = notificationLifecycleState.current;
    if (current.destination?.kind !== 'home' || current.destination.profile.id !== ownerID) return;
    const refreshedMember = refreshedPage && typeof refreshedPage !== 'string'
      ? refreshedPage.items.find((candidate) => candidate.userId === member.userId)
      : undefined;
    if (refreshedMember) await inspectPathMember(refreshedMember, false);
    else pathMemberTarget.current = createPathMemberTarget(ownerID, currentSession.token, member.pathId, member.userId);
    const currentAfterInspection = notificationLifecycleState.current;
    if (currentAfterInspection.destination?.kind !== 'home' || currentAfterInspection.destination.profile.id !== ownerID ||
      !ownsActivePathMember(member.pathId, member.userId)) return;
    setPathMemberPendingRole(null);
    setPathMemberRoleChangeBusy(false);
    await AccessibilityInfo.announceForAccessibility(i18n.t(result.receipt.role === 'supporter'
      ? 'pathMembers.roleChangedSupporter'
      : result.receipt.role === 'administrator'
        ? 'pathMembers.roleChangedAdministrator'
        : 'pathMembers.roleChangedParticipant'));
  }

  async function unblockPathMember(member: PathMemberSummary) {
    const active = notificationLifecycleState.current;
    if (!active.session || active.destination?.kind !== 'home' || !member.blockedByViewer || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy) return;
    const currentSession = active.session;
    const ticket = pathMemberUnblockOperations.issue();
    setPathMemberUnblockBusy(true);
    setPathMemberUnblockErrorKey(null);
    try {
      const result = await userBlockingPort.unblockUser(member.userId, Crypto.randomUUID());
      const current = notificationLifecycleState.current;
      if (!ticket.current() || !ownsPathMemberTarget(pathMemberTarget.current,
        current.destination?.kind === 'home' ? current.destination.profile.id : null,
        current.session?.token, member.pathId, member.userId) || result.target.userId !== member.userId) return;
      setPathMembers((current) => current.map((candidate) => candidate.userId === member.userId
        ? { ...candidate, blockedByViewer: false }
        : candidate));
      setSelectedPathMember((current) => current?.userId === member.userId
        ? { ...current, blockedByViewer: false }
        : current);
      await AccessibilityInfo.announceForAccessibility(i18n.t('blocking.unblockedSuccess', { username: member.username }));
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential && notificationLifecycleState.current.session === currentSession) {
        await handleSessionFailure(failure, currentSession);
      }
      else setPathMemberUnblockErrorKey('blocking.unblockUnavailable');
    } finally {
      if (ticket.current()) setPathMemberUnblockBusy(false);
    }
  }

  async function openActivityHistory(pathID: string, cursor?: string, navigate = true) {
    if (!session || destination?.kind !== 'home') return;
    if (navigate && !cursor && !activityHistoryOpen) {
      router.push({ pathname: '/path/[pathID]/history', params: { pathID } });
    }
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = pathDetailOperations.issue();
    pathDetailOwnerID.current = ownerID;
    const currentTarget = pathDetailTarget.current;
    const activityID = currentTarget?.pathID === pathID ? currentTarget.activityID : undefined;
    if (!bindPathDetailTarget(ownerID, currentSession.token, pathID, activityID)) return;
    setActivityHistoryOpen(true);
    setPathDetailBusy(true);
    setActivityHistoryErrorKey(null);
    try {
      const page = await loadActivityPage(currentSession, pathID, cursor);
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      setActivityHistory((current) => cursor ? appendUniqueActivities(current, page.items) : newestActivitiesFirst(page.items));
      setActivityHistoryCursor(page.nextCursor);
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        if (notificationLifecycleState.current.session === currentSession) {
          await handleSessionFailure(failure, currentSession, 'profile', ticket);
        } else setActivityHistoryErrorKey('errors.temporarilyUnavailable');
      } else setActivityHistoryErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setPathDetailBusy(false); }
  }

  async function inspectActivity(pathID: string, activityID: string, navigate = true) {
    if (!session || destination?.kind !== 'home') return;
    activityDeletionOperations.invalidate();
    activityDeletionTarget.current = null;
    setActivityDeletionBusy(false);
    setActivityDeletionErrorKey(null);
    setActivityDeletionRetryable(false);
    if (navigate) {
      router.push({
        pathname: '/path/[pathID]/history/[activityID]',
        params: { activityID, pathID },
      });
    }
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = pathDetailOperations.issue();
    pathDetailOwnerID.current = ownerID;
    if (!bindPathDetailTarget(ownerID, currentSession.token, pathID, activityID)) return;
    setSelectedActivity(null);
    setActivityRevisions([]);
    setActivityRevisionCursor(null);
    setActivityRevisionErrorKey(null);
    setPathDetailBusy(true);
    setPathDetailErrorKey(null);
    let detailLoaded = false;
    try {
      const detail = await validateSessionCredential<ActivityDetail>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).activity(pathID, activityID),
      ));
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      setSelectedActivity(detail);
      detailLoaded = true;
      const revisions = await loadRevisionPage(currentSession, pathID, activityID);
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      setActivityRevisions(appendUniqueRevisions([], revisions.items));
      setActivityRevisionCursor(revisions.nextCursor);
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        if (notificationLifecycleState.current.session === currentSession) {
          await handleSessionFailure(failure, currentSession, 'profile', ticket);
        } else setPathDetailErrorKey(localizedFailure({ kind: 'network' }, 'errors.temporarilyUnavailable'));
      }
      else if (detailLoaded && pathDetailTarget.current?.activityID === activityID) {
        setActivityRevisionErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
      } else setPathDetailErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setPathDetailBusy(false); }
  }

  function openPathMemberActivity(pathID: string, activityID: string) {
    setActivityHistoryOpen(true);
    void inspectActivity(pathID, activityID);
  }

  async function loadMoreActivityRevisions(pathID: string, activityID: string, cursor: string) {
    if (!session || destination?.kind !== 'home' || selectedActivity?.activity.id !== activityID) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    const ticket = pathDetailOperations.issue();
    pathDetailOwnerID.current = ownerID;
    if (!bindPathDetailTarget(ownerID, currentSession.token, pathID, activityID)) return;
    setPathDetailBusy(true);
    setActivityRevisionErrorKey(null);
    try {
      const page = await loadRevisionPage(currentSession, pathID, activityID, cursor);
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      setActivityRevisions((current) => appendUniqueRevisions(current, page.items));
      setActivityRevisionCursor(page.nextCursor);
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        if (notificationLifecycleState.current.session === currentSession) {
          await handleSessionFailure(failure, currentSession, 'profile', ticket);
        } else setActivityRevisionErrorKey(localizedFailure({ kind: 'network' }, 'errors.temporarilyUnavailable'));
      }
      else setActivityRevisionErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setPathDetailBusy(false); }
  }

  async function refreshPathDetail(pathID: string, activityID: string, currentSession: Session, ownerID: string | null) {
    const ticket = pathDetailOperations.issue();
    if (!ownerID || !bindPathDetailTarget(ownerID, currentSession.token, pathID, activityID)) return;
    setPathDetailBusy(true);
    setPathDetailErrorKey(null);
    try {
      const [history, detail, revisions] = await Promise.all([
        loadActivityPage(currentSession, pathID),
        validateSessionCredential<ActivityDetail>(currentSession, async (credential) => generatedResponse(
          await createSessionApiClient(apiURL, () => credential.token).activity(pathID, activityID),
        )),
        loadRevisionPage(currentSession, pathID, activityID),
      ]);
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      setActivityHistory(newestActivitiesFirst(history.items));
      setActivityHistoryCursor(history.nextCursor);
      setActivityHistoryErrorKey(null);
      setActivityHistoryOpen(true);
      setSelectedActivity(detail);
      setActivityRevisions(appendUniqueRevisions([], revisions.items));
      setActivityRevisionCursor(revisions.nextCursor);
      setActivityRevisionErrorKey(null);
    } catch (cause) {
      if (!ticket.current() || !ownsActivePathDetail(pathID, activityID)) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        if (notificationLifecycleState.current.session === currentSession) {
          await handleSessionFailure(failure, currentSession, 'profile', ticket);
        } else setPathDetailErrorKey(localizedFailure({ kind: 'network' }, 'errors.temporarilyUnavailable'));
      }
      else setPathDetailErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setPathDetailBusy(false); }
  }

  async function editSelectedPathActivity() {
    if (!session || destination?.kind !== 'home' || !selectedActivity || !selectedPathID || !selectedPath || !effectivePathCapabilities(selectedPath).trackTime) return;
    if (!activityBelongsToProfile(selectedActivity, destination.profile.id)) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    claimManualActivityPresentation('activity-details');
    const ticket = manualOperations.issue();
    manualOwnerID.current = ownerID;
    setManualBusy(true);
    setManualErrorKey(null);
    try {
      const defaults = await validateSessionCredential<ManualActivityDefaults>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).manualActivityDefaults(selectedPathID),
      ));
      if (!ticket.current() || manualOwnerID.current !== ownerID) return;
      const seed = activityEditSeed(selectedActivity, defaults);
      setManualPathID(selectedPathID);
      setManualDefaults(seed.defaults);
      setManualDefaultsLoadedAt(Date.now());
      setManualForm(seed.form);
      setManualNote(seed.note);
      manualDraftBaseline.current = manualActivityDraft(seed.form, seed.note);
      setManualActivity({ id: selectedActivity.activity.id, version: selectedActivity.version });
      setManualSavedVersion(null);
      setManualIdempotencyKey(Crypto.randomUUID());
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) await handleSessionFailure(failure, currentSession, 'profile', ticket);
      else setManualErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setManualBusy(false); }
  }

  function mobileManualNow(): ManualActivityParticipantNow {
    if (!manualDefaults) throw new Error('manual defaults unavailable');
    return manualActivityParticipantNow(manualDefaults.currentInstant, manualDefaults.timeZone, Math.max(0, Date.now() - manualDefaultsLoadedAt));
  }

  async function openManualActivity(pathID: string) {
    if (!session || destination?.kind !== 'home' || pathRenamePathID === pathID || goalManagementPathID === pathID || ownershipTransferPathID === pathID) return;
    const path = destination.profile.paths.find((candidate) => candidate.id === pathID);
    if (!path || !effectivePathCapabilities(path).trackTime || pathArchiveReview?.pathId === pathID) return;
    const currentSession = session;
    const ownerID = destination.profile.id;
    claimManualActivityPresentation('path-details');
    const ticket = manualOperations.issue();
    manualOwnerID.current = ownerID;
    setManualBusy(true); setManualErrorKey(null);
    try {
      const defaults = await validateSessionCredential<ManualActivityDefaults>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).manualActivityDefaults(pathID),
      ));
      if (!ticket.current() || manualOwnerID.current !== ownerID) return;
      setManualPathID(pathID); setManualDefaults(defaults); setManualDefaultsLoadedAt(Date.now());
      const form = createManualActivityFormState(manualActivityParticipantNow(defaults.currentInstant, defaults.timeZone));
      setManualForm(form);
      setManualNote(''); setManualActivity(null); setManualIdempotencyKey(Crypto.randomUUID());
      manualDraftBaseline.current = manualActivityDraft(form, '');
      setManualSavedVersion(null);
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) await handleSessionFailure(failure, currentSession, 'profile', ticket);
      else setManualErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setManualBusy(false); }
  }

  function changeManualOccurrence(patch: Partial<ManualActivityLocalDateTime>) {
    setManualForm((current) => current ? overrideManualActivityOccurrence(current, patch) : current);
    setManualSavedVersion(null);
    setManualIdempotencyKey(Crypto.randomUUID()); setManualErrorKey(null);
  }

  function changeManualDuration(durationSeconds: string) {
    setManualForm((current) => current ? updateManualActivityDuration(current, durationSeconds, mobileManualNow()) : current);
    setManualSavedVersion(null);
    setManualIdempotencyKey(Crypto.randomUUID()); setManualErrorKey(null);
  }

  function changeManualNote(note: string) {
    setManualNote(note);
    setManualSavedVersion(null);
    setManualIdempotencyKey(Crypto.randomUUID());
    setManualErrorKey(null);
  }

  async function submitManualActivity() {
    if (!session || !manualPathID || !manualForm || manualBusy || goalManagementPathID === manualPathID || ownershipTransferPathID === manualPathID) return;
    const path = destination?.kind === 'home'
      ? destination.profile.paths.find((candidate) => candidate.id === manualPathID)
      : undefined;
    if (!path || !effectivePathCapabilities(path).trackTime) return;
    const serialized = serializeManualActivityForm(manualForm, mobileManualNow());
    if (!serialized.ok) { setManualErrorKey(serialized.reason === 'future_end' ? 'activity.futureEnd' : 'activity.invalid'); return; }
    const currentSession = session;
    const pathID = manualPathID;
    const activity = manualActivity;
    const idempotencyKey = manualIdempotencyKey;
    const ownerID = manualOwnerID.current;
    if (!ownerID) return;
    const ticket = manualOperations.issue();
    setManualBusy(true); setManualErrorKey(null);
    try {
      const body = { localDate: serialized.fields.localDate, localStartTime: serialized.fields.localTime, durationSeconds: serialized.fields.durationSeconds, note: manualNote || undefined };
      const result = await validateSessionCredential<ActivityMutationResult>(currentSession, async (credential) => generatedResponse(
        activity
          ? await createSessionApiClient(apiURL, () => credential.token).updateActivity(pathID, activity.id, body, idempotencyKey)
          : await createSessionApiClient(apiURL, () => credential.token).createManualActivity(pathID, body, idempotencyKey),
      ));
      if (!ticket.current() || manualOwnerID.current !== ownerID) return;
      setManualActivity({ id: result.activity.id, version: result.version }); setManualIdempotencyKey(Crypto.randomUUID());
      setManualSavedVersion(result.version);
      manualDraftBaseline.current = manualActivityDraft(manualForm, manualNote);
      setDestination((current) => current?.kind === 'home' && current.profile.id === ownerID
        ? { ...current, profile: { ...current.profile, timers: { ...current.profile.timers, [pathID]: { ...(current.profile.timers[pathID] ?? { running: false }), accumulatedSeconds: result.accumulatedSeconds, intervalProgress: result.intervalProgress } } } }
        : current);
      await refreshHomeOrganizationPath(pathID, currentSession, ownerID);
      if (selectedPathID === pathID) await refreshPathDetail(pathID, result.activity.id, currentSession, ownerID);
    } catch (cause) {
      if (!ticket.current()) return;
      const failure: SessionFailure = isSessionFailure(cause) ? cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) await handleSessionFailure(failure, currentSession, 'profile', ticket);
      else setManualErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
    } finally { if (ticket.current()) setManualBusy(false); }
  }

  function closeManualActivity() {
    if (manualBusy) return;
    if (!manualDraftDirty) {
      resetManualActivity();
      return;
    }
    presentNativeDestructiveConfirmation({
      cancelLabel: i18n.t('activity.keepEditing'),
      confirmLabel: i18n.t('activity.discard'),
      message: i18n.t('activity.discardDescription'),
      onConfirm: resetManualActivity,
      title: i18n.t('activity.discardTitle'),
    });
  }

  function cancelPathLeaveReview() {
    if (pathLeaveBusy) return;
    pathLeaveOperations.cancel(pathLeaveReview?.pathId);
    pathLeaveTarget.current = null;
    setPathLeaveReview(null);
    setPathLeaveErrorKey(null);
  }

  function reviewPathLeaveIntent(path: SessionPath) {
    if (!session || destination?.kind !== 'home' || pathLeaveBusy || effectivePathCapabilities(path).leavePath !== true) return;
    const target = { ownerID: destination.profile.id, pathID: path.id, pathName: path.name, session };
    pathLeaveTarget.current = target;
    setPathLeaveErrorKey(null);
    setPathLeaveNotice(null);
    const retryReview = pathLeaveReview?.pathId === path.id ? pathLeaveReview : null;
    if (retryReview) {
      presentNativeDestructiveConfirmation({
        cancelLabel: i18n.t('common.cancel'),
        confirmLabel: i18n.t(retryReview.retainActivity ? 'pathLeave.confirm' : 'pathLeave.confirmDelete'),
        message: i18n.t(retryReview.retainActivity
          ? effectivePathCapabilities(path).trackTime ? 'pathLeave.warning' : 'pathLeave.supporterWarning'
          : 'pathLeave.deleteWarning'),
        onCancel: cancelPathLeaveReview,
        onConfirm: () => void confirmPathLeave(retryReview, target),
        title: i18n.t(retryReview.retainActivity ? 'pathLeave.heading' : 'pathLeave.deleteHeading', { pathName: path.name }),
      });
      return;
    }
    const retainReview = reviewPathLeave(path, true);
    setPathLeaveReview(retainReview);
    if (!effectivePathCapabilities(path).trackTime) {
      presentNativeDestructiveConfirmation({
        cancelLabel: i18n.t('common.cancel'), confirmLabel: i18n.t('pathLeave.confirm'),
        message: i18n.t('pathLeave.supporterWarning'), onCancel: cancelPathLeaveReview,
        onConfirm: () => void confirmPathLeave(retainReview, target),
        title: i18n.t('pathLeave.heading', { pathName: path.name }),
      });
      return;
    }
    presentNativePathLeaveChoice({
      cancelLabel: i18n.t('common.cancel'),
      deleteLabel: i18n.t('pathLeave.deleteActivity'),
      keepLabel: i18n.t('pathLeave.keepActivity'),
      message: `${i18n.t('pathLeave.warning')}\n\n${i18n.t('pathLeave.choicePrompt')}\n\n${i18n.t('pathLeave.keepActivityDescription')}\n\n${i18n.t('pathLeave.deleteActivityDescription')}`,
      onCancel: cancelPathLeaveReview,
      onDelete: () => {
        const deleteReview = reviewPathLeave(path, false);
        setPathLeaveReview(deleteReview);
        presentNativeDestructiveConfirmation({
          cancelLabel: i18n.t('common.cancel'), confirmLabel: i18n.t('pathLeave.confirmDelete'),
          message: i18n.t('pathLeave.deleteWarning'), onCancel: cancelPathLeaveReview,
          onConfirm: () => void confirmPathLeave(deleteReview, target), title: i18n.t('pathLeave.deleteHeading'),
        });
      },
      onKeep: () => void confirmPathLeave(retainReview, target),
      title: i18n.t('pathLeave.heading', { pathName: path.name }),
    });
  }

  async function confirmPathLeave(review: PathLeaveReview, target: NonNullable<typeof pathLeaveTarget.current>) {
    const current = notificationLifecycleState.current;
    if (pathLeaveTarget.current !== target || !current.session || current.destination?.kind !== 'home' || current.session !== target.session || current.destination.profile.id !== target.ownerID || pathLeaveBusy) return;
    const path = [...current.destination.profile.paths, ...current.destination.profile.archivedPaths]
      .find((candidate) => candidate.id === review.pathId);
    if (!path || path.name !== target.pathName || path.name !== review.pathName || effectivePathCapabilities(path).leavePath !== true || (!review.retainActivity && !effectivePathCapabilities(path).trackTime)) return;
    const currentSession = target.session;
    const ownerID = target.ownerID;
    const pathID = review.pathId;
    setPathLeaveBusy(true);
    setPathLeaveErrorKey(null);
    const result = await pathLeaveOperations.submit(review, true, (requestedPathID, body, idempotencyKey) =>
      validateSessionCredential<PathLeaveReceipt>(currentSession, async (credential) => generatedResponse(
        await createSessionApiClient(apiURL, () => credential.token).leavePath(
          requestedPathID, body, idempotencyKey,
        ),
      )),
    );
    if (pathLeaveTarget.current?.ownerID !== ownerID ||
      pathLeaveTarget.current?.pathID !== pathID ||
      pathLeaveTarget.current?.session !== currentSession) return;
    if (result.kind === 'failed') {
      const failure: SessionFailure = isSessionFailure(result.cause) ? result.cause : { kind: 'network' };
      if (classifySessionFailure(failure).discardCredential) {
        pathLeaveOperations.cancel(pathID);
        await handleSessionFailure(failure, currentSession);
      } else {
        setPathLeaveErrorKey(localizedFailure(failure, 'errors.temporarilyUnavailable'));
      }
      setPathLeaveBusy(false);
      return;
    }
    if (result.kind !== 'applied') {
      setPathLeaveBusy(false);
      return;
    }
    setDestination((current) => {
      if (current?.kind !== 'home' || current.profile.id !== ownerID) return current;
      const selected = [...current.profile.paths, ...current.profile.archivedPaths]
        .find((candidate) => candidate.id === selectedPathID) ?? null;
      const applied = applyPathLeaveResult({
        activePaths: current.profile.paths,
        archivedPaths: current.profile.archivedPaths,
        selectedPath: selected,
        timerStates: current.profile.timers,
      }, result);
      return {
        ...current,
        profile: {
          ...current.profile,
          paths: applied.activePaths,
          archivedPaths: applied.archivedPaths,
          timers: applied.timerStates,
        },
      };
    });
    setPathLeaveNotice(i18n.t(review.retainActivity ? 'pathLeave.completedRetained' : 'pathLeave.completedDeleted', { pathName: review.pathName }));
    setSelectedPathID(null);
    resetPathDetail();
  }

  const ownedHomeDestination = destination?.kind === 'home' && session &&
    homeProjectionSessionToken.current === session.token ? destination : null;
  const socialPresentationKey = [
    ownedHomeDestination?.profile.id ?? '',
    socialPresentationGeneration.current,
  ].join(':');
  const selectedPath = ownedHomeDestination
    ? ownedHomeDestination.profile.paths.find((path) => path.id === selectedPathID)
      ?? ownedHomeDestination.profile.archivedPaths.find((path) => path.id === selectedPathID)
      ?? null
    : null;
  const pathProjectionKey = ownedHomeDestination
    ? [...ownedHomeDestination.profile.paths, ...ownedHomeDestination.profile.archivedPaths]
      .map((path) => path.id).join('\u0000')
    : '';
  useEffect(() => {
    if (!currentPathRouteIntent || !ready || !ownedHomeDestination || !session) return;
    if (ownsPathRouteTarget(
      pathRouteTarget.current,
      ownedHomeDestination.profile.id,
      session.token,
      currentPathRouteIntent.routeKey,
    )) return;
    void recoverPathRoute(currentPathRouteIntent);
  }, [
    currentPathRouteIntent?.routeKey,
    ownedHomeDestination?.profile.id,
    pathProjectionKey,
    ready,
    session?.token,
  ]);
  useEffect(() => {
    if (!ready || !ownedHomeDestination || (pathname !== '/home' && pathname !== '/')) return;
    const pending = takeSocialRouteBootstrap();
    if (pending) router.push(socialRouteHref(pending) as never);
  }, [ownedHomeDestination?.profile.id, pathname, ready]);
  useEffect(() => {
    if (!ready || !ownedHomeDestination || (pathname !== '/home' && pathname !== '/')) return;
    const pending = takeNotificationJourneyBootstrap(socialPresentationKey);
    if (!pending) return;
    if (pending.kind === 'notifications') {
      void openNotifications(true);
      return;
    }
    if (pending.kind === 'invitations') {
      void openPendingInvitations(undefined, true);
      return;
    }
    router.push(notificationJourneyHref(pending) as never);
  }, [ownedHomeDestination?.profile.id, pathname, ready, socialPresentationKey]);
  useEffect(() => {
    if (!ready || !ownedHomeDestination || (pathname !== '/home' && pathname !== '/')) return;
    const pending = takeSettingsJourneyBootstrap(socialPresentationKey);
    if (pending) router.push(settingsJourneyHref(pending) as never);
  }, [ownedHomeDestination?.profile.id, pathname, ready, socialPresentationKey]);
  useEffect(() => {
    if (!ready || !ownedHomeDestination || !session || !currentNotificationJourneyIntent) return;
    if (currentNotificationJourneyIntent.kind === 'notifications' && !notificationsOpen &&
      !notificationsBusy && !notificationTarget.current && !notificationMutationAdmission.current) {
      void openNotifications(false);
      return;
    }
    if (currentNotificationJourneyIntent.kind === 'invitations' && !invitationsOpen &&
      !pendingInvitationsBusy && !pendingInvitationsTarget.current) {
      void openPendingInvitations(undefined, false);
    }
  }, [
    currentNotificationJourneyIntent?.routeKey,
    invitationsOpen,
    notificationsOpen,
    notificationMutationGeneration,
    ownedHomeDestination?.profile.id,
    pendingInvitationsBusy,
    ready,
    session?.token,
  ]);
  useEffect(() => {
    if (!ready || !ownedHomeDestination || !currentSocialRouteIntent) return;
    if (currentSocialRouteIntent.kind !== 'activity' && currentSocialRouteIntent.kind !== 'comments' &&
      currentSocialRouteIntent.kind !== 'comment-hearts') return;
    const event = currentSocialRouteIntent.kind === 'activity'
      ? socialFeedPage.current.items.find((candidate) => candidate.type === 'practice_session' &&
        candidate.path.id === currentSocialRouteIntent.pathID && candidate.activity.id === currentSocialRouteIntent.activityID)
      : socialDeepEvent.eventID === currentSocialRouteIntent.eventID && socialDeepEvent.status === 'ready'
        ? socialDeepEvent.event
        : undefined;
    if (!event) {
      if (currentSocialRouteIntent.kind === 'comments' || currentSocialRouteIntent.kind === 'comment-hearts') {
        if (socialDeepEvent.eventID !== currentSocialRouteIntent.eventID || socialDeepEvent.status === 'idle') {
          void loadSocialDeepEvent(currentSocialRouteIntent.eventID);
        }
      } else if (socialFeed.status === 'idle') void loadSocialFeed();
      else if (socialFeed.status === 'ready' && socialFeedPage.current.nextCursor && !socialFeed.loadingMore) {
        void loadSocialFeed(socialFeedPage.current.nextCursor);
      }
      return;
    }
    if (currentSocialRouteIntent.kind === 'activity') {
      if (event.type === 'practice_session' && !socialFeedActivity) void openSocialFeedActivity(event, false);
      return;
    }
    if (!practiceComments || practiceComments.eventID !== event.id) {
      openPracticeComments(event.id, event.participant.userId, undefined, false);
      return;
    }
    if (currentSocialRouteIntent.kind === 'comment-hearts' && !practiceCommentHeartRoster) {
      const comment = practiceCommentPage.current.items.find(({ id }) => id === currentSocialRouteIntent.commentID);
      if (comment) openPracticeCommentHeartRoster(comment, false);
      else if (practiceComments.status === 'ready' && practiceCommentPage.current.nextCursor && !practiceComments.loadingMore) {
        void loadPracticeComments(practiceCommentTarget.current!, practiceCommentPage.current.nextCursor);
      }
    }
  }, [
    currentSocialRouteIntent?.routeKey,
    ownedHomeDestination?.profile.id,
    practiceCommentHeartRoster?.commentID,
    practiceComments?.eventID,
    practiceComments?.loadingMore,
    practiceComments?.status,
    ready,
    socialFeed.loadingMore,
    socialFeed.status,
    socialFeedActivity?.event.id,
    socialDeepEvent.eventID,
    socialDeepEvent.status,
    socialFeedPage.current.items.length,
    socialFeedPage.current.nextCursor,
  ]);
  const selectedCapabilities = selectedPath
    ? effectivePathCapabilities(selectedPath)
    : { trackTime: false, inviteMembers: false, leavePath: false, manageGoals: false, manageLifecycle: false, manageVisibility: false, renamePath: false, transferOwnership: false };
  const goalManagementCanReview = goalManagementForm && goalManagementCurrent
    ? (() => {
        const prepared = buildPathGoalUpdateDraft(goalManagementForm);
        return prepared.kind !== 'valid'
          || compareGoalConfigurations(goalManagementCurrent, prepared.draft).changed;
      })()
    : false;
  const selectedTimerState = ownedHomeDestination && selectedPath
    && selectedCapabilities.trackTime ? ownedHomeDestination.profile.timers[selectedPath.id]
    : undefined;
  const selectedOverallProgress = selectedPath?.overallTarget && selectedTimerState
    ? overallProgress(selectedTimerState.accumulatedSeconds, selectedPath.overallTarget)
    : undefined;
  const selectedIntervalProgress = intervalProgressPresentation(selectedTimerState?.intervalProgress);
  const activeActivityID = pathDetailTarget.current?.pathID === selectedPath?.id
    ? pathDetailTarget.current?.activityID
    : undefined;
  const manualActivityPresentation = manualPathID && manualForm && manualDefaults ? <ManualActivityForm
    key={manualActivity ? [manualActivity.id, manualActivity.version].join(':') : manualPathID}
    activityVersion={manualSavedVersion ?? undefined}
    busy={manualBusy}
    editing={Boolean(manualActivity)}
    errorText={manualErrorKey ? i18n.t(manualErrorKey) : undefined}
    form={manualForm}
    note={manualNote}
    onCancel={closeManualActivity}
    onChangeDuration={changeManualDuration}
    onChangeNote={changeManualNote}
    onChangeOccurrence={changeManualOccurrence}
    onSave={() => void submitManualActivity()}
    timeZone={manualDefaults.timeZone}
  /> : null;
  const homeSections = ownedHomeDestination
    ? organizeHomePaths(ownedHomeDestination.profile.paths, ownedHomeDestination.profile.timers, homePreferences, homeFilter)
    : { active: [] as HomePath[], pinned: [] as HomePath[], trackable: [] as HomePath[], supporting: [] as HomePath[] };
  const renderHomePath = (path: HomePath) => {
    const pathCapabilities = effectivePathCapabilities(path);
    const state = ownedHomeDestination && pathCapabilities.trackTime
      ? ownedHomeDestination.profile.timers[path.id]
      : undefined;
    const progress = state && path.overallTarget
      ? overallProgress(state.accumulatedSeconds, path.overallTarget)
      : undefined;
    const currentIntervalProgress = homeIntervalProgress(state?.intervalProgress);
    const elapsedText = state?.running
      ? formatCompactDuration(activeTimerSeconds(state.timer?.startedAt, now), i18n)
      : undefined;
    const pinned = homePreferences.pinnedPathIDs.includes(path.id);
    return <PathCard
      actions={[{
        disabled: homePreferenceBusy,
        label: i18n.t(pinned ? 'home.arrange.unpin' : 'home.arrange.pin', { pathName: path.name }),
        onPress: () => void updateHomePreferences(pinned
          ? unpinHomePath(homePreferences, path.id)
          : pinHomePath(homePreferences, path.id)),
        systemImage: pinned ? 'pin.slash' : 'pin',
      }]}
      actionsAccessibilityLabel={i18n.t('home.pathActions', { pathName: path.name })}
      accumulatedText={state ? i18n.t('path.progress.accumulatedCompact', {
        duration: formatCompactDuration(state.accumulatedSeconds, i18n),
      }) : undefined}
      key={path.id}
      name={path.name}
      onOpen={() => void openPathDetail(path.id)}
      progress={currentIntervalProgress || progress ? <>
        {currentIntervalProgress ? <IntervalProgressIndicator compact progress={currentIntervalProgress} pathName={path.name} /> : null}
        {progress ? <OverallProgressIndicator compact progress={progress} pathName={path.name} /> : null}
      </> : undefined}
      timer={pathCapabilities.trackTime && state ? <TimerControl
        actionLabel={i18n.t(timerMutationPresentation(state).controlMessage)}
        busy={Boolean(timerBusy[path.id])}
        elapsedAccessibilityLabel={elapsedText ? i18n.t('timer.elapsedValue', { duration: elapsedText }) : undefined}
        elapsedText={elapsedText}
        errorText={timerErrorKeys[path.id] ? i18n.t(timerErrorKeys[path.id]!) : undefined}
        onPress={() => void toggleTimer(path.id)}
        running={state.running}
      /> : undefined}
    />;
  };
  const activeHomeVisibleCount = homeSections.active.length + homeSections.pinned.length +
    homeSections.trackable.length + homeSections.supporting.length;
  const homeViewSections: readonly HomeViewSection[] = (() => {
    if (!ownedHomeDestination) return [];
    if (archivedPathsOpen) return ownedHomeDestination.profile.archivedPaths.length > 0 ? [{
      key: 'archived',
      title: i18n.t('home.archivedPaths'),
      items: ownedHomeDestination.profile.archivedPaths.map((path) => <PathCard
        key={path.id}
        name={path.name}
        onOpen={() => void openPathDetail(path.id)}
        progress={<Text style={styles.textMuted}>{i18n.t('pathArchive.readOnly')}</Text>}
      />),
    }] : [];
    const sections: HomeViewSection[] = [];
    if (homeSections.active.length > 0) sections.push({
      key: 'active',
      title: i18n.t('home.activeTimersHeading'),
      items: homeSections.active.map(renderHomePath),
    });
    if (homeSections.pinned.length > 0) sections.push({
      key: 'pinned',
      title: i18n.t('home.arrange.pinnedHeading'),
      items: homeSections.pinned.map(renderHomePath),
    });
    if (homeSections.trackable.length > 0) sections.push({
      key: 'paths',
      title: i18n.t('home.section.paths'),
      items: homeSections.trackable.map(renderHomePath),
    });
    if (homeSections.supporting.length > 0) sections.push({
      key: 'supporting',
      title: i18n.t('home.section.supporting'),
      items: homeSections.supporting.map(renderHomePath),
    });
    return sections;
  })();
  const currentHomePresentation = homePresentation({
    failure: homeRecovery?.status === 'offline' || homeRecovery?.status === 'error'
      ? homeRecovery.status
      : null,
    filter: homeFilter,
    homeAvailable: ownedHomeDestination !== null,
    loading: !ready || homeRecovery?.status === 'loading',
    mode: archivedPathsOpen ? 'archived' : 'active',
    totalCount: ownedHomeDestination
      ? archivedPathsOpen ? ownedHomeDestination.profile.archivedPaths.length : ownedHomeDestination.profile.paths.length
      : 0,
    visibleCount: ownedHomeDestination
      ? archivedPathsOpen ? ownedHomeDestination.profile.archivedPaths.length : activeHomeVisibleCount
      : 0,
  });
  const homeNotice = ownedHomeDestination ? <>
    {accessState === 'authenticated_offline' && !errorKey && !offlineStatusDismissed ? <StatusBanner
      actionLabel={i18n.t('common.dismiss')}
      onAction={() => setOfflineStatusDismissed(true)}
      text={i18n.t('auth.offline')}
      tone="offline"
    /> : errorKey ? <StatusBanner text={i18n.t(errorKey)} tone="error" /> : null}
    {!archivedPathsOpen && homePreferenceErrorKey ? <StatusBanner
      actionLabel={i18n.t('common.retry')}
      onAction={() => void updateHomePreferences(homePreferences)}
      text={i18n.t(homePreferenceErrorKey)}
      tone="error"
    /> : null}
  </> : undefined;
  const currentDeepSocialEvent = currentSocialRouteIntent &&
    (currentSocialRouteIntent.kind === 'activity' || currentSocialRouteIntent.kind === 'comments' ||
      currentSocialRouteIntent.kind === 'comment-hearts')
    ? currentSocialRouteIntent.kind === 'activity'
      ? socialFeedPage.current.items.find((candidate) => candidate.type === 'practice_session' &&
        candidate.path.id === currentSocialRouteIntent.pathID && candidate.activity.id === currentSocialRouteIntent.activityID)
      : socialDeepEvent.eventID === currentSocialRouteIntent.eventID && socialDeepEvent.status === 'ready'
        ? socialDeepEvent.event
        : undefined
    : undefined;
  const deepSocialTargetUnavailable = Boolean(currentSocialRouteIntent &&
    currentSocialRouteIntent.kind === 'activity' && socialFeed.status === 'ready' &&
    !socialFeedPage.current.nextCursor && !currentDeepSocialEvent) ||
    Boolean(currentSocialRouteIntent &&
      (currentSocialRouteIntent.kind === 'comments' || currentSocialRouteIntent.kind === 'comment-hearts') &&
      socialDeepEvent.eventID === currentSocialRouteIntent.eventID && socialDeepEvent.status === 'error') ||
    Boolean(currentSocialRouteIntent?.kind === 'comment-hearts' && currentDeepSocialEvent &&
      practiceComments?.eventID === currentDeepSocialEvent.id && practiceComments.status === 'error') ||
    Boolean(currentSocialRouteIntent?.kind === 'comment-hearts' && currentDeepSocialEvent &&
      practiceComments?.eventID === currentDeepSocialEvent.id && practiceComments.status === 'ready' &&
      !practiceCommentPage.current.nextCursor &&
      !practiceCommentPage.current.items.some(({ id }) => id === currentSocialRouteIntent.commentID)) ||
    Boolean(currentSocialRouteIntent?.kind === 'activity' && socialFeed.detailErrorKey) ||
    Boolean(currentSocialRouteIntent?.kind === 'activity' && socialFeed.status === 'error') ||
    Boolean(currentSocialRouteIntent?.kind === 'comment-hearts' &&
      practiceCommentPage.current.items.some(({ heartCount, id, pending }) =>
        id === currentSocialRouteIntent.commentID && (heartCount <= 0 || pending)));
  const socialRouteRecoveryState: SocialRouteRecoveryState = ownedHomeDestination
    ? deepSocialTargetUnavailable ? 'unavailable' : 'loading'
    : homeRecovery?.status === 'offline' || accessState === 'authenticated_offline'
      ? 'offline'
      : homeRecovery?.status === 'error' || (ready && Boolean(session))
        ? 'unavailable'
        : 'loading';
  const retryCurrentSocialRoute = () => {
    if (!currentSocialRouteIntent) return;
    if (!ownedHomeDestination) {
      void retryAuthenticatedHome();
      return;
    }
    switch (currentSocialRouteIntent.kind) {
      case 'following':
        void loadSocialFeed('', true);
        void loadSocialActiveFollowing('', true);
        return;
      case 'people':
        void searchSocialProfiles(socialSearch.query, true);
        return;
      case 'follow-requests':
        void loadSocialFollowRequests(true);
        return;
      case 'profile':
        void loadSocialProfile(currentSocialRouteIntent.username, true);
        return;
      case 'activity':
        if (currentDeepSocialEvent?.type === 'practice_session') {
          void openSocialFeedActivity(currentDeepSocialEvent, false);
          return;
        }
        void loadSocialFeed('', true);
        return;
      case 'comments':
        if (currentDeepSocialEvent) {
          openPracticeComments(currentDeepSocialEvent.id, currentDeepSocialEvent.participant.userId, undefined, false);
          return;
        }
        void loadSocialDeepEvent(currentSocialRouteIntent.eventID);
        return;
      case 'comment-hearts':
        if (currentDeepSocialEvent) {
          const comment = practiceCommentPage.current.items.find(({ id }) => id === currentSocialRouteIntent.commentID);
          if (comment) {
            openPracticeCommentHeartRoster(comment, false);
            return;
          }
          openPracticeComments(currentDeepSocialEvent.id, currentDeepSocialEvent.participant.userId, undefined, false);
          return;
        }
        void loadSocialDeepEvent(currentSocialRouteIntent.eventID);
    }
  };
  return <SafeAreaView
    edges={destination ? ['left', 'right'] : ['top', 'left', 'right', 'bottom']}
    style={styles.screen}
  >
    {ownedHomeDestination && !selectedPath ? <HomeHeaderActions
      activeLabel={i18n.t('home.activePaths')}
      archivedLabel={i18n.t('home.archivedPaths')}
      createAccessibilityLabel={i18n.t('home.createPath')}
      filter={homeFilter}
      filterLabels={{
        all: i18n.t('home.filter.all'),
        shared: i18n.t('home.filter.shared'),
        solo: i18n.t('home.filter.solo'),
        supporting: i18n.t('home.filter.supporting'),
      }}
      mode={archivedPathsOpen ? 'archived' : 'active'}
      notificationsAccessibilityLabel={i18n.t('notification.bellLabel')}
      onArrange={() => setHomeArrangementOpen(true)}
      onCreate={beginPathCreation}
      onFilterChange={setHomeFilter}
      onModeChange={(mode) => {
        setArchivedPathsOpen(mode === 'archived');
        setPathArchiveSavedKey(null);
      }}
      onOrderChange={(order) => void updateHomePreferences({ ...homePreferences, order })}
      onOpenNotifications={() => void openNotifications()}
      onOpenSettings={() => router.push('/settings')}
      pathViewAccessibilityLabel={i18n.t('home.pathViewLabel')}
      order={homePreferences.order}
      orderAccessibilityLabel={i18n.t('home.order.label')}
      orderLabels={{
        alphabetical: i18n.t('home.order.alphabetical'),
        arrange: i18n.t('home.arrange.title'),
        manual: i18n.t('home.order.manual'),
        recent: i18n.t('home.order.recent'),
      }}
      settingsAccessibilityLabel={i18n.t('settings.openLabel')}
    /> : null}
    {ownedHomeDestination ? <NativeSheet
      onRequestClose={() => { if (!homePreferenceBusy) setHomeArrangementOpen(false); }}
      scrollable={false}
      title={i18n.t('home.arrange.title')}
      trailingAction={{
        disabled: homePreferenceBusy,
        label: i18n.t('home.arrange.done'),
        onPress: () => setHomeArrangementOpen(false),
      }}
      visible={homeArrangementOpen}
    >
      <HomeArrangementView
        busy={homePreferenceBusy}
        i18n={i18n}
        onMove={(collection: HomeArrangementCollection, sourceIndices, moveDestination) => {
          const pinned = new Set(homePreferences.pinnedPathIDs);
          const visiblePathIDs = ownedHomeDestination.profile.paths
            .filter(({ id }) => collection === 'pinned' ? pinned.has(id) : !pinned.has(id))
            .map(({ id }) => id);
          void updateHomePreferences(reorderVisibleHomePaths(
            homePreferences,
            collection,
            visiblePathIDs,
            sourceIndices,
            moveDestination,
          ));
        }}
        onPinChange={(pathID, pinned) => void updateHomePreferences(
          pinned ? pinHomePath(homePreferences, pathID) : unpinHomePath(homePreferences, pathID),
        )}
        paths={ownedHomeDestination.profile.paths}
        preferences={homePreferences}
      />
    </NativeSheet> : null}
    {ownedHomeDestination ? <SettingsPresentationSource
      displayName={ownedHomeDestination.profile.displayName}
      email={ownedHomeDestination.profile.email}
      sessionKey={socialPresentationKey}
      isCurrent={() => socialPresentationKey === `${notificationLifecycleState.current.destination?.kind === 'home'
        ? notificationLifecycleState.current.destination.profile.id
        : 'unavailable'}:${socialPresentationGeneration.current}`}
      getInteractionSettings={getInteractionSettings}
      getNudgeChannelPreference={getNudgeChannelPreference}
      getConfiguredTimeZone={getConfiguredTimeZone}
      runningTimerCount={Object.values(ownedHomeDestination.profile.timers).filter((state) => state.running).length}
      synchronizePushPermission={synchronizePushPermission}
      signOut={resolveTimerSignOut}
      updateInteractionSettings={updateInteractionSettings}
      updateNudgeChannelPreference={updateNudgeChannelPreference}
      updateConfiguredTimeZone={updateConfiguredTimeZone}
    /> : null}
    {ownedHomeDestination ? <UserBlockingRouteSource
      isCurrent={() => socialPresentationKey === `${notificationLifecycleState.current.destination?.kind === 'home'
        ? notificationLifecycleState.current.destination.profile.id
        : 'unavailable'}:${socialPresentationGeneration.current}`}
      onBlocked={(target) => hideBlockedUserFromSocialSurfaces(target.userId)}
      port={userBlockingPort}
      sessionKey={socialPresentationKey}
    /> : null}
    {!ready || ownedHomeDestination || (session?.nextAction === 'home' && !destination) ? <HomeView
      notice={<>
        {ready && ownedHomeDestination && notificationTapFeedbackKey ? <StatusBanner
          actionLabel={i18n.t('common.dismiss')}
          onAction={() => setNotificationTapFeedbackKey(null)}
          text={i18n.t(notificationTapFeedbackKey)}
          tone="offline"
        /> : null}
        {homeNotice}
      </>}
      onClearFilter={() => setHomeFilter('all')}
      onCreate={beginPathCreation}
      onRetry={() => void retryAuthenticatedHome()}
      presentation={currentHomePresentation}
      sections={homeViewSections}
    /> : null}
    {ready && ownedHomeDestination ? <>
      {selectedPath ? <NativeRouteSource
        actions={[
          {
            disabled: pathLeaveBusy,
            label: i18n.t('pathDetails.openHistory'),
            onPress: () => void openActivityHistory(selectedPath.id),
            systemImage: 'clock.arrow.circlepath',
          },
          ...([{
            disabled: pathLeaveBusy || pathMemberRemovalBusy || pathMemberUnblockBusy || pathMemberRoleChangeBusy,
            label: i18n.t('pathMembers.heading'),
            onPress: () => void openPathMembers(selectedPath.id),
            systemImage: 'person.2',
          } satisfies NativeRouteAction]),
          ...(selectedCapabilities.trackTime && !selectedPath.archivedAt ? [{
            disabled: pathNudgePreferenceBusy,
            label: i18n.t('nudge.audience.openLabel'),
            onPress: () => void loadPathNudgePreference(selectedPath.id),
            systemImage: 'hand.wave',
          } satisfies NativeRouteAction] : []),
          ...((selectedCapabilities.inviteMembers || selectedCapabilities.manageMembers || selectedCapabilities.manageVisibility) ? [{
            disabled: pathLeaveBusy || pathRenamePathID === selectedPath.id || invitationReviewBusy || invitationSendBusy || ownershipTransferPathID === selectedPath.id,
            label: i18n.t('pathInvitation.share'),
            onPress: () => openPathSharing(selectedPath),
            systemImage: 'person.badge.plus',
          } satisfies NativeRouteAction] : []),
          ...((selectedCapabilities.manageGoals || selectedCapabilities.renamePath || selectedCapabilities.manageLifecycle ||
            selectedCapabilities.transferOwnership ||
            (ownershipTransferPathID === selectedPath.id && Boolean(pendingOwnershipTransfer))) ? [{
            disabled: pathLeaveBusy || pathRenamePathID === selectedPath.id || goalManagementBusy || pathDeletionBusy || manualBusy || Boolean(timerBusy[selectedPath.id]),
            label: i18n.t('pathManage.action'),
            onPress: () => openPathManagement(selectedPath),
            systemImage: 'slider.horizontal.3',
          } satisfies NativeRouteAction] : []),
          ...(selectedCapabilities.leavePath ? [{
            destructive: true,
            disabled: pathLeaveBusy || pathRenameBusy || goalManagementBusy || manualBusy || Boolean(timerBusy[selectedPath.id]),
            label: i18n.t(pathLeaveBusy ? 'pathLeave.leaving' : 'pathLeave.action'),
            onPress: () => reviewPathLeaveIntent(selectedPath),
            systemImage: 'rectangle.portrait.and.arrow.right',
          } satisfies NativeRouteAction] : []),
        ]}
        onDismiss={() => {
          if (pathLeaveBusy) return;
          resetGoalManagement(); resetManualActivity(); resetPathDetail();
        }}
        pathID={selectedPath.id}
        title={selectedPath.name}
      ><>
      <PathDetailView
        accumulatedSeconds={selectedTimerState?.accumulatedSeconds}
        actionDisabled={pathLeaveBusy || pathRenamePathID === selectedPath.id || manualBusy || goalManagementPathID === selectedPath.id}
        archived={Boolean(selectedPath.archivedAt)}
        busy={manualBusy}
        canTrackTime={selectedCapabilities.trackTime}
        comparisonFirst={!selectedCapabilities.trackTime}
        intervalProgress={selectedIntervalProgress ? <IntervalProgressIndicator progress={selectedIntervalProgress} /> : undefined}
        onAddActivity={() => void openManualActivity(selectedPath.id)}
        onOpenHistory={() => void openActivityHistory(selectedPath.id)}
        overallProgress={selectedOverallProgress ? <OverallProgressIndicator progress={selectedOverallProgress} /> : undefined}
        participantComparison={<PathMemberManagementView
          i18n={i18n}
          onLoadMore={() => { if (pathMembersCursor) void openPathMembers(selectedPath.id, pathMembersCursor, false); }}
          onOpen={(member) => void inspectPathMember(member)}
          onRetry={() => void openPathMembers(selectedPath.id, undefined, false)}
          state={{
            error: pathMembersError,
            items: pathMembers.filter((member) => member.role !== 'supporter'),
            loading: pathMembersLoading && pathMembers.length === 0,
            loadingMore: pathMembersLoadingMore,
            nextCursor: pathMembersCursor ?? undefined,
          } satisfies PathMemberListState}
        />}
      />
      {pathLeaveBusy ? <StatusBanner text={i18n.t('pathLeave.leaving')} /> : null}
      {pathLeaveErrorKey ? <StatusBanner text={i18n.t(pathLeaveErrorKey)} tone="error" /> : null}
      {sharingPath && (selectedCapabilities.inviteMembers || selectedCapabilities.manageVisibility || selectedCapabilities.manageMembers) ? <PathShareSheet
        busy={invitationReviewBusy || invitationSendBusy || pathVisibilityBusy || Object.values(managedInvitationBusy).some(Boolean)}
        canInvite={selectedCapabilities.inviteMembers}
        effectiveVisibility={selectedPath.visibility}
        errorText={invitationErrorKey ? i18n.t(invitationErrorKey) : undefined}
        managedInvitationBusy={managedInvitationBusy}
        managedInvitationErrors={managedInvitationErrors}
        managedInvitationsBusy={managedInvitationsBusy}
        managedInvitationsErrorKey={managedInvitationsErrorKey}
        i18n={i18n}
        onCancel={closePathSharing}
        onCancelManagedInvitation={(managed) => void cancelManagedInvitation(managed)}
        onChangeRole={chooseInvitationRole}
        onChangeUsername={changeInvitationUsername}
        onInvite={() => void sendReviewedInvitation()}
        onLoadMorePeople={() => {
          if (pathMembersCursor) void openPathMembers(selectedPath.id, pathMembersCursor, false, false);
        }}
        onLoadMoreManagedInvitations={() => {
          if (selectedPath && managedInvitationState.nextCursor) {
            void loadManagedInvitations(selectedPath.id, managedInvitationState.nextCursor);
          }
        }}
        onRetryManagedInvitations={() => {
          if (selectedPath) void loadManagedInvitations(selectedPath.id);
        }}
        onRetryPeople={() => void openPathMembers(selectedPath.id, undefined, false, false)}
        onOpenPerson={(userID) => {
          const member = pathMembers.find((candidate) => candidate.userId === userID);
          if (!member) return;
          resetInvitationShare();
          void inspectPathMember(member);
        }}
        onReview={() => void reviewInvitationRecipient()}
        onVisibilityChange={selectedCapabilities.manageVisibility ? changePathVisibility : undefined}
        people={pathMembers.map((member) => ({
          canManage: member.canChangeRole || member.canGrantAdministrator || member.canRemove ||
            member.canRevokeAdministrator || member.canStepDownAdministrator,
          displayName: member.displayName,
          role: member.role,
          userID: member.userId,
          username: member.username,
        }))}
        peopleError={pathMembersError}
        peopleLoading={pathMembersLoading && pathMembers.length === 0}
        peopleLoadingMore={pathMembersLoadingMore}
        peopleNextCursor={pathMembersCursor ?? undefined}
        pendingInvitations={managedInvitationState}
        review={invitationReview}
        reviewed={Boolean(invitationReview)}
        role={invitationRole}
        sent={invitationSent}
        username={invitationUsername}
        visibilityOptions={selectedCapabilities.manageVisibility
          ? pathVisibilityOptions(ownedHomeDestination.profile.profileVisibility)
          : undefined}
      /> : null}
      {ownershipTransferPathID === selectedPath.id ? <>
        <OwnershipTransferSheet
          actionsDisabled={Boolean(selectedPath.archivedAt)}
          busy={ownershipTransferBusyAction !== undefined || ownershipTransferReviewBusy || pendingOwnershipTransferLoading}
          busyAction={ownershipTransferBusyAction}
          candidates={{
            errorKey: ownershipTransferCandidatesErrorKey,
            items: ownershipTransferCandidates,
            loading: ownershipTransferCandidatesLoading,
            nextCursor: ownershipTransferCandidatesCursor || undefined,
          }}
          errorText={ownershipTransferErrorKey ? i18n.t(ownershipTransferErrorKey) : undefined}
          expirationSummary={ownershipTransferExpirationSummary}
          i18n={i18n}
          onAcceptPending={() => void mutatePendingOwnershipTransfer('accept')}
          onCancelPending={() => void mutatePendingOwnershipTransfer('cancel')}
          onClose={closeOwnershipTransfer}
          onConfirm={(recipient) => void confirmOwnershipTransfer(recipient)}
          onDeclinePending={() => void mutatePendingOwnershipTransfer('decline')}
          onLoadCandidates={() => void loadOwnershipTransferCandidates()}
          onLoadMoreCandidates={() => void loadOwnershipTransferCandidates(ownershipTransferCandidatesCursor)}
          onSelectRecipient={(recipient) => {
            if (ownershipTransferBusyAction || ownershipTransferReviewBusy) return;
            void reviewOwnershipTransferRecipient(recipient);
          }}
          pending={pendingOwnershipTransfer}
          selectedRecipientID={ownershipTransferSelectedRecipient?.userID}
          visible={ownershipTransferOpen}
        />
      </> : null}
      {selectedCapabilities.trackTime && manualPathID
        ? ownsManualActivityPresentation(manualActivityPresentationOwner, 'path-details') ? manualActivityPresentation : null
        : null}
      {selectedCapabilities.manageLifecycle && pathArchiveReview ? <PathArchiveConfirmationSheet
        archived={pathArchiveReview.archived}
        busy={pathArchiveBusy}
        errorText={pathArchiveErrorKey ? i18n.t(pathArchiveErrorKey) : undefined}
        i18n={i18n}
        onCancel={cancelPathArchiveChange}
        onConfirm={() => void confirmPathArchiveChange()}
        visible
      /> : null}
      {(selectedCapabilities.manageGoals || selectedCapabilities.renamePath || selectedCapabilities.manageLifecycle ||
        selectedCapabilities.transferOwnership || (ownershipTransferPathID === selectedPath.id && Boolean(pendingOwnershipTransfer))) &&
        goalManagementPathID && goalManagementForm ? <PathGoalManagementForm
        archived={Boolean(selectedPath.archivedAt)}
        busy={goalManagementBusy}
        canDelete={selectedCapabilities.manageLifecycle}
        canManageGoals={selectedCapabilities.manageGoals}
        canManageVisibility={false}
        canRename={selectedCapabilities.renamePath}
        deleteBusy={pathDeletionBusy}
        deleteErrorText={pathDeletionErrorKey ? i18n.t(pathDeletionErrorKey) : undefined}
        errorText={goalManagementErrorKey ? i18n.t(goalManagementErrorKey) : undefined}
        form={goalManagementForm}
        goalCanReview={goalManagementCanReview}
        i18n={i18n}
        onCancel={closePathManagement}
        onCancelReview={cancelPathGoalReview}
        onConfirm={() => void confirmPathGoalChanges()}
        onDeleteCancel={cancelPathDeletion}
        onDeleteConfirm={() => void confirmPathDeletion()}
        onDeleteReview={reviewPathDeletionIntent}
        onOpenArchive={selectedCapabilities.manageLifecycle ? (dirty) => {
          handoffPathManagement(dirty, 'archive');
        } : undefined}
        onOpenOwnershipTransfer={selectedCapabilities.transferOwnership ||
          (ownershipTransferPathID === selectedPath.id && Boolean(pendingOwnershipTransfer)) ? (dirty) => {
            handoffPathManagement(dirty, 'ownership-transfer');
          } : undefined}
        onReview={reviewPathGoalChanges}
        onVisibilityCancelReview={cancelPathVisibilityReview}
        onVisibilityChange={changePathVisibility}
        onVisibilityConfirm={() => void confirmPathVisibility()}
        onVisibilityReview={reviewPathVisibility}
        onRenameChange={changePathRenameName}
        onRenameSave={() => void submitPathRename()}
        onUpdate={updateGoalManagementForm}
        renameBusy={pathRenameBusy}
        renameCurrentName={selectedPath.name}
        renameErrorText={pathRenameErrorKey ? i18n.t(pathRenameErrorKey) : undefined}
        renameName={pathRenameName}
        renameSavedName={pathRenameSavedName ?? undefined}
        pathName={selectedPath.name}
        review={goalManagementReview ?? undefined}
        saved={goalManagementSaved}
        visible
        visibilityBusy={pathVisibilityBusy}
        visibilityCurrent={selectedPath.visibility}
        visibilityDraft={pathVisibilityDraft}
        visibilityErrorText={pathVisibilityErrorKey ? i18n.t(pathVisibilityErrorKey) : undefined}
        visibilityOptions={pathVisibilityOptions(ownedHomeDestination.profile.profileVisibility)}
        visibilityReview={pathVisibilityReview ?? undefined}
        visibilitySaved={pathVisibilitySaved}
      /> : null}
      </></NativeRouteSource> : null}
      {currentPathRouteIntent && !selectedPath ? <NativeRouteSource
        actions={[]}
        onDismiss={() => {
          pathRouteTarget.current = null;
          setPathRouteRecovery(null);
        }}
        pathID={currentPathRouteIntent.pathID}
        title={i18n.t('pathDetails.genericTitle')}
      >
        <NativeRouteRecoveryView
          onGoHome={leavePathRouteToHome}
          onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
            ? () => void retryPathRoute(currentPathRouteIntent)
            : undefined}
          state={pathRouteRecovery ?? 'loading'}
        />
      </NativeRouteSource> : null}
      {currentPathRouteIntent?.kind === 'history' && !selectedPath ? <NativeChildRouteSource
        onDismiss={() => {
          pathRouteTarget.current = null;
          setPathRouteRecovery(null);
        }}
        routeKey={currentPathRouteIntent.routeKey}
        title={i18n.t('pathDetails.history')}
      >
        <NativeRouteRecoveryView
          onGoHome={leavePathRouteToHome}
          onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
            ? () => void retryPathRoute(currentPathRouteIntent)
            : undefined}
          state={pathRouteRecovery ?? 'loading'}
        />
      </NativeChildRouteSource> : null}
      {currentPathRouteIntent?.kind === 'activity' && !selectedPath ? <NativeChildRouteSource
        onDismiss={() => {
          pathRouteTarget.current = null;
          setPathRouteRecovery(null);
        }}
        routeKey={currentPathRouteIntent.routeKey}
        title={i18n.t('pathDetails.activityHeading')}
      >
        <NativeRouteRecoveryView
          onGoHome={leavePathRouteToHome}
          onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
            ? () => void retryPathRoute(currentPathRouteIntent)
            : undefined}
          state={pathRouteRecovery ?? 'loading'}
        />
      </NativeChildRouteSource> : null}
      {currentPathRouteIntent?.kind === 'members' && !selectedPath ? <NativeChildRouteSource
        grouped
        onDismiss={() => {
          pathRouteTarget.current = null;
          setPathRouteRecovery(null);
        }}
        routeKey={pathMembersRouteKey(currentPathRouteIntent.pathID)}
        title={i18n.t('pathMembers.heading')}
      >
        <NativeRouteRecoveryView
          onGoHome={leavePathRouteToHome}
          onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
            ? () => void retryPathRoute(currentPathRouteIntent)
            : undefined}
          state={pathRouteRecovery ?? 'loading'}
        />
      </NativeChildRouteSource> : null}
      {currentPathRouteIntent?.kind === 'member' && !selectedPath ? <NativeChildRouteSource
        grouped
        onDismiss={() => {
          pathRouteTarget.current = null;
          setPathRouteRecovery(null);
        }}
        routeKey={pathMemberRemovalRouteKey(currentPathRouteIntent.pathID, currentPathRouteIntent.userID)}
        title={i18n.t('pathMembers.memberHeading')}
      >
        <NativeRouteRecoveryView
          onGoHome={leavePathRouteToHome}
          onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
            ? () => void retryPathRoute(currentPathRouteIntent)
            : undefined}
          state={pathRouteRecovery ?? 'loading'}
        />
      </NativeChildRouteSource> : null}
      {selectedPath && pathMembersOpen ? <NativeChildRouteSource
        grouped
        onDismiss={() => closePathMembersRoute(true)}
        onRefresh={() => void openPathMembers(selectedPath.id)}
        refreshing={pathMembersLoading && pathMembers.length > 0}
        routeKey={pathMembersRouteKey(selectedPath.id)}
        title={i18n.t('pathMembers.heading')}
      >
        {currentPathRouteIntent?.kind === 'members' && currentPathRouteIntent.pathID === selectedPath.id && pathRouteRecovery
          ? <NativeRouteRecoveryView
            onGoHome={leavePathRouteToHome}
            onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
              ? () => void recoverPathRoute(currentPathRouteIntent)
              : undefined}
            state={pathRouteRecovery}
          />
          : <PathMemberManagementView
          i18n={i18n}
          onLoadMore={() => { if (pathMembersCursor) void openPathMembers(selectedPath.id, pathMembersCursor); }}
          onOpen={(member) => void inspectPathMember(member)}
          onRetry={() => void openPathMembers(selectedPath.id)}
          state={{
            error: pathMembersError,
            items: pathMembers,
            loading: pathMembersLoading && pathMembers.length === 0,
            loadingMore: pathMembersLoadingMore,
            nextCursor: pathMembersCursor ?? undefined,
          } satisfies PathMemberListState}
        />}
      </NativeChildRouteSource> : null}
      {selectedPath && selectedPathMember ? <NativeChildRouteSource
        dismissible={!pathMemberRemovalBusy && !pathMemberUnblockBusy && !pathMemberRoleChangeBusy && !nudgeSendBusy}
        grouped
        onDismiss={closePathMemberRemovalRoute}
        routeKey={pathMemberRemovalRouteKey(selectedPath.id, selectedPathMember.userId)}
        title={selectedPathMember.displayName}
      >
        <PathMemberRemovalReviewView
          activities={pathMemberActivities}
          activitiesBusy={pathMemberActivitiesLoading}
          activitiesErrorText={pathMemberActivitiesErrorKey ? i18n.t(pathMemberActivitiesErrorKey) : undefined}
          activitiesHaveMore={Boolean(pathMemberActivitiesCursor)}
          busy={pathMemberRemovalBusy}
          errorText={pathMemberRemovalErrorKey ? i18n.t(pathMemberRemovalErrorKey) : undefined}
          i18n={i18n}
          loading={pathMemberReviewLoading}
          member={selectedPathMember}
          onCancelRoleChange={cancelPathMemberRoleChange}
          onChooseRole={choosePathMemberRole}
          onConfirmRoleChange={() => void changeSelectedPathMemberRole()}
          onLoadMoreActivities={() => void loadPathMemberActivities(selectedPathMember, pathMemberActivitiesCursor ?? undefined)}
          onOpenActivity={(activityID) => openPathMemberActivity(selectedPath.id, activityID)}
          onRemove={() => void removeSelectedPathMember()}
          onRetry={() => void inspectPathMember(selectedPathMember, false)}
          onSendNudge={() => openNudgeComposer(selectedPathMember)}
          onUnblock={() => void unblockPathMember(selectedPathMember)}
          nudgeConfirmationText={nudgeSendConfirmationKey ? i18n.t(nudgeSendConfirmationKey) : undefined}
          pendingRole={pathMemberPendingRole}
          review={pathMemberRemovalReview}
          roleChangeBusy={pathMemberRoleChangeBusy}
          roleChangeErrorText={pathMemberRoleChangeErrorKey ? i18n.t(pathMemberRoleChangeErrorKey) : undefined}
          unblockBusy={pathMemberUnblockBusy}
          unblockErrorText={pathMemberUnblockErrorKey ? i18n.t(pathMemberUnblockErrorKey) : undefined}
        />
        <NudgeComposerSheet
          busy={nudgeSendBusy}
          errorText={nudgeSendErrorKey ? i18n.t(nudgeSendErrorKey) : undefined}
          i18n={i18n}
          onCancel={closeNudgeComposer}
          onSelect={selectNudgeComposerPreset}
          onSend={() => void sendSelectedNudge()}
          pathName={selectedPath.name}
          recipientDisplayName={selectedPathMember.displayName}
          selectedPreset={nudgeComposerPreset}
          visible={nudgeComposerOpen}
        />
      </NativeChildRouteSource> : null}
      {selectedPath && currentPathRouteIntent?.kind === 'member' &&
      selectedPathMember?.userId !== currentPathRouteIntent.userID && pathRouteRecovery ? <NativeChildRouteSource
        grouped
        onDismiss={closePathMemberRemovalRoute}
        routeKey={pathMemberRemovalRouteKey(selectedPath.id, currentPathRouteIntent.userID)}
        title={i18n.t('pathMembers.memberHeading')}
      >
        <NativeRouteRecoveryView
          onGoHome={leavePathRouteToHome}
          onRetry={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'
            ? () => void recoverPathRoute(currentPathRouteIntent)
            : undefined}
          state={pathRouteRecovery}
        />
      </NativeChildRouteSource> : null}
      {currentPathRouteIntent?.kind === 'nudge-settings' && (!selectedPath || !pathNudgeSettingsOpen) ? <NativeChildRouteSource
        grouped
        onDismiss={() => closePathNudgeSettings()}
        routeKey={pathNudgeSettingsRouteKey(currentPathRouteIntent.pathID)}
        title={i18n.t('nudge.audience.heading')}
      >
        <PathNudgeSettingsView
          busy={false}
          error={pathRouteRecovery === 'unavailable' || pathRouteRecovery === 'offline'}
          i18n={i18n}
          loading={pathRouteRecovery === null || pathRouteRecovery === 'loading'}
          onGoHome={leavePathRouteToHome}
          onRetry={() => void retryPathRoute(currentPathRouteIntent)}
          onSelect={() => undefined}
          preference={null}
          recoveryState={pathRouteRecovery ?? 'loading'}
        />
      </NativeChildRouteSource> : null}
      {selectedPath && pathNudgeSettingsOpen ? <NativeChildRouteSource
        dismissible={!pathNudgePreferenceBusy}
        grouped
        onDismiss={() => closePathNudgeSettings()}
        routeKey={pathNudgeSettingsRouteKey(selectedPath.id)}
        title={i18n.t('nudge.audience.heading')}
      >
        <PathNudgeSettingsView
          busy={pathNudgePreferenceBusy}
          error={pathNudgePreferenceError}
          i18n={i18n}
          loading={pathNudgePreferenceLoading}
          onGoHome={leavePathRouteToHome}
          onRetry={() => void loadPathNudgePreference(selectedPath.id, false)}
          onSelect={(audience) => void updatePathNudgeAudience(audience)}
          preference={pathNudgePreference}
          recoveryState={!pathNudgePreference ? pathRouteRecovery ?? 'loading' : undefined}
        />
      </NativeChildRouteSource> : null}
      {selectedPath && activityHistoryOpen ? <NativeChildRouteSource
        onDismiss={closeActivityHistoryRoute}
        routeKey={activityHistoryRouteKey(selectedPath.id)}
        title={i18n.t('pathDetails.history')}
      >
        <ActivityHistoryView
          activities={activityHistory}
          busy={pathDetailBusy}
          errorText={activityHistoryErrorKey ? i18n.t(activityHistoryErrorKey) : undefined}
          hasMore={Boolean(activityHistoryCursor)}
          onLoadMore={() => { if (activityHistoryCursor) void openActivityHistory(selectedPath.id, activityHistoryCursor); }}
          onOpenActivity={(activityID) => void inspectActivity(selectedPath.id, activityID)}
          onRetry={() => void openActivityHistory(selectedPath.id, activityHistoryCursor ?? undefined)}
        />
      </NativeChildRouteSource> : null}
      {selectedPath && activityHistoryOpen && activeActivityID ? <NativeChildRouteSource
        dismissible={!activityDeletionBusy}
        onDismiss={() => closeActivityDetailRoute(activeActivityID)}
        routeKey={activityDetailRouteKey(selectedPath.id, activeActivityID)}
        title={i18n.t('pathDetails.activityHeading')}
      >
        <ActivityDetailView
          activity={selectedActivity}
          archived={!selectedCapabilities.trackTime || Boolean(selectedPath.archivedAt)}
          busy={pathDetailBusy}
          deletionBusy={activityDeletionBusy}
          deletionErrorText={activityDeletionErrorKey ? i18n.t(activityDeletionErrorKey) : undefined}
          deletionRetryable={activityDeletionRetryable}
          errorText={pathDetailErrorKey ? i18n.t(pathDetailErrorKey) : undefined}
          hasMoreRevisions={Boolean(activityRevisionCursor)}
          manualBusy={manualBusy}
          onDelete={beginActivityDeletion}
          onEdit={() => void editSelectedPathActivity()}
          onLoadMoreRevisions={() => {
            if (activityRevisionCursor) void loadMoreActivityRevisions(selectedPath.id, activeActivityID, activityRevisionCursor);
          }}
          onRetry={() => void inspectActivity(selectedPath.id, activeActivityID, false)}
          onRetryDeletion={beginActivityDeletion}
          onRetryRevisions={() => activityRevisionCursor
            ? void loadMoreActivityRevisions(selectedPath.id, activeActivityID, activityRevisionCursor)
            : void inspectActivity(selectedPath.id, activeActivityID, false)}
          profileID={ownedHomeDestination.profile.id}
          revisionErrorText={activityRevisionErrorKey ? i18n.t(activityRevisionErrorKey) : undefined}
          revisions={activityRevisions}
        />
        {ownsManualActivityPresentation(manualActivityPresentationOwner, 'activity-details') ? manualActivityPresentation : null}
      </NativeChildRouteSource> : null}
      {pathCreated ? <Text accessibilityRole="alert">{i18n.t('pathCreate.created')}</Text> : null}
      {pathArchiveSavedKey ? <Text accessibilityLiveRegion="polite">{i18n.t(pathArchiveSavedKey)}</Text> : null}
      {timerNoticeKey ? <Text accessibilityLiveRegion="polite">{i18n.t(timerNoticeKey)}</Text> : null}
      {pathLeaveNotice ? <Text accessibilityLiveRegion="polite">{pathLeaveNotice}</Text> : null}
      {creatingPath ? <PathCreateForm
        busy={pathSubmitting}
        errorText={pathErrorKey ? i18n.t(pathErrorKey) : undefined}
        form={pathGoalForm}
        i18n={i18n}
        name={pathName}
        onCancel={cancelPathCreation}
        onCreate={() => void createPath()}
        onNameChange={updatePathName}
        onVisibilityChange={updatePathVisibility}
        onUpdate={updatePathGoalForm}
        visibility={pathVisibility}
        visibilityOptions={pathVisibilityOptions(ownedHomeDestination.profile.profileVisibility)}
        visible={creatingPath}
      /> : null}
    </> : null}
    {currentSocialRouteIntent ? <SocialRouteRecoverySource
      onGoFollowing={() => router.replace('/(tabs)/following')}
      onGoHome={() => router.replace('/(tabs)/home')}
      onRetry={session ? retryCurrentSocialRoute : undefined}
      state={socialRouteRecoveryState}
      target={currentSocialRouteIntent}
    /> : null}
    {currentNotificationJourneyIntent && ownedHomeDestination ? <NotificationJourneyRecoverySource
      intent={currentNotificationJourneyIntent}
      onHome={() => router.replace('/(tabs)/home')}
      onRetry={() => retryNotificationJourney(currentNotificationJourneyIntent)}
      sessionKey={socialPresentationKey}
      state={currentNotificationJourneyIntent.kind === 'notifications' && notificationsErrorKey
        ? 'offline'
        : currentNotificationJourneyIntent.kind === 'invitations' && pendingInvitationsErrorKey
          ? 'offline'
          : accessState === 'authenticated_offline'
            ? 'offline'
            : 'loading'}
    /> : null}
    {currentSettingsJourneyIntent && ownedHomeDestination ? <SettingsJourneyRecoverySource
      intent={currentSettingsJourneyIntent}
      onHome={() => router.replace('/(tabs)/home')}
      onRetry={() => void retryAuthenticatedHome()}
      sessionKey={socialPresentationKey}
      state={accessState === 'authenticated_offline' ? 'offline' : 'loading'}
    /> : null}
    {ready && ownedHomeDestination ? <SocialFeedRouteSource
      active={socialActiveFollowing}
      dismissInteractionNotice={() => setSocialFeed((current) => ({
        ...current,
        interactionNoticeEventID: undefined,
        interactionNoticeKey: undefined,
      }))}
      feed={socialFeed}
      loadActive={() => void loadSocialActiveFollowing()}
      loadFeed={() => void loadSocialFeed()}
      loadMoreActive={() => {
        if (socialActiveFollowingPage.current.nextCursor) {
          void loadSocialActiveFollowing(socialActiveFollowingPage.current.nextCursor);
        }
      }}
      loadMore={() => {
        if (socialFeedPage.current.nextCursor) void loadSocialFeed(socialFeedPage.current.nextCursor);
      }}
      openActivity={(event) => void openSocialFeedActivity(event)}
      openComments={(event) => openPracticeComments(event.id, event.participant.userId)}
      removeReaction={(event) => mutateSocialFeedReaction(event, null)}
      refresh={() => void loadSocialFeed('', true)}
      refreshActive={() => void loadSocialActiveFollowing('', true)}
      retry={() => void loadSocialFeed()}
      retryActive={() => void loadSocialActiveFollowing()}
      setReaction={(event, reaction) => mutateSocialFeedReaction(event, reaction)}
    /> : null}
    {ready && ownedHomeDestination && practiceComments ? <PracticeCommentsRouteSource
      busy={practiceComments.busy}
      create={createPracticeComment}
      edit={editPracticeComment}
      errorKey={practiceComments.errorKey}
      eventID={practiceComments.eventID}
      eventOwnerID={practiceComments.eventOwnerID}
      focusedCommentID={practiceComments.focusedCommentID}
      history={practiceComments.history}
      items={practiceCommentPage.current.items}
      loadHistory={(comment, cursor) => void loadPracticeCommentHistory(comment, cursor)}
      loadMore={() => {
        const target = practiceCommentTarget.current;
        if (target && practiceCommentPage.current.nextCursor) void loadPracticeComments(target, practiceCommentPage.current.nextCursor);
      }}
      loadingMore={practiceComments.loadingMore}
      mutateHeart={mutatePracticeCommentHeart}
      nextCursor={practiceCommentPage.current.nextCursor || undefined}
      onUnavailable={() => router.replace('/(tabs)/following')}
      openHeartRoster={openPracticeCommentHeartRoster}
      refresh={() => {
        const target = practiceCommentTarget.current;
        if (target) {
          practiceCommentPage.current = { items: [], nextCursor: '' };
          void loadPracticeComments(target, '', true);
        }
      }}
      refreshing={practiceComments.refreshing}
      remove={deletePracticeComment}
      retry={() => {
        const target = practiceCommentTarget.current;
        if (target) void loadPracticeComments(target);
      }}
      status={practiceComments.status}
      viewerID={ownedHomeDestination.profile.id}
    /> : null}
    {ready && ownedHomeDestination && practiceCommentHeartRoster && practiceCommentHeartRosterPage.current
      ? <CommentHeartRosterRouteSource
          commentID={practiceCommentHeartRoster.commentID}
          errorKey={practiceCommentHeartRoster.errorKey}
          eventID={practiceCommentHeartRoster.eventID}
          items={practiceCommentHeartRosterPage.current.items}
          loadMore={() => {
            const target = practiceCommentHeartRosterTarget.current;
            const page = practiceCommentHeartRosterPage.current;
            if (target && page?.nextCursor) void loadPracticeCommentHeartRoster(target, page.nextCursor);
          }}
          loadingMore={practiceCommentHeartRoster.loadingMore}
          nextCursor={practiceCommentHeartRosterPage.current.nextCursor || undefined}
          onUnavailable={() => router.back()}
          openProfile={(person: PracticeCommentHearter) => {
            void loadSocialProfile(person.username);
            router.push({ pathname: '/profile/[username]', params: { username: person.username } });
          }}
          refresh={() => {
            const target = practiceCommentHeartRosterTarget.current;
            if (target) void loadPracticeCommentHeartRoster(target, '', true);
          }}
          refreshing={practiceCommentHeartRoster.refreshing}
          retry={() => {
            const target = practiceCommentHeartRosterTarget.current;
            const page = practiceCommentHeartRosterPage.current;
            if (target && page) void loadPracticeCommentHeartRoster(target, page.items.length ? page.nextCursor : '');
          }}
          status={practiceCommentHeartRoster.status}
        />
      : null}
    {ready && ownedHomeDestination ? <SocialProfileRouteSource
      isCurrent={() => socialPresentationKey === `${notificationLifecycleState.current.destination?.kind === 'home'
        ? notificationLifecycleState.current.destination.profile.id
        : 'unavailable'}:${socialPresentationGeneration.current}`}
      loadMore={() => void loadMoreSocialProfiles()}
      loadFollowRequests={(refreshing) => void loadSocialFollowRequests(refreshing)}
      loadMoreFollowRequests={() => {
        if (socialFollowRequestPage.current.nextCursor) void loadSocialFollowRequests(false, socialFollowRequestPage.current.nextCursor);
      }}
      loadProfile={(username) => void loadSocialProfile(username)}
      mutateRelationship={(action, username, idempotencyKey) => void mutateSocialRelationship(action, username, idempotencyKey)}
      profile={socialProfile}
      followRequests={socialFollowRequests}
      refreshProfile={() => {
        if (socialProfile.username) void loadSocialProfile(socialProfile.username, true);
      }}
      refreshSearch={() => void searchSocialProfiles(socialSearch.query, true)}
      retryProfile={() => {
        if (socialProfile.username) void loadSocialProfile(socialProfile.username);
      }}
      retrySearch={() => void searchSocialProfiles(socialSearch.query)}
      reviewFollowRequest={(decision, requestID, idempotencyKey) => void reviewSocialFollowRequest(decision, requestID, idempotencyKey)}
      search={socialSearch}
      searchQuery={(query) => void searchSocialProfiles(query)}
      sessionKey={socialPresentationKey}
    /> : null}
    {ready && ownedHomeDestination && socialFeedActivity ? <SocialFeedActivityDetailRouteSource
      busy={socialFeedActivity.busy}
      detail={socialFeedActivity.detail}
      errorKey={socialFeedActivity.errorKey}
      event={socialFeedActivity.event}
      loadMoreRevisions={() => void loadMoreSocialFeedActivityRevisions()}
      nextCursor={socialFeedActivity.nextCursor}
      profileID={ownedHomeDestination.profile.id}
      retryRevisions={() => void loadMoreSocialFeedActivityRevisions()}
      revisions={socialFeedActivity.revisions}
      routeKey={socialFeedActivityDetailRouteKey(
        socialFeedActivity.event.path.id,
        socialFeedActivity.event.activity.id,
      )}
    /> : null}
    {ready && ownedHomeDestination && notificationsOpen ? <NotificationRouteSource
      busy={notificationsBusy}
      canMarkAllRead={notificationHistory.items.some((item) => !item.read)}
      invitationCount={ownedHomeDestination.profile.pendingInvitations.nextCursor
        ? undefined
        : ownedHomeDestination.profile.pendingInvitations.items.length}
      markAllRead={() => void mutateNotification({ kind: 'read-all' })}
      onDismiss={closeNotificationsRoute}
      onUnavailable={() => {
        closeNotificationsRoute();
        router.replace('/(tabs)/home');
      }}
      openInvitations={() => void openPendingInvitations()}
      refresh={() => void refreshNotifications()}
      refreshing={notificationsRefreshing}
    >
      <NotificationHistoryView
        busy={notificationsBusy}
        canOpen={(notification) => notification.type === 'path_invitation_received'
          ? true
          : (notification.type === 'practice_comment' || notification.type === 'comment_heart')
            ? true
          : notification.type === 'path_deleted' || notification.type === 'path_member_removed'
            ? false
            : notification.type === 'follow_request_received'
              || notification.type === 'new_follower'
              || notification.type === 'follow_request_accepted'
              ? true
            : ownedHomeDestination.profile.paths.some((path) => path.id === notification.pathId)
              || ownedHomeDestination.profile.archivedPaths.some((path) => path.id === notification.pathId)}
        errorKey={notificationsErrorKey ?? undefined}
        history={notificationHistory}
        i18n={i18n}
        onDelete={(notificationID) => void mutateNotification({ kind: 'delete', notificationId: notificationID })}
        onLoadMore={() => {
          if (notificationHistory.nextCursor) void loadMoreNotifications(notificationHistory.nextCursor);
        }}
        onOpen={(notification) => void openNotification(notification)}
        onRetry={() => void refreshNotifications()}
      />
    </NotificationRouteSource> : null}
    {ready && ownedHomeDestination && invitationsOpen ? <InvitationRouteSource
      onDismiss={closeInvitationsRoute}
      onUnavailable={() => {
        closeInvitationsRoute();
        router.replace('/(tabs)/home');
      }}
    >
      <PendingInvitationsView
        acceptedRole={acceptedInvitation?.role}
        busy={pendingInvitationsBusy}
        errorKey={pendingInvitationsErrorKey ?? undefined}
        focusedInvitationID={focusedInvitationID ?? undefined}
        invitationBusy={pendingInvitationBusy}
        invitationErrors={pendingInvitationErrors}
        invitationRejecting={pendingInvitationRejecting}
        invitations={ownedHomeDestination.profile.pendingInvitations}
        i18n={i18n}
        onAccept={beginPendingInvitationAcceptance}
        onReject={beginPendingInvitationRejection}
        onLoadMore={() => {
          if (ownedHomeDestination.profile.pendingInvitations.nextCursor) {
            void loadMorePendingInvitations(ownedHomeDestination.profile.pendingInvitations.nextCursor);
          }
        }}
        onRetry={() => void openPendingInvitations(focusedInvitationID ?? undefined, false)}
        rejected={rejectedInvitationID !== null}
      />
      {pendingInvitationAcceptanceReview ? <PathInvitationVisibilityWarningSheet
          busy={Boolean(pendingInvitationBusy[pendingInvitationAcceptanceReview.invitationId])}
          onCancel={cancelPendingInvitationAcceptance}
          onConfirm={() => void submitPendingInvitationAcceptance(pendingInvitationAcceptanceReview)}
          review={pendingInvitationAcceptanceReview}
          translator={i18n}
        /> : null}
    </InvitationRouteSource> : null}
    {ready && destination?.kind === 'onboarding' ? <OnboardingForm
      busy={activatingOnboarding || onboardingHomeRecovery !== null}
      canSubmit={canCompleteMobileOnboarding(destination.profile) && errorKey !== 'onboarding.usernameUnavailable'}
      errorText={!onboardingHomeRecovery && errorKey && errorKey !== 'onboarding.usernameUnavailable' ? i18n.t(errorKey) : undefined}
      homeRecoveryStatus={onboardingHomeRecovery?.status}
      onChangeDisplayName={updateOnboardingDisplayName}
      onChangeUsername={updateOnboardingUsername}
      onConfirm={() => void completeOnboarding()}
      onOpenPolicy={(url) => void openPolicyLink(url)}
      onReviewUsername={reviewOnboardingUsername}
      onRetryHome={() => void retryOnboardingHome()}
      onSignOut={() => void clearSession()}
      onUpdate={updateOnboardingDraft}
      profile={destination.profile}
      usernameReviewed={destination.profile.usernameReviewed}
      usernameUnavailable={errorKey === 'onboarding.usernameUnavailable'}
    /> : null}
    {ready && destination?.kind === 'duplicate_email_recovery' ? <DuplicateEmailRecoveryScreen
      declining={decliningRecovery}
      errorText={errorKey ? i18n.t(errorKey) : undefined}
      onContinue={() => void continueCreatingNewAccount()}
      onReturnToSignIn={() => void clearSession()}
    /> : null}
    {ready && !destination && accessState !== 'authenticated_offline' ? <SignedOutScreen
      errorText={errorKey ? i18n.t(errorKey) : undefined}
      onSignIn={() => void beginSignIn()}
      onRetry={providerSignIn.retry}
      providerBusy={providerSignIn.busy}
      providerDiscoveryFailed={providerSignIn.discoveryFailed}
      providerReady={providerSignIn.ready}
      sessionExpired={errorKey === 'errors.sessionExpired'}
    /> : null}
  </SafeAreaView>;
}
