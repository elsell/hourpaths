import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { ActivityDetail } from '../activity-history';
import type { ActivityRevision } from '../activity-history';
import type { MessageKey } from '@hourpaths/i18n';
import type { SocialFeedEvent } from './social-feed-presentation';

export type SocialFeedActivityDetailPresentation = {
  busy: boolean;
  detail: ActivityDetail;
  errorKey?: MessageKey;
  event: SocialFeedEvent;
  loadMoreRevisions: () => void;
  nextCursor?: string;
  profileID: string;
  retryRevisions: () => void;
  revisions: readonly ActivityRevision[];
  routeKey: string;
};

type PublishedPresentation = SocialFeedActivityDetailPresentation & { owner: object };

let currentPresentation: PublishedPresentation | null = null;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function socialFeedActivityDetailRouteKey(pathID: string, activityID: string) {
  return `social-feed-activity:${pathID}:${activityID}`;
}

export function SocialFeedActivityDetailRouteSource(props: SocialFeedActivityDetailPresentation) {
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
    props.busy,
    props.detail,
    props.errorKey,
    props.event,
    props.loadMoreRevisions,
    props.nextCursor,
    props.profileID,
    props.retryRevisions,
    props.revisions,
    props.routeKey,
  ]);

  return null;
}

export function useSocialFeedActivityDetailRoutePresentation(
  routeKey: string,
): SocialFeedActivityDetailPresentation | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => currentPresentation?.routeKey === routeKey ? currentPresentation : null,
    () => null,
  );
}
