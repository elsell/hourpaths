import { NativeButton } from './native-button';
import type { PathMemberRemovalReview } from '@hourpaths/client-core';
import type { NudgeEligibility } from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { formatCompactDuration } from './compact-duration';
import { ActivityHistoryView } from './activity-history-view';
import type { ActivityDetail } from '@hourpaths/api-client';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeChoicePicker } from './native-choice-picker';
import { SettingsActionRow, SettingsSection, SettingsSeparator, SettingsValueRow } from './settings-list';
import { ThemedText as Text } from './primitives';
import { ProgressIndicator } from './progress-indicator';
import { mobileTheme } from './tokens';
import { nudgeActionState } from '../nudge-presentation';

export type PathMemberGoalProgress = Readonly<{
  accumulatedSeconds: number;
  targetSeconds: number;
}>;

export type PathMemberSummary = Readonly<{
  blockedByViewer: boolean;
  canGrantAdministrator: boolean;
  canLeave: boolean;
  canChangeRole: boolean;
  canRevokeAdministrator: boolean;
  canRemove: boolean;
  canStepDownAdministrator: boolean;
  displayName: string;
  isViewer: boolean;
  nudgeEligibility?: NudgeEligibility;
  intervalProgress?: PathMemberGoalProgress;
  overallProgress?: PathMemberGoalProgress;
  pathId: string;
  role: 'creator' | 'administrator' | 'participant' | 'supporter';
  sessionCount: number;
  totalTrackedSeconds: number;
  userId: string;
  username: string;
}>;

export type PathMemberListState = Readonly<{
  error: boolean;
  items: readonly PathMemberSummary[];
  loading: boolean;
  loadingMore: boolean;
  nextCursor?: string;
}>;

const roleLabels = {
  administrator: 'pathMembers.administrators',
  creator: 'pathMembers.creator',
  participant: 'pathMembers.participant',
  supporter: 'pathMembers.supporter',
} as const satisfies Record<PathMemberSummary['role'], MessageKey>;

function PathMemberProgressRow({ i18n, inset = true, label, progress }: {
  i18n: Translator;
  inset?: boolean;
  label: string;
  progress: PathMemberGoalProgress;
}) {
  return <View style={[styles.progressRow, inset ? null : styles.progressRowCompact]}>
    <ProgressIndicator
      accessibilityLabel={label}
      compact
      targetValue={progress.targetSeconds}
      text={pathMemberProgressSummary(i18n, label, progress)}
      visualValue={Math.min(progress.accumulatedSeconds, progress.targetSeconds)}
    />
  </View>;
}

function pathMemberProgressSummary(i18n: Translator, label: string, progress: PathMemberGoalProgress): string {
  return i18n.t('pathMembers.progressSummary', {
    accumulated: formatCompactDuration(progress.accumulatedSeconds, i18n),
    label,
    target: formatCompactDuration(progress.targetSeconds, i18n),
  });
}

function MemberRow({ i18n, member, onOpen }: {
  i18n: Translator;
  member: PathMemberSummary;
  onOpen: () => void;
}) {
  const intervalLabel = i18n.t('pathMembers.intervalProgressLabel');
  const overallLabel = i18n.t('pathMembers.overallProgressLabel');
  const accessibilityLabel = [
    i18n.t('pathMembers.identity', { displayName: member.displayName, username: member.username }),
    i18n.t(roleLabels[member.role]),
    member.intervalProgress ? pathMemberProgressSummary(i18n, intervalLabel, member.intervalProgress) : null,
    member.overallProgress ? pathMemberProgressSummary(i18n, overallLabel, member.overallProgress) : null,
  ].filter((value): value is string => value !== null).join('. ');
  const content = <>
    <View style={styles.identity}>
      <Text style={styles.displayName}>{member.displayName}</Text>
      <Text style={styles.username}>@{member.username}</Text>
      {member.intervalProgress || member.overallProgress ? <View importantForAccessibility="no-hide-descendants" style={styles.rowProgress}>
        {member.intervalProgress ? <PathMemberProgressRow i18n={i18n} inset={false} label={intervalLabel} progress={member.intervalProgress} /> : null}
        {member.overallProgress ? <PathMemberProgressRow i18n={i18n} inset={false} label={overallLabel} progress={member.overallProgress} /> : null}
      </View> : null}
    </View>
    <Text style={styles.role}>{i18n.t(roleLabels[member.role])}</Text>
    <Text accessibilityElementsHidden style={styles.chevron}>›</Text>
  </>;
  return <Pressable
    accessibilityLabel={accessibilityLabel}
    accessibilityRole="button"
    onPress={onOpen}
    style={({ pressed }) => [styles.row, pressed ? styles.pressed : null]}
  >{content}</Pressable>;
}

