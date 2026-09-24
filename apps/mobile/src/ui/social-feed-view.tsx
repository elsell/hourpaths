import type { Translator } from '@hourpaths/i18n';
import { activeTimerSeconds } from '@hourpaths/client-core';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { formatCompactDuration } from './compact-duration';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ThemedText as Text } from './primitives';
import {
  optimisticSocialReactionSummary,
  type GoalAchievementFeedEvent,
  type PracticeSessionFeedEvent,
  type SocialFeedEvent,
  type SocialReactionSummary,
} from './social-feed-presentation';
import type { ActiveFollowingItem } from './social-active-following-presentation';
import type { ActiveFollowingState, SocialFeedState } from './social-feed-route-presentation';
import { SocialProfileAvatar } from './social-profile-avatar';
import { SettingsIcon } from './settings-icon';
import { SocialReactionMenu } from './social-reaction-menu';
import {
  SOCIAL_REACTIONS,
  socialReactionDefinition,
  type SocialReaction,
  visibleReactionCounts,
} from './social-reaction-presentation';
import { mobileTheme } from './tokens';

function ActiveFollowingRow({
  i18n,
  item,
  now,
}: {
  i18n: Translator;
  item: ActiveFollowingItem;
  now: number;
}) {
  const timerLabels = item.timers.map((timer) => i18n.t('social.activeTimer', {
    duration: formatCompactDuration(activeTimerSeconds(timer.startedAt, now), i18n),
    path: timer.path.name,
  }));

  return <View
    accessible
    accessibilityLabel={i18n.t('social.activeGroupAccessibility', {
      participant: item.participant.displayName,
      timers: timerLabels.join(', '),
    })}
    style={styles.activeRow}
  >
    <SocialProfileAvatar
      accessibilityLabel={i18n.t('social.neutralAvatarLabel')}
      size={40}
    />
    <View style={styles.copy}>
      <Text style={styles.summary}>{item.participant.displayName}</Text>
      {timerLabels.map((label, index) => <Text key={item.timers[index]?.id} style={styles.activeTimer}>
        {label}
      </Text>)}
    </View>
  </View>;
}

function ActiveFollowingSection({
  i18n,
  onLoadMore,
  onRetry,
  state,
}: {
  i18n: Translator;
  onLoadMore: () => void;
  onRetry: () => void;
  state: ActiveFollowingState;
}) {
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    if (state.items.length === 0) return;
    setNow(Date.now());
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const tick = () => {
      if (!active) return;
      setNow(Date.now());
      timer = setTimeout(tick, 1_000);
    };
    timer = setTimeout(tick, 1_000);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [state.items.length]);

  return <View style={styles.activeSection}>
    <Text accessibilityRole="header" style={styles.sectionHeading}>{i18n.t('social.activeHeading')}</Text>
    {state.items.length > 0 ? <View style={styles.activeList}>
      {state.items.map((item, index) => <View key={item.participant.userId}>
        {index > 0 ? <View style={styles.separator} /> : null}
        <ActiveFollowingRow i18n={i18n} item={item} now={now} />
      </View>)}
    </View> : state.status === 'loading' || state.status === 'idle'
      ? <View accessibilityLabel={i18n.t('social.activeLoading')} accessibilityRole="progressbar" style={styles.activeState}>
          <ActivityIndicator color={mobileTheme.colors.accent} size="small" />
          <Text style={styles.secondary}>{i18n.t('social.activeLoading')}</Text>
        </View>
      : state.status === 'error'
        ? <View accessibilityRole="alert" style={styles.activeState}>
            <Text style={styles.error}>{i18n.t(state.errorKey ?? 'social.activeUnavailable')}</Text>
            <Pressable accessibilityRole="button" onPress={onRetry} style={styles.compactAction}>
              <Text style={styles.actionLabel}>{i18n.t('common.retry')}</Text>
            </Pressable>
          </View>
        : <Text style={styles.activeEmpty}>{i18n.t('social.activeEmpty')}</Text>}
    {state.nextCursor && state.status !== 'error' ? <Pressable
      accessibilityRole="button"
      accessibilityState={{ busy: state.loadingMore, disabled: state.loadingMore }}
      disabled={state.loadingMore}
      onPress={onLoadMore}
      style={styles.compactAction}
    >
      {state.loadingMore ? <ActivityIndicator color={mobileTheme.colors.accent} size="small" /> : null}
      <Text style={styles.actionLabel}>{i18n.t(state.loadingMore
        ? 'social.activeLoadingMore'
        : 'social.activeLoadMore')}</Text>
    </Pressable> : null}
    {state.status === 'error' && state.items.length > 0 ? <View accessibilityRole="alert" style={styles.activeState}>
      <Text style={styles.error}>{i18n.t(state.errorKey ?? 'social.activeUnavailable')}</Text>
      <Pressable accessibilityRole="button" onPress={onRetry} style={styles.compactAction}>
        <Text style={styles.actionLabel}>{i18n.t('common.retry')}</Text>
      </Pressable>
    </View> : null}
  </View>;
}

