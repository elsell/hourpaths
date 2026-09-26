import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  applySocialReactionSummary,
  mergeSocialFeedPage,
  optimisticSocialReactionSummary,
  preserveNewerSocialReactionSummaries,
  socialFeedEventFromAPI,
  socialReactionSummaryFromAPI,
  type PracticeSessionFeedEvent,
  type SocialFeedEvent,
} from './ui/social-feed-presentation';
import {
  SOCIAL_REACTIONS,
  socialReactionFromAPI,
  visibleReactionCounts,
} from './ui/social-reaction-presentation';
import { prepareSocialFeedActivityDetail } from './social-feed-activity-detail';
import type { ActivityDetail } from './activity-history';
import type { ActivityRevision } from './activity-history';

function source(path: string) {
  const file = fileURLToPath(new URL(path, import.meta.url));
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const followingLayout = source('../app/(tabs)/following/_layout.tsx') + source('../app/_layout.tsx');
const followingRoute = source('../app/(tabs)/following/index.tsx');
const peopleRoute = source('../app/following/people.tsx');
const feedView = source('./ui/social-feed-view.tsx');
const routePresentation = source('./ui/social-feed-route-presentation.tsx');
const reactionMenu = source('./ui/social-reaction-menu.ios.tsx');
const reactionMenuFallback = source('./ui/social-reaction-menu.tsx');
const detailRoute = source('../app/following/activity/[pathID]/[activityID].tsx');
const detailView = source('./ui/social-feed-activity-detail-view.tsx');
const homeOrchestration = source('../app/index.tsx');
const fixtureDisplayName = ['Alex', 'Rivera'].join(' ');
const fixturePathName = ['Pi', 'ano'].join('');

function event(id: string, publishedAt: string): PracticeSessionFeedEvent {
  return {
    activity: { durationSeconds: 900, edited: false, id: `activity-${id}` },
    commentsEnabled: true,
    id,
    participant: { displayName: fixtureDisplayName, userId: 'user-1', username: 'alex' },
    path: { id: 'path-1', name: fixturePathName },
    publishedAt,
    reactionsEnabled: true,
    reactions: { applause: 0, celebrate: 0, fire: 0, heart: 0, strong: 0 },
    type: 'practice_session',
    viewerReaction: null,
  };
}

test('feed pagination preserves server order and removes stable duplicate events', () => {
  const first = event('event-1', '2026-07-27T12:00:00Z');
  const second = event('event-2', '2026-07-27T11:00:00Z');
  const result = mergeSocialFeedPage(
    { items: [first], nextCursor: 'page-2' },
    { items: [first, second], nextCursor: 'page-3' },
    'page-2',
  );

  assert.deepEqual(result.items.map(({ id }) => id), ['event-1', 'event-2']);
  assert.equal(result.nextCursor, 'page-3');
});

test('a refreshed first page replaces stale chronological results', () => {
  const first = event('event-1', '2026-07-27T12:00:00Z');
  const replacement = event('event-3', '2026-07-27T13:00:00Z');
  const result = mergeSocialFeedPage(
    { items: [first], nextCursor: 'old' },
    { items: [replacement], nextCursor: 'new' },
  );

  assert.deepEqual(result.items.map(({ id }) => id), ['event-3']);
  assert.equal(result.nextCursor, 'new');
});

test('authoritative reaction summary updates one event without reordering or dropping concurrent feed items', () => {
  const first = event('event-1', '2026-07-27T12:00:00Z');
  const second = event('event-2', '2026-07-27T11:00:00Z');
  const third = event('event-3', '2026-07-27T10:00:00Z');
  const page = { items: [first, second, third], nextCursor: 'page-2' };

  const updated = applySocialReactionSummary(page, second.id, {
    reactions: { applause: 0, celebrate: 0, fire: 1, heart: 4, strong: 0 },
    viewerReaction: 'fire',
  });

  assert.deepEqual(updated.items.map(({ id }) => id), ['event-1', 'event-2', 'event-3']);
  assert.equal(updated.nextCursor, 'page-2');
  assert.strictEqual(updated.items[0], first);
  assert.strictEqual(updated.items[2], third);
  assert.deepEqual(updated.items[1]?.reactions, { applause: 0, celebrate: 0, fire: 1, heart: 4, strong: 0 });
  assert.equal(updated.items[1]?.viewerReaction, 'fire');
  assert.strictEqual(applySocialReactionSummary(page, 'stale-event', {
    reactions: first.reactions,
    viewerReaction: null,
  }), page);
});

test('a delayed feed response preserves only reaction projections changed after admission', () => {
  const first = event('event-1', '2026-07-27T13:00:00Z');
  const second = event('event-2', '2026-07-27T12:00:00Z');
  const admitted = new Map([['event-1', 3], ['event-2', 7]]);
  const currentRevisions = new Map([['event-1', 4], ['event-2', 7]]);
  const current = {
    items: [
      { ...first, reactions: { ...first.reactions, fire: 2 }, viewerReaction: 'fire' as const },
      { ...second, reactions: { ...second.reactions, heart: 9 } },
    ],
    nextCursor: 'current',
  };
  const incoming = {
    items: [
      { ...first, reactions: { ...first.reactions, applause: 1 } },
      { ...second, reactions: { ...second.reactions, heart: 4 } },
    ],
    nextCursor: 'incoming',
  };

  const result = preserveNewerSocialReactionSummaries(incoming, current, admitted, currentRevisions);

  assert.equal(result.nextCursor, 'incoming');
  assert.equal(result.items[0]?.viewerReaction, 'fire');
  assert.equal(result.items[0]?.reactions.fire, 2);
  assert.equal(result.items[0]?.reactions.applause, 0);
  assert.equal(result.items[1]?.reactions.heart, 4);
});

test('generated reaction summary mapping admits only curated viewer reactions', () => {
  const reactions = { applause: 1, celebrate: 2, fire: 3, heart: 4, strong: 5 };
  assert.deepEqual(socialReactionSummaryFromAPI({ reactions, viewerReaction: 'heart' }), {
    reactions,
    viewerReaction: 'heart',
  });
  assert.equal(socialReactionSummaryFromAPI({
    reactions,
    viewerReaction: 'unapproved',
  } as unknown as Parameters<typeof socialReactionSummaryFromAPI>[0]).viewerReaction, null);
});

test('optimistic reaction summaries replace and remove without disturbing other counts', () => {
  const selected = {
    ...event('event-1', '2026-07-27T12:00:00Z'),
    reactions: { applause: 2, celebrate: 0, fire: 0, heart: 4, strong: 0 },
    viewerReaction: 'heart' as const,
  };
  assert.deepEqual(optimisticSocialReactionSummary(selected, 'applause'), {
    reactions: { applause: 3, celebrate: 0, fire: 0, heart: 3, strong: 0 },
    viewerReaction: 'applause',
  });
  assert.deepEqual(optimisticSocialReactionSummary(selected, null), {
    reactions: { applause: 2, celebrate: 0, fire: 0, heart: 3, strong: 0 },
    viewerReaction: null,
  });
  assert.deepEqual(optimisticSocialReactionSummary(selected, 'heart'), {
    reactions: selected.reactions,
    viewerReaction: 'heart',
  });
});

test('generated feed mapping keeps only the approved projection and never copies a note', () => {
  const mapped = socialFeedEventFromAPI({
    ...event('event-1', '2026-07-27T12:00:00Z'),
    note: 'not in the contract',
  } as Parameters<typeof socialFeedEventFromAPI>[0]);
  assert.deepEqual(Object.keys(mapped).sort(), [
    'activity',
    'commentsEnabled',
    'id',
    'participant',
    'path',
    'publishedAt',
    'reactions',
    'reactionsEnabled',
    'type',
    'viewerReaction',
  ]);
  assert.equal(mapped.type, 'practice_session');
  if (mapped.type !== 'practice_session') assert.fail('expected practice session');
  assert.deepEqual(Object.keys(mapped.activity).sort(), ['durationSeconds', 'edited', 'id']);
  assert.equal('note' in mapped, false);
  assert.equal('note' in mapped.activity, false);
});

test('feed mapping forms an exact discriminated union for practice and goal achievements', () => {
  const reactions = { applause: 0, celebrate: 0, fire: 0, heart: 0, strong: 0 };
  const achievement = socialFeedEventFromAPI({
    achievement: {
      intervalEndedAt: '2026-07-28T12:00:00Z',
      intervalStartedAt: '2026-07-27T12:00:00Z',
      kind: 'interval',
      note: 'must not enter the mobile projection',
      targetSeconds: 7_200,
    },
    id: 'achievement-1',
    commentsEnabled: false,
    participant: { displayName: fixtureDisplayName, userId: 'user-1', username: 'alex' },
    path: { id: 'path-1', name: fixturePathName },
    publishedAt: '2026-07-28T12:00:00Z',
    reactions,
    reactionsEnabled: true,
    type: 'goal_achievement',
    viewerReaction: null,
  } as Parameters<typeof socialFeedEventFromAPI>[0]);

  assert.equal(achievement.type, 'goal_achievement');
  if (achievement.type !== 'goal_achievement') assert.fail('expected goal achievement');
  assert.deepEqual(achievement.achievement, {
    intervalEndedAt: '2026-07-28T12:00:00Z',
    intervalStartedAt: '2026-07-27T12:00:00Z',
    kind: 'interval',
    targetSeconds: 7_200,
  });
  assert.equal('activity' in achievement, false);
  assert.equal('note' in achievement.achievement, false);
  assert.equal(achievement.commentsEnabled, false);
  assert.equal(achievement.reactionsEnabled, true);

  const overall = socialFeedEventFromAPI({
    achievement: { kind: 'overall', targetSeconds: 36_000 },
    id: 'achievement-2',
    commentsEnabled: true,
    participant: { displayName: fixtureDisplayName, userId: 'user-1', username: 'alex' },
    path: { id: 'path-1', name: fixturePathName },
    publishedAt: '2026-07-28T12:00:00Z',
    reactions,
    reactionsEnabled: false,
    type: 'goal_achievement',
    viewerReaction: null,
  });
  assert.equal(overall.type, 'goal_achievement');
  if (overall.type !== 'goal_achievement') assert.fail('expected overall achievement');
  assert.deepEqual(overall.achievement, { kind: 'overall', targetSeconds: 36_000 });
});

test('feed mapping rejects crossed or invalid discriminant payloads at runtime', () => {
  const base = {
    commentsEnabled: true,
    id: 'event-invalid',
    participant: { displayName: fixtureDisplayName, userId: 'user-1', username: 'alex' },
    path: { id: 'path-1', name: fixturePathName },
    publishedAt: '2026-07-28T12:00:00Z',
    reactions: { applause: 0, celebrate: 0, fire: 0, heart: 0, strong: 0 },
    reactionsEnabled: true,
    viewerReaction: null,
  };
  const activity = { durationSeconds: 900, edited: false, id: 'activity-1' };
  const interval = {
    intervalEndedAt: '2026-07-28T12:00:00Z',
    intervalStartedAt: '2026-07-27T12:00:00Z',
    kind: 'interval' as const,
    targetSeconds: 7_200,
  };
  const invalid = [
    { ...base, commentsEnabled: undefined, activity, type: 'practice_session' },
    { ...base, reactionsEnabled: undefined, activity, type: 'practice_session' },
    { ...base, type: 'practice_session' },
    { ...base, activity: { ...activity, durationSeconds: 0 }, type: 'practice_session' },
    { ...base, achievement: interval, activity, type: 'practice_session' },
    { ...base, type: 'goal_achievement' },
    { ...base, achievement: interval, activity, type: 'goal_achievement' },
    { ...base, achievement: { ...interval, intervalEndedAt: undefined }, type: 'goal_achievement' },
    { ...base, achievement: { ...interval, intervalEndedAt: interval.intervalStartedAt }, type: 'goal_achievement' },
    { ...base, achievement: { kind: 'overall', targetSeconds: 0 }, type: 'goal_achievement' },
    { ...base, achievement: { kind: 'overall', targetSeconds: 1.5 }, type: 'goal_achievement' },
    { ...base, achievement: { intervalStartedAt: interval.intervalStartedAt, kind: 'overall', targetSeconds: 7_200 }, type: 'goal_achievement' },
  ];

  for (const item of invalid) {
    assert.throws(() => socialFeedEventFromAPI(item as never), /invalid_social_feed_item/);
  }
});

test('Following root is the compact practice feed and keeps social destinations native', () => {
  assert.match(followingLayout, /<Stack/);
  assert.match(followingLayout, /name="people"/);
  assert.match(followingRoute, /<SocialFeedView/);
  assert.match(followingRoute, /router\.push\('\/(?:\(tabs\)\/)?following\/people'\)/);
  assert.match(followingRoute, /router\.push\('\/follow-requests'\)/);
  assert.doesNotMatch(followingRoute, /headerSearchBarOptions/);
  assert.match(peopleRoute, /headerSearchBarOptions/);
  assert.match(peopleRoute, /<SocialProfileSearchView/);
});

test('feed rows are accessible, compact, branded, and open existing activity detail', () => {
  assert.match(feedView, /accessibilityRole="button"/);
  assert.match(feedView, /<SocialProfileAvatar/);
  assert.match(feedView, /formatCompactDuration/);
  assert.match(feedView, /mobileTheme\.colors\.accent/);
  assert.match(feedView, /social\.feedRowEdited/);
  assert.match(feedView, /NativeContentUnavailable/);
  assert.match(feedView, /RefreshControl/);
  assert.match(feedView, /state\.detailErrorKey/);
  assert.match(followingRoute, /onOpen=\{presentation\.openActivity\}/);
});

test('goal achievement row is compact, native, accessible, and exposes engagement without activity navigation', () => {
  const achievementRow = feedView.slice(
    feedView.indexOf('function AchievementFeedRow'),
    feedView.indexOf('function FeedRow'),
  );
  assert.match(achievementRow, /accessible/);
  assert.match(achievementRow, /accessibilityLabel/);
  assert.match(achievementRow, /systemName="trophy\.fill"/);
  assert.match(achievementRow, /formatCompactDuration\(event\.achievement\.targetSeconds, i18n\)/);
  assert.match(achievementRow, /social\.feedAchievementInterval/);
  assert.match(achievementRow, /social\.feedAchievementOverall/);
  assert.match(achievementRow, /<FeedEngagement/);
  assert.match(achievementRow, /onOpenComments=\{onOpenComments\}/);
  assert.doesNotMatch(achievementRow, /onOpen(?:=|:)|chevron\.right/);
});

test('all social events use the same comments and reaction controls while only practice opens activity', () => {
  const engagement = feedView.slice(
    feedView.indexOf('function FeedEngagement'),
    feedView.indexOf('function AchievementFeedRow'),
  );
  assert.match(engagement, /event: SocialFeedEvent/);
  assert.match(engagement, /onOpenComments: \(\) => void/);
  assert.match(engagement, /<ReactionStrip/);
  assert.match(engagement, /SettingsIcon systemName="bubble\.left"/);
  assert.match(engagement, /event\.commentsEnabled \? <Pressable/);
  assert.match(engagement, /event\.reactionsEnabled \? <ReactionStrip/);
  assert.match(feedView, /onOpenComments: \(event: SocialFeedEvent\) => void/);
  assert.match(feedView, /onRemoveReaction: \(event: SocialFeedEvent\) => Promise<void>/);
  assert.match(feedView, /onSetReaction: \(event: SocialFeedEvent, reaction: SocialReaction\) => Promise<void>/);
  assert.match(feedView, /onDismissInteractionNotice: \(\) => void/);
  assert.match(feedView, /systemName="xmark"/);
  assert.match(feedView, /onPress=\{onDismiss\}/);
  assert.match(routePresentation, /openActivity: \(event: PracticeSessionFeedEvent\) => void/);
  assert.match(routePresentation, /openComments: \(event: SocialFeedEvent\) => void/);
  assert.match(routePresentation, /removeReaction: \(event: SocialFeedEvent\) => Promise<void>/);
  assert.match(routePresentation, /setReaction: \(event: SocialFeedEvent, reaction: SocialReaction\) => Promise<void>/);
  assert.match(routePresentation, /dismissInteractionNotice: \(\) => void/);
  assert.match(homeOrchestration, /mutateSocialFeedReaction\(event: SocialFeedEvent/);
  assert.match(homeOrchestration, /currentEvent = socialFeedPage\.current\.items\.find[\s\S]*!currentEvent\?\.reactionsEnabled/);
});

test('practice reaction presentation is curated, deterministic, and hides zero counts', () => {
  assert.deepEqual(SOCIAL_REACTIONS.map(({ emoji, type }) => [type, emoji]), [
    ['heart', '❤️'],
    ['applause', '👏'],
    ['fire', '🔥'],
    ['strong', '💪'],
    ['celebrate', '🎉'],
  ]);
  assert.deepEqual(visibleReactionCounts({
    applause: 0,
    celebrate: 1,
    fire: 3,
    heart: 2,
    strong: 0,
  }), [
    { count: 2, emoji: '❤️', labelKey: 'social.reactionHeart', type: 'heart' },
    { count: 3, emoji: '🔥', labelKey: 'social.reactionFire', type: 'fire' },
    { count: 1, emoji: '🎉', labelKey: 'social.reactionCelebrate', type: 'celebrate' },
  ]);
  assert.equal(socialReactionFromAPI('strong'), 'strong');
  assert.equal(socialReactionFromAPI('discourage'), null);
});

test('feed interaction controls stay outside activity navigation and expose native accessibility states', () => {
  const feedRow = feedView.slice(feedView.indexOf('function FeedRow'), feedView.indexOf('function FeedUnavailable'));
  const activityNavigation = feedRow.slice(feedRow.indexOf('<Pressable'), feedRow.indexOf('</Pressable>'));
  assert.match(feedView, /<Pressable[\s\S]*onPress=\{onOpen\}[\s\S]*<\/Pressable>[\s\S]*<FeedEngagement/);
  assert.match(feedView, /minHeight: 44/);
  assert.match(feedView, /flexWrap: 'wrap'/);
  assert.match(feedView, /accessibilityLiveRegion="assertive"/);
  assert.match(feedView, /accessibilityState=\{\{ selected/);
  assert.match(reactionMenu, /Image as SwiftUIImage/);
  assert.match(reactionMenu, /systemName=\{selectedEmoji \? 'heart\.fill' : 'heart'\}/);
  assert.match(reactionMenu, /tint\(mobileTheme\.colors\.accent\)/);
  assert.match(reactionMenu, /nativeDisabled\(busy\)/);
  assert.match(reactionMenuFallback, /<Modal/);
  assert.match(reactionMenuFallback, /AccessibilityInfo\.isReduceMotionEnabled/);
  assert.doesNotMatch(activityNavigation, /ReactionStrip|SocialReactionMenu/);
});

test('published feed state supports initial load, refresh, retry, and pagination', () => {
  assert.match(routePresentation, /useSyncExternalStore/);
  assert.match(routePresentation, /SocialFeedRouteSource/);
  assert.match(routePresentation, /loadFeed/);
  assert.match(routePresentation, /loadMore/);
  assert.match(routePresentation, /openActivity/);
  assert.match(routePresentation, /refresh/);
  assert.match(routePresentation, /retry/);
});

test('feed activity detail loads by event identifiers without requiring session Path membership', async () => {
  const base = event('event-public', '2026-07-27T12:00:00Z');
  const selected = {
    ...base,
    activity: { ...base.activity, edited: true },
  };
  const calls: string[] = [];
  const detail: ActivityDetail = {
    activity: {
      createdAt: selected.publishedAt,
      durationSeconds: selected.activity.durationSeconds,
      endedAt: '2026-07-27T11:15:00Z',
      id: selected.activity.id,
      note: 'must remain private',
      occurrenceTimeZone: 'America/New_York',
      participantId: selected.participant.userId,
      pathId: selected.path.id,
      startedAt: '2026-07-27T11:00:00Z',
      updatedAt: selected.publishedAt,
    },
    version: 1,
  };
  const revision: ActivityRevision = {
    ...detail.activity,
    note: undefined,
    replacedAt: '2026-07-27T12:30:00Z',
    version: 1,
  };

  const result = await prepareSocialFeedActivityDetail({
    event: selected,
    load: async (pathID, activityID) => {
      calls.push(`load:${pathID}:${activityID}`);
      return detail;
    },
    loadRevisions: async (pathID, activityID) => {
      calls.push(`revisions:${pathID}:${activityID}`);
      return { items: [revision], nextCursor: 'revision-page-2' };
    },
    commitRoute: (pathID, activityID) => calls.push(`navigate:${pathID}:${activityID}`),
    publish: (loaded, revisions, nextCursor) => calls.push(
      `publish:${loaded.activity.id}:${revisions.length}:${nextCursor}`,
    ),
  });

  assert.deepEqual(calls, [
    'load:path-1:activity-event-public',
    'revisions:path-1:activity-event-public',
    'publish:activity-event-public:1:revision-page-2',
    'navigate:path-1:activity-event-public',
  ]);
  assert.equal(result.kind, 'loaded');
  if (result.kind === 'loaded') assert.equal(result.revisions.length, 1);
});

test('failed or mismatched feed activity detail never navigates', async () => {
  const selected = event('event-public', '2026-07-27T12:00:00Z');
  let navigations = 0;
  const result = await prepareSocialFeedActivityDetail({
    event: selected,
    load: async () => { throw new Error('unavailable'); },
    loadRevisions: async () => assert.fail('revisions require a loaded detail'),
    commitRoute: () => { navigations += 1; },
    publish: () => assert.fail('failed detail must not publish'),
  });

  assert.equal(result.kind, 'failed');
  assert.equal(navigations, 0);
});

test('feed-origin detail is a dedicated native read-only destination with public revision history', () => {
  assert.match(detailRoute, /useSocialFeedActivityDetailRoutePresentation/);
  assert.match(detailRoute, /<NativeRouteScreen/);
  assert.match(detailRoute, /<SocialFeedActivityDetailView/);
  assert.match(detailView, /<ActivityDetailView/);
  assert.match(detailView, /archived=\{true\}/);
  assert.match(detailView, /revisions=\{state\.revisions\}/);
  assert.match(detailView, /onEdit=\{noop\}/);
  assert.match(detailView, /participantLabel=\{state\.event\.participant\.displayName\}/);
  assert.match(homeOrchestration, /prepareSocialFeedActivityDetail/);
  assert.match(homeOrchestration, /\.activity\(pathID, activityID\)/);
  assert.match(homeOrchestration, /<SocialFeedActivityDetailRouteSource/);
  assert.match(homeOrchestration, /\.socialFeed\(cursor \|\| undefined\)/);
  assert.match(homeOrchestration, /<SocialFeedRouteSource/);
  assert.match(homeOrchestration, /pathname: '\/following\/activity\/\[pathID\]\/\[activityID\]'/);
});
