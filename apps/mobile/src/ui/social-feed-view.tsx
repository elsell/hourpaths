import { EmojiPicker } from './emoji-picker';
import { ReactionPeopleSheet, type ReactionPeopleLoader } from './reaction-people-sheet';
import { SegmentedAvatarRing } from './segmented-avatar-ring';
import { PathEmoji } from './path-emoji';
import type { Translator } from '@hourpaths/i18n';
import { activeTimerSeconds } from '@hourpaths/client-core';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { defaultPathAppearance, pathPalette } from './path-appearance';
import { formatGoalDuration, formatCompactDuration } from './compact-duration';
import { NativeButton } from './native-button';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ThemedText as Text } from './primitives';
import {
  type GoalAchievementFeedEvent,
  type PracticeSessionFeedEvent,
  type SocialFeedEvent,
} from './social-feed-presentation';
import type { ActiveFollowingItem } from './social-active-following-presentation';
import type { ActiveFollowingState, SocialFeedState } from './social-feed-route-presentation';
import { SocialProfileAvatar } from './social-profile-avatar';
import { SettingsIcon } from './settings-icon';
import {
  visibleReactionCounts,
} from './social-reaction-presentation';
import { mobileTheme } from './tokens';

function ActiveFollowingRow({ i18n, item, now, onOpen }: {
  i18n: Translator; item: ActiveFollowingItem; now: number; onOpen: () => void;
}) {
  const timerLabels = item.timers.map((timer) => i18n.t('social.activeTimer', {
    duration: formatGoalDuration(activeTimerSeconds(timer.startedAt, now), i18n), path: timer.path.name,
  }));
  return <Pressable accessibilityRole="button"
    accessibilityLabel={i18n.t('social.activeGroupAccessibility', { participant: item.participant.displayName, timers: timerLabels.join(', ') })}
    onPress={onOpen} style={({ pressed }) => [styles.story, pressed ? styles.pressed : null]}>
    <SegmentedAvatarRing count={item.timers.length}><SocialProfileAvatar accessibilityLabel={i18n.t('social.neutralAvatarLabel')} profilePictureURL={item.participant.profilePictureURL} size={56} /></SegmentedAvatarRing>
    <Text style={styles.storyName}>{item.participant.displayName}</Text>
  </Pressable>;
}

function ActiveFollowingSection({
  onOpen,
  i18n,
  onLoadMore,
  onRetry,
  state,
}: {
  i18n: Translator;
  onOpen: (item: ActiveFollowingItem) => void;
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
    {state.items.length > 0 ? <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.storyList}>
      {state.items.map((item) => <ActiveFollowingRow key={item.participant.userId} i18n={i18n} item={item} now={now} onOpen={() => onOpen(item)} />)}
    </ScrollView> : state.status === 'loading' || state.status === 'idle'
      ? <View accessibilityLabel={i18n.t('social.activeLoading')} accessibilityRole="progressbar" style={styles.activeState}>
          <ActivityIndicator color={mobileTheme.colors.accent} size="small" />
          <Text style={styles.secondary}>{i18n.t('social.activeLoading')}</Text>
        </View>
      : state.status === 'error'
        ? <View accessibilityRole="alert" style={styles.activeState}>
            <Text style={styles.error}>{i18n.t(state.errorKey ?? 'social.activeUnavailable')}</Text>
            <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
          </View>
        : <Text style={styles.activeEmpty}>{i18n.t('social.activeEmpty')}</Text>}
    {state.nextCursor && state.status !== 'error' ? <NativeButton busy={state.loadingMore} disabled={state.loadingMore} label={i18n.t(state.loadingMore ? 'social.activeLoadingMore' : 'social.activeLoadMore')} onPress={onLoadMore} variant="quiet" /> : null}
    {state.status === 'error' && state.items.length > 0 ? <View accessibilityRole="alert" style={styles.activeState}>
      <Text style={styles.error}>{i18n.t(state.errorKey ?? 'social.activeUnavailable')}</Text>
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : null}
  </View>;
}