function ReactionStrip({
  event,
  i18n,
  onRemoveReaction,
  onSetReaction,
}: {
  event: SocialFeedEvent;
  i18n: Translator;
  onRemoveReaction: (event: SocialFeedEvent) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, reaction: SocialReaction) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [optimistic, setOptimistic] = useState<SocialReactionSummary>();
  const busyRef = useRef(false);
  const projection = optimistic ?? event;
  const selectedDefinition = projection.viewerReaction
    ? socialReactionDefinition(projection.viewerReaction)
    : undefined;
  useEffect(() => {
    if (optimistic && event.viewerReaction === optimistic.viewerReaction) setOptimistic(undefined);
  }, [event.viewerReaction, optimistic]);

  const updateReaction = async (
    nextReaction: SocialReaction | null,
    operation: () => Promise<void>,
  ) => {
    if (busyRef.current) return;
    busyRef.current = true;
    setOptimistic(optimisticSocialReactionSummary(event, nextReaction));
    setBusy(true);
    setFailed(false);
    try {
      await operation();
    } catch {
      setOptimistic(undefined);
      setFailed(true);
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  };

  return <>
    <View style={styles.reactionStrip}>
      <View style={styles.reactionCounts}>
        {visibleReactionCounts(projection.reactions).map((reaction) => {
          const selected = reaction.type === projection.viewerReaction;
          return <View
            accessible
            accessibilityLabel={i18n.t('social.reactionCount', {
              count: reaction.count,
              reaction: i18n.t(reaction.labelKey),
            })}
            accessibilityState={{ selected }}
            key={reaction.type}
            style={[styles.reactionCount, selected ? styles.reactionCountSelected : null]}
          >
            <Text style={styles.reactionEmoji}>{reaction.emoji}</Text>
            <Text style={[styles.reactionNumber, selected ? styles.reactionNumberSelected : null]}>
              {i18n.number(reaction.count)}
            </Text>
          </View>;
        })}
      </View>
      <SocialReactionMenu
        accessibilityLabel={selectedDefinition
          ? i18n.t('social.reactionPickerSelected', { reaction: i18n.t(selectedDefinition.labelKey) })
          : i18n.t('social.reactionPicker', { participant: event.participant.displayName })}
        busy={busy}
        cancelLabel={i18n.t('common.cancel')}
        choices={SOCIAL_REACTIONS.map((reaction) => ({
          emoji: reaction.emoji,
          label: i18n.t(reaction.labelKey),
          menuLabel: i18n.t('social.reactionMenuChoice', {
            emoji: reaction.emoji,
            reaction: i18n.t(reaction.labelKey),
          }),
          onPress: () => { void updateReaction(
            reaction.type,
            () => onSetReaction(event, reaction.type),
          ); },
          selected: reaction.type === projection.viewerReaction,
          type: reaction.type,
        }))}
        onRemove={projection.viewerReaction
          ? () => { void updateReaction(null, () => onRemoveReaction(event)); }
          : undefined}
        removeLabel={i18n.t('social.reactionRemove')}
        selectedEmoji={selectedDefinition?.emoji}
      />
      {busy ? <Text
        accessibilityLiveRegion="polite"
        style={styles.reactionStatus}
      >{i18n.t('social.reactionUpdating')}</Text> : null}
    </View>
    {failed ? <Text
      accessibilityLiveRegion="assertive"
      accessibilityRole="alert"
      style={styles.reactionError}
    >{i18n.t('social.reactionUnavailable')}</Text> : null}
  </>;
}

function FeedEngagement({
  event,
  i18n,
  onOpenComments,
  onRemoveReaction,
  onSetReaction,
}: {
  event: SocialFeedEvent;
  i18n: Translator;
  onOpenComments: () => void;
  onRemoveReaction: (event: SocialFeedEvent) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, reaction: SocialReaction) => Promise<void>;
}) {
  return <>
    {event.commentsEnabled ? <Pressable
      accessibilityLabel={i18n.t('social.commentsOpen')}
      accessibilityRole="button"
      onPress={onOpenComments}
      style={({ pressed }) => [styles.commentsAction, pressed ? styles.rowPressed : null]}
    >
      <SettingsIcon systemName="bubble.left" />
      <Text style={styles.commentsActionLabel}>{i18n.t('social.commentsHeading')}</Text>
    </Pressable> : null}
    {event.reactionsEnabled ? <ReactionStrip
      event={event}
      i18n={i18n}
      onRemoveReaction={onRemoveReaction}
      onSetReaction={onSetReaction}
    /> : null}
  </>;
}

