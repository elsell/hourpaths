import type { MessageKey } from '@hourpaths/i18n';
import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { SocialPublicProfile } from './social-profile-presentation';

export type SocialProfileSearchState = {
  errorKey?: MessageKey;
  items: readonly SocialPublicProfile[];
  loadingMore: boolean;
  nextCursor?: string;
  query: string;
  refreshing: boolean;
  status: 'idle' | 'loading' | 'ready' | 'error';
};

export type SocialProfileDetailState = {
  errorKey?: MessageKey;
  mutationErrorKey?: MessageKey;
  profile?: SocialPublicProfile;
  refreshing: boolean;
  status: 'loading' | 'ready' | 'error';
  username?: string;
  mutating?: boolean;
};

export type SocialRelationshipAction = 'follow' | 'cancel-request' | 'unfollow';

export type SocialFollowRequest = {
  id: string;
  requester: SocialPublicProfile;
  createdAt: string;
};

export type SocialFollowRequestState = {
  busyRequestID?: string;
  errorKey?: MessageKey;
  items: readonly SocialFollowRequest[];
  nextCursor?: string;
  refreshing: boolean;
  status: 'idle' | 'loading' | 'ready' | 'error';
};

export type SocialProfileRoutePresentation = {
  loadMore: () => void;
  loadFollowRequests: (refreshing?: boolean) => void;
  loadMoreFollowRequests: () => void;
  loadProfile: (username: string) => void;
  mutateRelationship: (action: SocialRelationshipAction, username: string, idempotencyKey?: string) => void;
  profile: SocialProfileDetailState;
  followRequests: SocialFollowRequestState;
  refreshProfile: () => void;
  refreshSearch: () => void;
  retryProfile: () => void;
  retrySearch: () => void;
  reviewFollowRequest: (decision: 'accept' | 'reject', requestID: string, idempotencyKey?: string) => void;
  search: SocialProfileSearchState;
  searchQuery: (query: string) => void;
};

type PublishedPresentation = SocialProfileRoutePresentation & { owner: object; sessionKey: string };

let currentPresentation: PublishedPresentation | null | undefined;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function SocialProfileRouteSource(props: SocialProfileRoutePresentation & { isCurrent: () => boolean; sessionKey: string }) {
  const owner = useRef({});
  const sessionKey = props.sessionKey;

  const publish = (): PublishedPresentation => ({
    ...props,
    owner: owner.current,
    sessionKey,
    mutateRelationship: (...arguments_) => {
      if (props.isCurrent() && currentPresentation?.sessionKey === sessionKey) props.mutateRelationship(...arguments_);
    },
    reviewFollowRequest: (...arguments_) => {
      if (props.isCurrent() && currentPresentation?.sessionKey === sessionKey) props.reviewFollowRequest(...arguments_);
    },
  });

  useEffect(() => {
    currentPresentation = publish();
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
    currentPresentation = publish();
    emitChange();
  }, [
    props.loadMore,
    props.isCurrent,
    props.loadFollowRequests,
    props.loadMoreFollowRequests,
    props.loadProfile,
    props.mutateRelationship,
    props.profile,
    props.followRequests,
    props.refreshProfile,
    props.refreshSearch,
    props.retryProfile,
    props.retrySearch,
    props.reviewFollowRequest,
    props.search,
    props.searchQuery,
    sessionKey,
  ]);

  return null;
}

export function useSocialProfileRoutePresentation(): SocialProfileRoutePresentation | null | undefined {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => currentPresentation,
    () => null,
  );
}

export function canBlockCurrentSocialProfile(username: string) {
  return currentPresentation?.profile.status === 'ready' &&
    currentPresentation.profile.profile?.username.toLowerCase() === username.toLowerCase() &&
    currentPresentation.profile.profile.relationship !== 'self';
}
