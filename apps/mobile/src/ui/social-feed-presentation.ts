import type { PracticeFeedItem, PracticeReactionSummary } from '@hourpaths/api-client';
import {
  socialReactionFromAPI,
  type SocialReaction,
  type SocialReactionCounts,
} from './social-reaction-presentation';

type SocialFeedEventBase = {
  commentsEnabled: boolean;
  id: string;
  participant: {
    displayName: string;
    profilePictureURL?: string;
    userId: string;
    username: string;
  };
  path: {
    id: string;
    name: string;
  };
  publishedAt: string;
  reactions: SocialReactionCounts;
  reactionsEnabled: boolean;
  viewerReaction: SocialReaction | null;
};

export type PracticeSessionFeedEvent = SocialFeedEventBase & {
  achievement?: never;
  activity: {
    durationSeconds: number;
    edited: boolean;
    id: string;
  };
  type: 'practice_session';
};

export type GoalAchievementFeedEvent = SocialFeedEventBase & {
  activity?: never;
  achievement: {
    intervalEndedAt?: string;
    intervalStartedAt?: string;
    kind: 'interval' | 'overall';
    targetSeconds: number;
  };
  type: 'goal_achievement';
};

export type SocialFeedEvent = PracticeSessionFeedEvent | GoalAchievementFeedEvent;

export type SocialFeedPage = {
  items: readonly SocialFeedEvent[];
  nextCursor: string;
};

export type SocialReactionSummary = {
  reactions: SocialReactionCounts;
  viewerReaction: SocialReaction | null;
};

export function socialFeedEventFromAPI(item: PracticeFeedItem): SocialFeedEvent {
  if (typeof item.commentsEnabled !== 'boolean' || typeof item.reactionsEnabled !== 'boolean') {
    throw new Error('invalid_social_feed_item');
  }
  const shared = {
    commentsEnabled: item.commentsEnabled,
    id: item.id,
    participant: {
      displayName: item.participant.displayName,
      profilePictureURL: item.participant.profilePictureURL,
      userId: item.participant.userId,
      username: item.participant.username,
    },
    path: { id: item.path.id, name: item.path.name },
    publishedAt: item.publishedAt,
    reactions: {
      applause: item.reactions.applause,
      celebrate: item.reactions.celebrate,
      fire: item.reactions.fire,
      heart: item.reactions.heart,
      strong: item.reactions.strong,
    },
    reactionsEnabled: item.reactionsEnabled,
    viewerReaction: socialReactionFromAPI(item.viewerReaction),
  };

  if (item.type === 'practice_session') {
    if (!validPracticeFeedActivity(item.activity) || item.achievement !== undefined) {
      throw new Error('invalid_social_feed_item');
    }
    return {
      ...shared,
      activity: {
        durationSeconds: item.activity.durationSeconds,
        edited: item.activity.edited,
        id: item.activity.id,
      },
      type: 'practice_session',
    };
  }
  if (item.type === 'goal_achievement') {
    if (item.activity !== undefined || !validGoalAchievement(item.achievement)) {
      throw new Error('invalid_social_feed_item');
    }
    const achievement = item.achievement.kind === 'interval'
      ? {
          intervalEndedAt: item.achievement.intervalEndedAt,
          intervalStartedAt: item.achievement.intervalStartedAt,
          kind: item.achievement.kind,
          targetSeconds: item.achievement.targetSeconds,
        }
      : {
          kind: item.achievement.kind,
          targetSeconds: item.achievement.targetSeconds,
        };
    return {
      ...shared,
      achievement,
      type: 'goal_achievement',
    };
  }
  throw new Error('invalid_social_feed_item');
}

function validPracticeFeedActivity(activity: PracticeFeedItem['activity']): activity is NonNullable<PracticeFeedItem['activity']> {
  return activity !== undefined &&
    typeof activity.id === 'string' && activity.id.trim() === activity.id && activity.id.length > 0 &&
    Number.isSafeInteger(activity.durationSeconds) && activity.durationSeconds > 0 &&
    typeof activity.edited === 'boolean';
}

function validGoalAchievement(achievement: PracticeFeedItem['achievement']): achievement is NonNullable<PracticeFeedItem['achievement']> {
  if (!achievement || !Number.isSafeInteger(achievement.targetSeconds) || achievement.targetSeconds <= 0) {
    return false;
  }
  if (achievement.kind === 'overall') {
    return achievement.intervalStartedAt === undefined && achievement.intervalEndedAt === undefined;
  }
  if (achievement.kind !== 'interval' ||
      !validDateTime(achievement.intervalStartedAt) || !validDateTime(achievement.intervalEndedAt)) {
    return false;
  }
  return Date.parse(achievement.intervalEndedAt) > Date.parse(achievement.intervalStartedAt);
}

function validDateTime(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && Number.isFinite(Date.parse(value));
}

export function mergeSocialFeedPage(
  current: SocialFeedPage,
  incoming: SocialFeedPage,
  requestedCursor?: string,
): SocialFeedPage {
  if (!requestedCursor) return incoming;

  const ids = new Set(current.items.map(({ id }) => id));
  return {
    items: [
      ...current.items,
      ...incoming.items.filter(({ id }) => !ids.has(id)),
    ],
    nextCursor: incoming.nextCursor,
  };
}

export function preserveNewerSocialReactionSummaries(
  incoming: SocialFeedPage,
  current: SocialFeedPage,
  admittedRevisions: ReadonlyMap<string, number>,
  currentRevisions: ReadonlyMap<string, number>,
): SocialFeedPage {
  const currentByID = new Map(current.items.map((item) => [item.id, item]));
  let changed = false;
  const items = incoming.items.map((item) => {
    if ((currentRevisions.get(item.id) ?? 0) <= (admittedRevisions.get(item.id) ?? 0)) return item;
    const authoritative = currentByID.get(item.id);
    if (!authoritative) return item;
    changed = true;
    return {
      ...item,
      reactions: { ...authoritative.reactions },
      viewerReaction: authoritative.viewerReaction,
    };
  });
  return changed ? { ...incoming, items } : incoming;
}

export function applySocialReactionSummary(
  page: SocialFeedPage,
  eventID: string,
  summary: SocialReactionSummary,
): SocialFeedPage {
  const index = page.items.findIndex(({ id }) => id === eventID);
  if (index < 0) return page;

  const items = [...page.items];
  items[index] = {
    ...items[index]!,
    reactions: { ...summary.reactions },
    viewerReaction: summary.viewerReaction,
  };
  return { ...page, items };
}

export function socialReactionSummaryFromAPI(summary: PracticeReactionSummary): SocialReactionSummary {
  return {
    reactions: {
      applause: summary.reactions.applause,
      celebrate: summary.reactions.celebrate,
      fire: summary.reactions.fire,
      heart: summary.reactions.heart,
      strong: summary.reactions.strong,
    },
    viewerReaction: socialReactionFromAPI(summary.viewerReaction),
  };
}

export function optimisticSocialReactionSummary(
  event: SocialFeedEvent,
  nextReaction: SocialReaction | null,
): SocialReactionSummary {
  const reactions = { ...event.reactions };
  if (event.viewerReaction !== nextReaction) {
    if (event.viewerReaction) {
      reactions[event.viewerReaction] = Math.max(0, reactions[event.viewerReaction] - 1);
    }
    if (nextReaction) reactions[nextReaction] += 1;
  }
  return { reactions, viewerReaction: nextReaction };
}