function AchievementFeedRow({
  event,
  i18n,
  onOpenComments,
  onRemoveReaction,
  onSetReaction,
}: {
  event: GoalAchievementFeedEvent;
  i18n: Translator;
  onOpenComments: () => void;
  onRemoveReaction: (event: SocialFeedEvent) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, reaction: SocialReaction) => Promise<void>;
}) {
  const target = formatCompactDuration(event.achievement.targetSeconds, i18n);
  const publishedAt = new Date(event.publishedAt);
  const date = i18n.date(publishedAt, { dateStyle: 'medium' });
  const time = i18n.time(publishedAt, { timeStyle: 'short' });
  const summaryKey = event.achievement.kind === 'interval'
    ? 'social.feedAchievementInterval'
    : 'social.feedAchievementOverall';
  const rowKey = event.achievement.kind === 'interval'
    ? 'social.feedAchievementRowInterval'
    : 'social.feedAchievementRowOverall';

  return <View>
    <View
      accessible
      accessibilityLabel={i18n.t(rowKey, {
        duration: target,
        participant: event.participant.displayName,
        path: event.path.name,
        time,
      })}
      style={styles.achievementRow}
    >
      <SocialProfileAvatar
        accessibilityLabel={i18n.t('social.neutralAvatarLabel')}
        profilePictureURL={event.participant.profilePictureURL}
        size={40}
      />
      <View style={styles.copy}>
        <Text style={styles.summary}>{i18n.t(summaryKey, {
          participant: event.participant.displayName,
          path: event.path.name,
        })}</Text>
        <View style={styles.metadata}>
          <Text style={styles.achievementTarget}>{i18n.t('social.feedAchievementTarget', { duration: target })}</Text>
          <Text style={styles.secondary}>{date}</Text>
          <Text style={styles.secondary}>{time}</Text>
        </View>
      </View>
      <SettingsIcon systemName="trophy.fill" />
    </View>
    <FeedEngagement
      event={event}
      i18n={i18n}
      onOpenComments={onOpenComments}
      onRemoveReaction={onRemoveReaction}
      onSetReaction={onSetReaction}
    />
  </View>;
}

