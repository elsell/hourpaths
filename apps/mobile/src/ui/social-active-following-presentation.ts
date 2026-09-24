import type { ActiveFollowingItem as ActiveFollowingAPIItem } from '@hourpaths/api-client';

export type ActiveFollowingTimer = {
  id: string;
  path: {
    id: string;
    name: string;
  };
  startedAt: string;
};

export type ActiveFollowingItem = {
  participant: {
    displayName: string;
    profilePictureURL?: string;
    userId: string;
    username: string;
  };
  timers: readonly ActiveFollowingTimer[];
};

export type ActiveFollowingPage = {
  items: readonly ActiveFollowingItem[];
  nextCursor: string;
};

export function shouldRefreshActiveFollowing(
  previousAppState: string,
  nextAppState: string,
  routeFocused: boolean,
): boolean {
  return routeFocused && previousAppState !== 'active' && nextAppState === 'active';
}

export function activeFollowingItemFromAPI(item: ActiveFollowingAPIItem): ActiveFollowingItem {
  return {
    participant: {
      displayName: item.participant.displayName,
      profilePictureURL: item.participant.profilePictureURL,
      userId: item.participant.userId,
      username: item.participant.username,
    },
    timers: item.timers.map((timer) => ({
      id: timer.id,
      path: { id: timer.path.id, name: timer.path.name },
      startedAt: timer.startedAt,
    })),
  };
}

export function mergeActiveFollowingPage(
  current: ActiveFollowingPage,
  incoming: ActiveFollowingPage,
  requestedCursor?: string,
): ActiveFollowingPage {
  if (!requestedCursor) return incoming;

  const participantIDs = new Set(current.items.map(({ participant }) => participant.userId));
  return {
    items: [
      ...current.items,
      ...incoming.items.filter(({ participant }) => !participantIDs.has(participant.userId)),
    ],
    nextCursor: incoming.nextCursor,
  };
}
