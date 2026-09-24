import type { MessageKey } from '@hourpaths/i18n';
import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { ActiveFollowingItem } from './social-active-following-presentation';
import type { PracticeSessionFeedEvent, SocialFeedEvent } from './social-feed-presentation';
import type { SocialReaction } from './social-reaction-presentation';

export type ActiveFollowingState = {
  errorKey?: MessageKey;
  items: readonly ActiveFollowingItem[];
  loadingMore: boolean;
  nextCursor?: string;
  refreshing: boolean;
  status: 'idle' | 'loading' | 'ready' | 'error';
};

export type SocialFeedState = {
  detailErrorKey?: MessageKey;
  errorKey?: MessageKey;
  items: readonly SocialFeedEvent[];
  interactionNoticeEventID?: string;
  interactionNoticeKey?: MessageKey;
  loadingMore: boolean;
  nextCursor?: string;
  refreshing: boolean;
  status: 'idle' | 'loading' | 'ready' | 'error';
};

export type SocialFeedRoutePresentation = {
  active: ActiveFollowingState;
  dismissInteractionNotice: () => void;
  feed: SocialFeedState;
  loadActive: () => void;
  loadFeed: () => void;
  loadMoreActive: () => void;
  loadMore: () => void;
  openActivity: (event: PracticeSessionFeedEvent) => void;
  openComments: (event: SocialFeedEvent) => void;
  removeReaction: (event: SocialFeedEvent) => Promise<void>;
  refresh: () => void;
  refreshActive: () => void;
  retry: () => void;
  retryActive: () => void;
  setReaction: (event: SocialFeedEvent, reaction: SocialReaction) => Promise<void>;
};

type PublishedPresentation = SocialFeedRoutePresentation & { owner: object };

let currentPresentation: PublishedPresentation | null | undefined;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function SocialFeedRouteSource(props: SocialFeedRoutePresentation) {
  const owner = useRef({});

  useEffect(() => {
    currentPresentation = { ...props, owner: owner.current };
    emitChange();
    return () => {
      if (currentPresentation?.owner === owner.current) {
        currentPresentation = null;
        emitChange();
      }
    };
  }, []);

  useEffect(() => {
    if (currentPresentation?.owner !== owner.current) return;
    currentPresentation = { ...props, owner: owner.current };
    emitChange();
  }, [
    props.active,
    props.dismissInteractionNotice,
    props.feed,
    props.loadActive,
    props.loadFeed,
    props.loadMore,
    props.loadMoreActive,
    props.openActivity,
    props.openComments,
    props.removeReaction,
    props.refresh,
    props.refreshActive,
    props.retry,
    props.retryActive,
    props.setReaction,
  ]);

  return null;
}

export function useSocialFeedRoutePresentation(): SocialFeedRoutePresentation | null | undefined {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => currentPresentation,
    () => null,
  );
}