function ReactionStrip({ event, i18n, onRemoveReaction, onSetReaction, loadReactionPeople, onOpenProfile }: {
  event: SocialFeedEvent; i18n: Translator;
  onRemoveReaction: (event: SocialFeedEvent, emoji?: string) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, emoji: string) => Promise<void>;
  loadReactionPeople: ReactionPeopleLoader;
  onOpenProfile: (username: string) => void;
}) {
  const [picker, setPicker] = useState(false);
  const [peopleEmoji, setPeopleEmoji] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const busyRef = useRef(false);
  const reactions = event.emojiReactions ?? visibleReactionCounts(event.reactions).map(value => ({ emoji: value.emoji, count: value.count, reacted: value.type === event.viewerReaction }));
  async function toggle(emoji: string) {
    if (busyRef.current) return;
    busyRef.current = true; setBusy(true); setFailed(false);
    try {
      if (reactions.some(value => value.emoji === emoji && value.reacted)) await onRemoveReaction(event, emoji);
      else await onSetReaction(event, emoji);
    } catch { setFailed(true); }
    finally { busyRef.current = false; setBusy(false); }
  }
  return <View style={styles.reactionStrip}>
    {reactions.map(reaction => <Pressable key={reaction.emoji} accessibilityRole="button"
      accessibilityLabel={i18n.t('social.reactionCount', { count: reaction.count, reaction: reaction.emoji })}
      accessibilityState={{ selected: reaction.reacted }} onPress={() => setPeopleEmoji(reaction.emoji)}
      style={[styles.reactionCount, reaction.reacted ? styles.reactionCountSelected : null]}>
      <Text style={styles.reactionEmoji}>{reaction.emoji}</Text><Text style={styles.reactionNumber}>{i18n.number(reaction.count)}</Text>
    </Pressable>)}
    <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('reactions.addAction')} disabled={busy}
      onPress={() => setPicker(true)} style={styles.reactionCount}>
      <SettingsIcon systemName="face.smiling" variant="inline" /><Text style={styles.reactionNumber}>+</Text>
    </Pressable>
    <EmojiPicker i18n={i18n} visible={picker} onClose={() => setPicker(false)} selectedEmojis={reactions.filter(value => value.reacted).map(value => value.emoji)}
      onSelect={emoji => { setPicker(false); void toggle(emoji); }} />
    {peopleEmoji ? <ReactionPeopleSheet key={peopleEmoji} eventId={event.id} emoji={peopleEmoji} i18n={i18n}
      loadPeople={loadReactionPeople} onClose={() => setPeopleEmoji(undefined)} onOpenProfile={onOpenProfile}
      reacted={reactions.some(value => value.emoji === peopleEmoji && value.reacted)} busy={busy}
      onToggle={() => { void toggle(peopleEmoji); }} /> : null}
    {failed ? <Text accessibilityRole="alert" style={styles.reactionError}>{i18n.t('social.reactionUnavailable')}</Text> : null}
  </View>;
}

function FeedEngagement({
  event,
  i18n,
  onOpenComments,
  loadReactionPeople,
  onOpenProfile,
  onRemoveReaction,
  onSetReaction,
}: {
  event: SocialFeedEvent;
  i18n: Translator;
  onOpenComments: () => void;
  loadReactionPeople: ReactionPeopleLoader;
  onOpenProfile: (username: string) => void;
  onRemoveReaction: (event: SocialFeedEvent, emoji?: string) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, reaction: string) => Promise<void>;
}) {
  return <View style={styles.engagement}>
    {event.commentsEnabled ? <Pressable
      accessibilityLabel={i18n.t('social.commentCount', { count: event.commentCount ?? 0 })}
      accessibilityRole="button"
      onPress={onOpenComments}
      style={({ pressed }) => [styles.commentsAction, pressed ? styles.rowPressed : null]}
    >
      <SettingsIcon systemName="bubble.left" variant="inline" />
      <Text style={styles.commentsActionLabel}>{i18n.t('social.commentCount', { count: event.commentCount ?? 0 })}</Text>
    </Pressable> : null}
    {event.reactionsEnabled ? <ReactionStrip
      event={event}
      i18n={i18n}
      onRemoveReaction={onRemoveReaction}
      onSetReaction={onSetReaction}
      loadReactionPeople={loadReactionPeople}
      onOpenProfile={onOpenProfile}
    /> : null}
  </View>;
}