function FeedRow({
  event,
  i18n,
  onOpen,
  onOpenComments,
  onRemoveReaction,
  onSetReaction,
}: {
  event: PracticeSessionFeedEvent;
  i18n: Translator;
  onOpen: () => void;
  onOpenComments: () => void;
  onRemoveReaction: (event: SocialFeedEvent) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, reaction: SocialReaction) => Promise<void>;
}) {
  const duration = formatCompactDuration(event.activity.durationSeconds, i18n);
  const publishedAt = new Date(event.publishedAt);
  const date = i18n.date(publishedAt, { dateStyle: 'medium' });
  const time = i18n.time(publishedAt, { timeStyle: 'short' });
  const labelKey = event.activity.edited ? 'social.feedRowEdited' : 'social.feedRow';

  return <View>
    <Pressable
      accessibilityLabel={i18n.t(labelKey, {
        duration,
        participant: event.participant.displayName,
        path: event.path.name,
        time,
      })}
      accessibilityRole="button"
      onPress={onOpen}
      style={({ pressed }) => [styles.row, pressed ? styles.rowPressed : null]}
    >
      <SocialProfileAvatar
        accessibilityLabel={i18n.t('social.neutralAvatarLabel')}
        size={40}
      />
      <View style={styles.copy}>
        <Text style={styles.summary}>
          {i18n.t('social.feedPracticeOnPath', {
            participant: event.participant.displayName,
            path: event.path.name,
          })}
        </Text>
        <View style={styles.metadata}>
          <Text style={styles.duration}>{duration}</Text>
          <Text style={styles.secondary}>{date}</Text>
          <Text style={styles.secondary}>{time}</Text>
          {event.activity.edited ? <Text style={styles.edited}>{i18n.t('social.edited')}</Text> : null}
        </View>
      </View>
      <SettingsIcon systemName="chevron.right" variant="disclosure" />
    </Pressable>
    <FeedEngagement
      event={event}
      i18n={i18n}
      onOpenComments={onOpenComments}
      onRemoveReaction={onRemoveReaction}
      onSetReaction={onSetReaction}
    />
  </View>;
}

function FeedUnavailable({
  i18n,
  onRetry,
  state,
}: {
  i18n: Translator;
  onRetry: () => void;
  state: SocialFeedState;
}) {
  if (state.status === 'loading' || state.status === 'idle') {
    return <View
      accessibilityLabel={i18n.t('common.loading')}
      accessibilityRole="progressbar"
      style={styles.centeredState}
    >
      <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
    </View>;
  }

  if (state.status === 'error') {
    return <View style={styles.unavailableState}>
      <NativeContentUnavailable
        description={i18n.t(state.errorKey ?? 'social.feedUnavailableDescription')}
        systemImage="wifi.exclamationmark"
        title={i18n.t('social.feedUnavailableHeading')}
      />
      <Pressable
        accessibilityLabel={i18n.t('common.retry')}
        accessibilityRole="button"
        onPress={onRetry}
        style={styles.unavailableAction}
      >
        <Text style={styles.actionLabel}>{i18n.t('common.retry')}</Text>
      </Pressable>
    </View>;
  }

  return <NativeContentUnavailable
    description={i18n.t('social.feedEmptyDescription')}
    systemImage="clock.arrow.circlepath"
    title={i18n.t('social.feedEmptyHeading')}
  />;
}

function InteractionNotice({
  i18n,
  messageKey,
  onDismiss,
}: {
  i18n: Translator;
  messageKey: NonNullable<SocialFeedState['interactionNoticeKey']>;
  onDismiss: () => void;
}) {
  return <View accessibilityRole="alert" style={styles.interactionNotice}>
    <Text style={styles.interactionNoticeCopy}>{i18n.t(messageKey)}</Text>
    <Pressable
      accessibilityLabel={i18n.t('social.interactionDisabledDismiss')}
      accessibilityRole="button"
      onPress={onDismiss}
      style={({ pressed }) => [styles.interactionNoticeDismiss, pressed ? styles.rowPressed : null]}
    >
      <SettingsIcon systemName="xmark" />
    </Pressable>
  </View>;
}