function ListState({ i18n, state }: { i18n: Translator; state: PathMemberListState }) {
  if (state.loading) return <View accessibilityRole="progressbar" style={styles.centered}>
    <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
    <Text style={styles.secondary}>{i18n.t('pathMembers.loading')}</Text>
  </View>;
  return <NativeContentUnavailable
    description={i18n.t(state.error ? 'pathMembers.unavailableDescription' : 'pathMembers.emptyDescription')}
    systemImage={state.error ? 'wifi.exclamationmark' : 'person.2'}
    title={i18n.t(state.error ? 'pathMembers.unavailableHeading' : 'pathMembers.emptyHeading')}
  />;
}

export function PathMemberManagementView({
  i18n,
  onLoadMore,
  onOpen,
  onRetry,
  state,
}: {
  i18n: Translator;
  onLoadMore: () => void;
  onOpen: (member: PathMemberSummary) => void;
  onRetry: () => void;
  state: PathMemberListState;
}) {
  if (state.items.length === 0) return <View style={styles.state}>
    <ListState i18n={i18n} state={state} />
    {state.error ? <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" /> : null}
  </View>;

  return <View style={styles.group}>
    {state.items.map((member, index) => <View key={member.userId}>
      {index > 0 ? <View style={styles.separator} /> : null}
      <MemberRow i18n={i18n} member={member} onOpen={() => onOpen(member)} />
    </View>)}
    {state.nextCursor ? <>
      <View style={styles.separator} />
      <NativeButton busy={state.loadingMore} disabled={state.loadingMore} label={i18n.t(state.loadingMore ? 'common.loading' : 'common.loadMore')} onPress={onLoadMore} variant="quiet" />
    </> : null}
    {state.error ? <View accessibilityRole="alert" style={styles.partialError}>
      <Text style={styles.error}>{i18n.t('pathMembers.unavailableDescription')}</Text>
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : null}
  </View>;
}

export function PathMemberRemovalReviewView({
  busy,
  activities,
  activitiesBusy,
  activitiesErrorText,
  activitiesHaveMore,
  errorText,
  i18n,
  loading,
  member,
  onLoadMoreActivities,
  onCancelRoleChange,
  onChooseRole,
  onConfirmRoleChange,
  onOpenActivity,
  onRemove,
  onRetry,
  onSendNudge,
  onUnblock,
  review,
  roleChangeBusy,
  roleChangeErrorText,
  nudgeConfirmationText,
  pendingRole,
  unblockBusy,
  unblockErrorText,
}: {
  activities: readonly ActivityDetail[];
  activitiesBusy: boolean;
  activitiesErrorText?: string;
  activitiesHaveMore: boolean;
  busy: boolean;
  errorText?: string;
  i18n: Translator;
  loading: boolean;
  member: PathMemberSummary;
  onLoadMoreActivities: () => void;
  onCancelRoleChange: () => void;
  onChooseRole: (role: 'administrator' | 'participant' | 'supporter') => void;
  onConfirmRoleChange: () => void;
  onOpenActivity: (activityID: string) => void;
  onRemove: () => void;
  onRetry: () => void;
  onSendNudge: () => void;
  onUnblock: () => void;
  review: PathMemberRemovalReview | null;
  roleChangeBusy: boolean;
  roleChangeErrorText?: string;
  nudgeConfirmationText?: string;
  pendingRole: 'administrator' | 'participant' | 'supporter' | null;
  unblockBusy: boolean;
  unblockErrorText?: string;
}) {
  const participant = review?.role === 'participant';
  const nudgeAction = nudgeActionState(member);
  const canGrantAdministrator = member.role === 'participant' && member.canGrantAdministrator;
  const canRevokeAdministrator = member.role === 'administrator' && !member.isViewer && member.canRevokeAdministrator;
  const canStepDownAdministrator = member.role === 'administrator' && member.isViewer && member.canStepDownAdministrator;
  const administratorConfirmation = pendingRole === 'administrator'
    ? {
      confirmKey: 'pathMembers.confirmGrantAdministrator' as const,
      tone: 'default' as const,
      warningKey: 'pathMembers.grantAdministratorWarning' as const,
    }
    : pendingRole === 'participant' && member.role === 'administrator'
      ? canStepDownAdministrator
        ? {
          confirmKey: 'pathMembers.confirmStepDownAdministrator' as const,
          tone: 'destructive' as const,
          warningKey: 'pathMembers.stepDownAdministratorWarning' as const,
        }
        : {
          confirmKey: 'pathMembers.confirmRevokeAdministrator' as const,
          tone: 'destructive' as const,
          warningKey: 'pathMembers.revokeAdministratorWarning' as const,
        }
      : null;
  return <View style={styles.review}>
    <SettingsSection>
      <SettingsValueRow label={i18n.t('pathMembers.personLabel')} value={i18n.t('pathMembers.identity', {
        displayName: member.displayName,
        username: member.username,
      })} />
      <SettingsSeparator />
      <SettingsValueRow label={i18n.t('pathMembers.roleLabel')} value={i18n.t(roleLabels[member.role])} />
      <SettingsSeparator />
      <SettingsValueRow label={i18n.t('pathMembers.sessionsLabel')} value={i18n.number(member.sessionCount)} />
      <SettingsSeparator />
      <SettingsValueRow label={i18n.t('pathMembers.totalTimeLabel')} value={formatCompactDuration(member.totalTrackedSeconds, i18n)} />
      {member.intervalProgress ? <><SettingsSeparator />
        <PathMemberProgressRow i18n={i18n} label={i18n.t('pathMembers.intervalProgressLabel')} progress={member.intervalProgress} /></> : null}
      {member.overallProgress ? <><SettingsSeparator />
        <PathMemberProgressRow i18n={i18n} label={i18n.t('pathMembers.overallProgressLabel')} progress={member.overallProgress} /></> : null}
      {review ? <><SettingsSeparator />
        <SettingsValueRow label={i18n.t('pathMembers.timerLabel')} value={i18n.t(review.runningTimer ? 'pathMembers.timerRunning' : 'pathMembers.timerNotRunning')} /></> : null}
    </SettingsSection>
    {nudgeAction.kind !== 'hidden' ? <SettingsSection title={i18n.t('nudge.section')}>
      {nudgeAction.kind === 'send' ? <SettingsActionRow
        accessibilityLabel={i18n.t('nudge.sendAction')}
        disabled={busy || unblockBusy || roleChangeBusy}
        label={i18n.t('nudge.sendAction')}
        onPress={onSendNudge}
        tone="default"
      /> : <SettingsValueRow
        label={i18n.t('nudge.sendAction')}
        value={i18n.t(nudgeAction.kind === 'goal_complete'
          ? 'nudge.eligibility.goalComplete'
          : 'nudge.eligibility.rateLimited')}
      />}
    </SettingsSection> : null}
    {nudgeConfirmationText ? <Text accessibilityLiveRegion="polite" style={styles.confirmation}>
      {nudgeConfirmationText}
    </Text> : null}
    {member.canChangeRole && review ? <>
      <SettingsSection>
        <View style={styles.rolePicker}>
          <NativeChoicePicker
            accessibilityLabel={i18n.t('pathMembers.changeRole')}
            choices={[
              { label: i18n.t('pathMembers.participant'), value: 'participant' },
              { label: i18n.t('pathMembers.supporter'), value: 'supporter' },
            ]}
            disabled={busy || unblockBusy || roleChangeBusy || pendingRole !== null}
            label={i18n.t('pathMembers.changeRole')}
            onChange={onChooseRole}
            value={member.role === 'supporter' ? 'supporter' : 'participant'}
          />
          <Text style={styles.secondary}>{i18n.t(member.role === 'supporter'
            ? 'pathMembers.roleSupporterEffect'
            : 'pathMembers.roleParticipantEffect')}</Text>
        </View>
      </SettingsSection>
      {pendingRole === 'supporter' ? <View accessibilityRole="alert" style={styles.roleWarning}>
        <Text style={styles.warningTitle}>{i18n.t('pathMembers.confirmSupporter')}</Text>
        <Text style={styles.warningText}>{i18n.t('pathMembers.roleChangeWarning', { displayName: member.displayName })}</Text>
        <Text style={styles.warningText}>{i18n.t('pathMembers.roleChangeTimerWarning')}</Text>
        <SettingsSection>
          <SettingsActionRow
            accessibilityLabel={i18n.t('pathMembers.confirmSupporter')}
            disabled={roleChangeBusy}
            label={i18n.t(roleChangeBusy ? 'pathMembers.changingRole' : 'pathMembers.confirmSupporter')}
            onPress={onConfirmRoleChange}
            tone="destructive"
          />
          <SettingsSeparator />
          <SettingsActionRow
            accessibilityLabel={i18n.t('common.cancel')}
            disabled={roleChangeBusy}
            label={i18n.t('common.cancel')}
            onPress={onCancelRoleChange}
          />
        </SettingsSection>
      </View> : null}
    </> : null}
    {canGrantAdministrator || canRevokeAdministrator || canStepDownAdministrator ? <SettingsSection>
      {canGrantAdministrator ? <SettingsActionRow
        accessibilityLabel={i18n.t('pathMembers.confirmGrantAdministrator')}
        disabled={busy || unblockBusy || roleChangeBusy || pendingRole !== null}
        label={i18n.t('pathMembers.confirmGrantAdministrator')}
        onPress={() => onChooseRole('administrator')}
      /> : null}
      {canRevokeAdministrator ? <SettingsActionRow
        accessibilityLabel={i18n.t('pathMembers.confirmRevokeAdministrator')}
        disabled={busy || unblockBusy || roleChangeBusy || pendingRole !== null}
        label={i18n.t('pathMembers.confirmRevokeAdministrator')}
        onPress={() => onChooseRole('participant')}
        tone="destructive"
      /> : null}
      {canStepDownAdministrator ? <SettingsActionRow
        accessibilityLabel={i18n.t('pathMembers.confirmStepDownAdministrator')}
        disabled={busy || unblockBusy || roleChangeBusy || pendingRole !== null}
        label={i18n.t('pathMembers.confirmStepDownAdministrator')}
        onPress={() => onChooseRole('participant')}
        tone="destructive"
      /> : null}
    </SettingsSection> : null}
    {administratorConfirmation ? <View accessibilityRole="alert" style={styles.roleWarning}>
      <Text style={styles.warningTitle}>{i18n.t(administratorConfirmation.confirmKey)}</Text>
      <Text style={styles.warningText}>{i18n.t(administratorConfirmation.warningKey, { displayName: member.displayName })}</Text>
      {pendingRole === 'administrator' ? <Text style={styles.secondary}>{i18n.t('pathMembers.roleAdministratorEffect')}</Text> : null}
      <SettingsSection>
        <SettingsActionRow
          accessibilityLabel={i18n.t(administratorConfirmation.confirmKey)}
          disabled={roleChangeBusy}
          label={i18n.t(roleChangeBusy ? 'pathMembers.changingRole' : administratorConfirmation.confirmKey)}
          onPress={onConfirmRoleChange}
          tone={administratorConfirmation.tone}
        />
        <SettingsSeparator />
        <SettingsActionRow
          accessibilityLabel={i18n.t('common.cancel')}
          disabled={roleChangeBusy}
          label={i18n.t('common.cancel')}
          onPress={onCancelRoleChange}
        />
      </SettingsSection>
    </View> : null}
    {member.canRemove && loading ? <View accessibilityRole="progressbar" style={styles.centered}>
      <ActivityIndicator color={mobileTheme.colors.accent} />
      <Text style={styles.secondary}>{i18n.t('pathMembers.reviewLoading')}</Text>
    </View> : member.canRemove && !review ? <View style={styles.state}>
      <NativeContentUnavailable
        description={i18n.t('pathMembers.reviewUnavailableDescription')}
        systemImage="wifi.exclamationmark"
        title={i18n.t('pathMembers.reviewUnavailableHeading')}
      />
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : review ? <><View accessibilityRole="alert" style={styles.warning}>
      {participant ? <>
        <Text style={styles.warningText}>{i18n.t('pathMembers.participantActivityWarning')}</Text>
        <Text style={styles.warningText}>{i18n.t('pathMembers.participantSocialWarning')}</Text>
        <Text style={styles.warningText}>{i18n.t('pathMembers.offlineWarning')}</Text>
        <Text style={styles.warningText}>{i18n.t('pathMembers.runningTimerWarning')}</Text>
        <Text style={styles.warningText}>{i18n.t('pathMembers.reinviteWarning')}</Text>
      </> : <Text style={styles.warningText}>{i18n.t('pathMembers.supporterWarning')}</Text>}
    </View><SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t(participant ? 'pathMembers.removeParticipantAndData' : 'pathMembers.removeSupporter')}
        disabled={busy || unblockBusy}
        label={i18n.t(busy ? 'pathMembers.removing' : participant ? 'pathMembers.removeParticipantAndData' : 'pathMembers.removeSupporter')}
        onPress={onRemove}
      />
    </SettingsSection></> : null}
    {member.blockedByViewer ? <SettingsSection>
      {member.blockedByViewer ? <SettingsActionRow
        accessibilityLabel={i18n.t('blocking.unblockLabel', { username: member.username })}
        disabled={busy || unblockBusy}
        label={i18n.t(unblockBusy ? 'common.loading' : 'blocking.unblock')}
        onPress={onUnblock}
        tone="default"
      /> : null}
    </SettingsSection> : null}
    <ActivityHistoryView
      activities={activities}
      busy={activitiesBusy}
      errorText={activitiesErrorText}
      hasMore={activitiesHaveMore}
      hideParticipant
      onLoadMore={onLoadMoreActivities}
      onOpenActivity={onOpenActivity}
      onRetry={onLoadMoreActivities}
    />
    {errorText ? <Text accessibilityRole="alert" style={styles.error}>{errorText}</Text> : null}
    {unblockErrorText ? <Text accessibilityRole="alert" style={styles.error}>{unblockErrorText}</Text> : null}
    {roleChangeErrorText ? <Text accessibilityRole="alert" style={styles.error}>{roleChangeErrorText}</Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  centered: { alignItems: 'center', gap: mobileTheme.spacing.sm, minHeight: 260, justifyContent: 'center' },
  confirmation: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20, textAlign: 'center' },
  chevron: { color: mobileTheme.colors.textMuted, fontSize: 28, lineHeight: 28 },
  displayName: { fontSize: 17, fontWeight: '600', lineHeight: 22 },
  error: { color: mobileTheme.colors.error, textAlign: 'center' },
  group: { backgroundColor: mobileTheme.colors.surface, borderRadius: mobileTheme.radii.md, overflow: 'hidden' },
  identity: { flex: 1, minWidth: 0 },
  partialError: { gap: mobileTheme.spacing.xs, padding: mobileTheme.spacing.md },
  pressed: { backgroundColor: mobileTheme.colors.surfaceRaised },
  progressRow: { paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  progressRowCompact: { paddingHorizontal: 0, paddingVertical: 0 },
  review: { gap: mobileTheme.spacing.lg },
  rolePicker: { gap: mobileTheme.spacing.xs, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.xs },
  roleWarning: { backgroundColor: mobileTheme.colors.surface, borderRadius: mobileTheme.radii.md, gap: mobileTheme.spacing.sm, padding: mobileTheme.spacing.md },
  role: { color: mobileTheme.colors.textMuted, flexShrink: 1, fontSize: 15, lineHeight: 20, textAlign: 'right' },
  row: { alignItems: 'flex-start', flexDirection: 'row', gap: mobileTheme.spacing.sm, minHeight: 60, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  rowProgress: { gap: mobileTheme.spacing.xxs, marginTop: mobileTheme.spacing.xs },
  secondary: { color: mobileTheme.colors.textMuted },
  separator: { backgroundColor: mobileTheme.colors.border, height: StyleSheet.hairlineWidth, marginLeft: mobileTheme.spacing.md },
  state: { gap: mobileTheme.spacing.sm },
  username: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20 },
  warning: { backgroundColor: mobileTheme.colors.surface, borderRadius: mobileTheme.radii.md, gap: mobileTheme.spacing.sm, padding: mobileTheme.spacing.md },
  warningText: { color: mobileTheme.colors.text, fontSize: 15, lineHeight: 21 },
  warningTitle: { color: mobileTheme.colors.text, fontSize: 17, fontWeight: '600', lineHeight: 22 },
});