export function SocialPost({ event, i18n, onOpen, onOpenProfile, onOpenComments, onRemoveReaction, onSetReaction, loadReactionPeople }: {
  event: SocialFeedEvent;
  i18n: Translator;
  onOpen: (event: PracticeSessionFeedEvent) => void;
  onOpenProfile: (username: string) => void;
  onOpenComments: (event: SocialFeedEvent) => void;
  loadReactionPeople: ReactionPeopleLoader;
  onRemoveReaction: (event: SocialFeedEvent, emoji?: string) => Promise<void>;
  onSetReaction: (event: SocialFeedEvent, reaction: string) => Promise<void>;
}) {
  const appearance = defaultPathAppearance(event.path.id);
  const tone = pathPalette[event.type === 'goal_achievement' ? 'gold' : appearance.color];
  const publishedAt = new Date(event.publishedAt);
  const date = i18n.date(publishedAt, { dateStyle: 'medium' });
  const time = i18n.time(publishedAt, { timeStyle: 'short' });
  const duration = formatGoalDuration(event.type === 'practice_session' ? event.activity.durationSeconds : event.achievement.targetSeconds, i18n);
  const summary = event.type === 'practice_session'
    ? i18n.t('social.post.practice', { path: event.path.name, duration })
    : i18n.t(event.achievement.kind === 'interval' ? 'social.post.intervalAchievement' : 'social.post.overallAchievement');
  return <View style={styles.post}>
    <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('social.post.openProfile', { name: event.participant.displayName })}
      onPress={() => onOpenProfile(event.participant.username)} style={({ pressed }) => [styles.author, pressed ? styles.pressed : null]}>
      <SocialProfileAvatar accessibilityLabel={i18n.t('social.neutralAvatarLabel')} profilePictureURL={event.participant.profilePictureURL} size={40} />
      <View style={styles.copy}>
        <Text style={styles.summary}>{event.participant.displayName}</Text>
        <Text style={styles.secondary}>{i18n.t('social.post.metadata', { username: event.participant.username, date, time })}</Text>
      </View>
    </Pressable>
    <Text style={styles.postSummary}>{summary}</Text>
    <Pressable accessibilityRole={event.type === 'practice_session' ? 'button' : undefined}
      accessibilityLabel={summary} disabled={event.type !== 'practice_session'}
      onPress={() => { if (event.type === 'practice_session') onOpen(event); }}
      style={({ pressed }) => [styles.attachment, { backgroundColor: tone.background }, pressed ? styles.pressed : null]}>
      <PathEmoji emoji={event.type === 'goal_achievement' ? '🏆' : appearance.emoji} size={44} />
      <View style={styles.copy}>
        <Text style={[styles.summary, { color: tone.foreground }]}>{event.path.name}</Text>
        <Text style={[styles.postDuration, { color: tone.foreground }]}>{duration}</Text>
        <Text style={[styles.secondary, { color: tone.foreground }]}>{i18n.t(event.type === 'practice_session' ? 'social.post.session' : 'social.post.goalCompleted')}</Text>
      </View>
    </Pressable>
    {event.type === 'practice_session' && event.activity.edited ? <Text style={styles.edited}>{i18n.t('social.edited')}</Text> : null}
    <FeedEngagement loadReactionPeople={loadReactionPeople} onOpenProfile={onOpenProfile} event={event} i18n={i18n} onOpenComments={() => onOpenComments(event)} onRemoveReaction={onRemoveReaction} onSetReaction={onSetReaction} />
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
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
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

export type SocialTimelineProps = {
  i18n: Translator;
  onLoadMore: () => void;
  onDismissInteractionNotice: () => void;
  onOpen: (event: PracticeSessionFeedEvent) => void;
  onOpenProfile: (username: string) => void;
  onOpenComments: (event: SocialFeedEvent) => void;
  loadReactionPeople: ReactionPeopleLoader;
  onRemoveReaction: (event: SocialFeedEvent, emoji?: string) => Promise<void>;
  onRetry: () => void;
  onSetReaction: (event: SocialFeedEvent, reaction: string) => Promise<void>;
  state: SocialFeedState;
  profile?: boolean;
};

export function SocialTimeline(props: SocialTimelineProps) {
  const { i18n, onLoadMore, onDismissInteractionNotice, onRetry, state, profile } = props;
  return <View>
    {state.interactionNoticeKey ? <InteractionNotice i18n={i18n} messageKey={state.interactionNoticeKey} onDismiss={onDismissInteractionNotice} /> : null}
    {state.items.length > 0 ? <View style={styles.list}>{state.items.map((event, index) => <View key={event.id}>
      {index > 0 ? <View style={styles.separator} /> : null}
      <SocialPost loadReactionPeople={props.loadReactionPeople} event={event} i18n={props.i18n} onOpen={props.onOpen} onOpenProfile={props.onOpenProfile} onOpenComments={props.onOpenComments} onRemoveReaction={props.onRemoveReaction} onSetReaction={props.onSetReaction} />
    </View>)}</View> : profile && state.status === 'ready' ? <Text style={styles.activeEmpty}>{i18n.t('social.profileActivityEmpty')}</Text> : <FeedUnavailable i18n={i18n} onRetry={onRetry} state={state} />}
    {state.detailErrorKey ? <Text accessibilityRole="alert" style={styles.error}>{i18n.t(state.detailErrorKey)}</Text> : null}
    {state.status === 'error' && state.items.length > 0 ? <View style={styles.footer}>
      <Text accessibilityRole="alert" style={styles.error}>{i18n.t(state.errorKey ?? 'social.feedUnavailableDescription')}</Text>
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : state.nextCursor ? <NativeButton busy={state.loadingMore} disabled={state.loadingMore} label={i18n.t(state.loadingMore ? 'social.loadingMore' : 'social.loadMore')} onPress={onLoadMore} variant="quiet" /> : null}
  </View>;
}