export function SocialFeedView({
  active,
  i18n,
  onLoadMoreActive,
  onLoadMore,
  onDismissInteractionNotice,
  onOpen,
  onOpenComments,
  onRemoveReaction,
  onRefresh,
  onRetry,
  onRetryActive,
  onSetReaction,
  state,
}: {
  active: ActiveFollowingState;
  i18n: Translator;
  onLoadMoreActive: () => void;
  onLoadMore: () => void;
  onDismissInteractionNotice: () => void;
  onOpen: (event: PracticeSessionFeedEvent) => void;
  onOpenComments: (event: SocialFeedEvent) => void;
  onRemoveReaction: (event: SocialFeedEvent) => Promise<void>;
  onRefresh: () => void;
  onRetry: () => void;
  onRetryActive: () => void;
  onSetReaction: (event: SocialFeedEvent, reaction: SocialReaction) => Promise<void>;
  state: SocialFeedState;
}) {
  const hasActive = active.items.length > 0;
  const hasEvents = state.items.length > 0;
  const noticeEventVisible = state.items.some(({ id }) => id === state.interactionNoticeEventID);
  const footer = state.status === 'error' && hasEvents
    ? <View accessibilityRole="alert" style={styles.footer}>
        <Text style={styles.error}>{i18n.t(state.errorKey ?? 'social.feedUnavailableDescription')}</Text>
        <Pressable accessibilityRole="button" onPress={onRetry} style={styles.footerAction}>
          <Text style={styles.actionLabel}>{i18n.t('common.retry')}</Text>
        </Pressable>
      </View>
    : state.nextCursor
      ? <Pressable
          accessibilityRole="button"
          accessibilityState={{ busy: state.loadingMore, disabled: state.loadingMore }}
          disabled={state.loadingMore}
          onPress={onLoadMore}
          style={styles.footerAction}
        >
          {state.loadingMore ? <ActivityIndicator color={mobileTheme.colors.accent} /> : null}
          <Text style={styles.actionLabel}>{i18n.t(state.loadingMore ? 'social.loadingMore' : 'social.loadMore')}</Text>
        </Pressable>
      : null;

  return <ScrollView
    alwaysBounceVertical
    automaticallyAdjustContentInsets
    contentContainerStyle={[styles.content, !hasActive && !hasEvents ? styles.emptyContent : null]}
    contentInsetAdjustmentBehavior="automatic"
    refreshControl={<RefreshControl
      colors={[mobileTheme.colors.accent]}
      onRefresh={onRefresh}
      refreshing={state.refreshing || active.refreshing}
      tintColor={mobileTheme.colors.accent}
    />}
    style={styles.screen}
  >
    <ActiveFollowingSection
      i18n={i18n}
      onLoadMore={onLoadMoreActive}
      onRetry={onRetryActive}
      state={active}
    />
    {state.interactionNoticeKey && !noticeEventVisible ? <InteractionNotice
      i18n={i18n}
      messageKey={state.interactionNoticeKey}
      onDismiss={onDismissInteractionNotice}
    /> : null}
    {hasEvents
      ? <View style={styles.list}>{state.items.map((event, index) => <View key={event.id}>
          {index > 0 ? <View style={styles.separator} /> : null}
          {state.interactionNoticeKey && state.interactionNoticeEventID === event.id
            ? <InteractionNotice
                i18n={i18n}
                messageKey={state.interactionNoticeKey}
                onDismiss={onDismissInteractionNotice}
              />
            : null}
          {event.type === 'goal_achievement'
            ? <AchievementFeedRow
                event={event}
                i18n={i18n}
                onOpenComments={() => onOpenComments(event)}
                onRemoveReaction={onRemoveReaction}
                onSetReaction={onSetReaction}
              />
            : <FeedRow
                event={event}
                i18n={i18n}
                onOpen={() => onOpen(event)}
                onOpenComments={() => onOpenComments(event)}
                onRemoveReaction={onRemoveReaction}
                onSetReaction={onSetReaction}
              />}
        </View>)}</View>
      : <FeedUnavailable i18n={i18n} onRetry={onRetry} state={state} />}
    {state.detailErrorKey ? <View accessibilityRole="alert" style={styles.footer}>
      <Text style={styles.error}>{i18n.t(state.detailErrorKey)}</Text>
    </View> : null}
    {footer}
  </ScrollView>;
}

