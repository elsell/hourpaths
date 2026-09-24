import type { PracticeCommentHearter, PracticeCommentHeartRosterPage } from '@hourpaths/client-core';
import type { MessageKey } from '@hourpaths/i18n';
import { useEffect, useRef, useSyncExternalStore } from 'react';

export type CommentHeartRosterRoutePresentation = {
  commentID: string;
  errorKey?: MessageKey;
  eventID: string;
  items: PracticeCommentHeartRosterPage['items'];
  loadingMore: boolean;
  nextCursor?: string;
  refreshing: boolean;
  status: 'loading' | 'ready' | 'error';
  loadMore: () => void;
  openProfile: (person: PracticeCommentHearter) => void;
  refresh: () => void;
  retry: () => void;
};

type Published = CommentHeartRosterRoutePresentation & { owner: object };
let current: Published | null | undefined;
let recovery: (() => void) | undefined;
const listeners = new Set<() => void>();
const emit = () => { for (const listener of listeners) listener(); };

export function CommentHeartRosterRouteSource(
  props: CommentHeartRosterRoutePresentation & { onUnavailable: () => void },
) {
  const owner = useRef({});
  const unavailable = useRef(props.onUnavailable);
  unavailable.current = props.onUnavailable;
  useEffect(() => {
    recovery = () => unavailable.current();
    current = { ...props, owner: owner.current };
    emit();
    return () => {
      if (current?.owner === owner.current) { current = null; emit(); }
    };
  }, []);
  useEffect(() => {
    if (current?.owner === owner.current) { current = { ...props, owner: owner.current }; emit(); }
  }, [props]);
  return null;
}

export function prepareCommentHeartRosterRoute() { current = undefined; emit(); }
export function recoverCommentHeartRosterRoute() { const callback = recovery; recovery = undefined; callback?.(); }
export function dismissCommentHeartRosterRoute() { current = null; emit(); }
export function useCommentHeartRosterRoutePresentation(eventID: string, commentID: string) {
  return useSyncExternalStore(
    (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    () => current?.eventID === eventID && current.commentID === commentID
      ? current
      : current === undefined ? undefined : null,
    () => null,
  );
}