export function SocialFeedView({ active, onOpenActive, onLoadMoreActive, onRetryActive, onRefresh, ...props }: SocialTimelineProps & {
  active: ActiveFollowingState;
  onOpenActive: (item: ActiveFollowingItem) => void;
  onLoadMoreActive: () => void;
  onRetryActive: () => void;
  onRefresh: () => void;
}) {
  return <ScrollView alwaysBounceVertical automaticallyAdjustContentInsets contentContainerStyle={styles.content}
    contentInsetAdjustmentBehavior="automatic" style={styles.screen}
    refreshControl={<RefreshControl colors={[mobileTheme.colors.accent]} tintColor={mobileTheme.colors.accent} onRefresh={onRefresh} refreshing={props.state.refreshing || active.refreshing} />}>
    <ActiveFollowingSection i18n={props.i18n} onOpen={onOpenActive} onLoadMore={onLoadMoreActive} onRetry={onRetryActive} state={active} />
    <Text accessibilityRole="header" style={styles.sectionHeading}>{props.i18n.t('social.post.latest')}</Text>
    <SocialTimeline loadReactionPeople={props.loadReactionPeople} i18n={props.i18n} onLoadMore={props.onLoadMore} onDismissInteractionNotice={props.onDismissInteractionNotice} onOpen={props.onOpen} onOpenProfile={props.onOpenProfile} onOpenComments={props.onOpenComments} onRemoveReaction={props.onRemoveReaction} onRetry={props.onRetry} onSetReaction={props.onSetReaction} state={props.state} profile={props.profile} />
  </ScrollView>;
}

const styles = StyleSheet.create({
  story: { alignItems: 'center', gap: mobileTheme.spacing.xxs, width: 88 },
  storyRing: { borderWidth: 2, borderColor: mobileTheme.colors.accent, borderRadius: mobileTheme.radii.pill, padding: mobileTheme.spacing.xxs },
  storyList: { gap: mobileTheme.spacing.sm, paddingVertical: mobileTheme.spacing.xs },
  storyName: { ...mobileTheme.typography.caption, color: mobileTheme.colors.text, textAlign: 'center' },
  post: { padding: mobileTheme.spacing.md, gap: mobileTheme.spacing.sm },
  author: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.sm, minHeight: mobileTheme.sizes.minimumTouchTarget },
  postSummary: { ...mobileTheme.typography.body, color: mobileTheme.colors.text },
  attachment: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.sm, borderRadius: mobileTheme.radii.lg, borderCurve: 'continuous', padding: mobileTheme.spacing.sm },
  pathEmoji: { fontSize: 32 },
  postDuration: { ...mobileTheme.typography.heading, fontVariant: ['tabular-nums'] },
  pressed: { opacity: 0.65 },
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
    ...mobileTheme.typography.caption,
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
    paddingBottom: mobileTheme.spacing.sm,
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
  centeredState: {
    alignItems: 'center',
    flex: 1,
    justifyContent: 'center',
    minHeight: 260,
  },
  content: {
    paddingBottom: mobileTheme.spacing.sm,
  },
  copy: {
    flex: 1,
    gap: mobileTheme.spacing.xxs,
    minWidth: 0,
  },
  engagement: { alignItems: 'center', flexDirection: 'row', flexWrap: 'wrap', gap: mobileTheme.spacing.sm, paddingVertical: mobileTheme.spacing.xxs },
  commentsAction: {
    alignItems: 'center',
    alignSelf: 'flex-start',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xxs,
    minHeight: 44,
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
  unavailableState: {
    alignItems: 'stretch',
  },
  reactionCount: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xxs,
    minHeight: 44,
    borderWidth: 1,
    borderColor: mobileTheme.colors.border,
    borderRadius: mobileTheme.radii.pill,
    paddingHorizontal: mobileTheme.spacing.sm,
  },
  reactionCountSelected: {
    borderColor: mobileTheme.colors.accent,
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  reactionCounts: {
    flexDirection: 'row',
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