const styles = StyleSheet.create({
  achievementRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  achievementTarget: {
    color: mobileTheme.colors.textMuted,
    fontVariant: ['tabular-nums'],
    ...mobileTheme.typography.caption,
  },
  interactionNotice: {
    alignItems: 'center',
    backgroundColor: mobileTheme.colors.surface,
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    padding: mobileTheme.spacing.md,
  },
  interactionNoticeCopy: {
    color: mobileTheme.colors.text,
    flex: 1,
    ...mobileTheme.typography.body,
  },
  interactionNoticeDismiss: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 44,
  },
  activeEmpty: {
    color: mobileTheme.colors.textMuted,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
    ...mobileTheme.typography.body,
  },
  activeList: {
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  activeRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  activeSection: {
    gap: mobileTheme.spacing.xs,
    marginHorizontal: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.lg,
    paddingTop: mobileTheme.spacing.sm,
  },
  activeState: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xs,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  activeTimer: {
    color: mobileTheme.colors.accent,
    fontVariant: ['tabular-nums'],
    ...mobileTheme.typography.caption,
  },
  actionLabel: {
    color: mobileTheme.colors.accent,
    fontSize: 17,
    fontWeight: '600',
  },
  centeredState: {
    alignItems: 'center',
    flex: 1,
    justifyContent: 'center',
    minHeight: 260,
  },
  content: {
    paddingBottom: mobileTheme.spacing.lg,
  },
  compactAction: {
    alignItems: 'center',
    alignSelf: 'flex-start',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xs,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  copy: {
    flex: 1,
    gap: mobileTheme.spacing.xxs,
    minWidth: 0,
  },
  commentsAction: {
    alignItems: 'center',
    alignSelf: 'flex-start',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xxs,
    minHeight: 44,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  commentsActionLabel: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  duration: {
    color: mobileTheme.colors.accent,
    fontVariant: ['tabular-nums'],
    ...mobileTheme.typography.caption,
  },
  edited: {
    color: mobileTheme.colors.accent,
    ...mobileTheme.typography.caption,
  },
  emptyContent: {
    flexGrow: 1,
    justifyContent: 'center',
  },
  error: {
    color: mobileTheme.colors.error,
    textAlign: 'center',
  },
  footer: {
    gap: mobileTheme.spacing.xs,
    padding: mobileTheme.spacing.md,
  },
  footerAction: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xs,
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  list: {
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  metadata: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: mobileTheme.spacing.xs,
  },
  row: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  rowPressed: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  unavailableAction: {
    alignItems: 'center',
    alignSelf: 'center',
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    minWidth: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  unavailableState: {
    alignItems: 'stretch',
  },
  reactionCount: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xxs,
    minHeight: 32,
    paddingHorizontal: mobileTheme.spacing.sm,
  },
  reactionCountSelected: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  reactionCounts: {
    flexDirection: 'row',
    flexGrow: 1,
    flexShrink: 1,
    flexWrap: 'wrap',
    gap: mobileTheme.spacing.xs,
  },
  reactionEmoji: { fontSize: 16 },
  reactionError: {
    color: mobileTheme.colors.error,
    paddingBottom: mobileTheme.spacing.sm,
    paddingHorizontal: mobileTheme.spacing.md,
    ...mobileTheme.typography.caption,
  },
  reactionNumber: {
    color: mobileTheme.colors.textMuted,
    fontVariant: ['tabular-nums'],
    ...mobileTheme.typography.caption,
  },
  reactionNumberSelected: { color: mobileTheme.colors.accent },
  reactionStatus: {
    color: mobileTheme.colors.textMuted,
    flexShrink: 1,
    ...mobileTheme.typography.caption,
  },
  reactionStrip: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: mobileTheme.spacing.xs,
    minHeight: 44,
    paddingBottom: mobileTheme.spacing.sm,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  screen: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
  },
  secondary: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  sectionHeading: {
    color: mobileTheme.colors.text,
    fontSize: 20,
    fontWeight: '700',
    lineHeight: 25,
  },
  separator: {
    backgroundColor: mobileTheme.colors.border,
    height: StyleSheet.hairlineWidth,
    marginLeft: 68,
  },
  summary: {
    color: mobileTheme.colors.text,
    fontSize: 16,
    fontWeight: '600',
    lineHeight: 21,
  },
});
