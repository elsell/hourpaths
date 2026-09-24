import type { MessageKey } from '@hourpaths/i18n';
import type { PracticeComment } from '@hourpaths/client-core';
import { useEffect, useRef, useSyncExternalStore } from 'react';

export type CommentVersionPresentation = Readonly<{ createdAt: string; text: string; version: number }>;
export type PracticeCommentsRoutePresentation = {
  busy: boolean;
  editingID?: string;
  errorKey?: MessageKey;
  eventID: string;
  eventOwnerID: string;
  focusedCommentID?: string;
  history?: { commentID: string; errorKey?: MessageKey; loading: boolean; nextCursor?: string; versions: readonly CommentVersionPresentation[] };
  items: readonly PracticeComment[];
  loadingMore: boolean;
  nextCursor?: string;
  refreshing: boolean;
  status: 'loading' | 'ready' | 'error';
  viewerID: string;
  create: (text: string) => Promise<void>;
  edit: (comment: PracticeComment, text: string) => Promise<void>;
  remove: (comment: PracticeComment) => Promise<void>;
  loadHistory: (comment: PracticeComment, cursor?: string) => void;
  mutateHeart: (comment: PracticeComment) => Promise<void>;
  openHeartRoster: (comment: PracticeComment) => void;
  loadMore: () => void;
  refresh: () => void;
  retry: () => void;
};

type Published = PracticeCommentsRoutePresentation & { owner: object };
let current: Published | null | undefined;
let recovery: (() => void) | undefined;
const listeners = new Set<() => void>();
const emit = () => { for (const listener of listeners) listener(); };

export function PracticeCommentsRouteSource(props: PracticeCommentsRoutePresentation & { onUnavailable: () => void }) {
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

export function preparePracticeCommentsRoute() { current = undefined; emit(); }
export function recoverPracticeCommentsRoute() { const callback = recovery; recovery = undefined; callback?.(); }
export function dismissPracticeCommentsRoute() { current = null; emit(); }
export function usePracticeCommentsRoutePresentation(eventID: string) {
  return useSyncExternalStore(
    (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    () => current?.eventID === eventID ? current : current === undefined ? undefined : null,
    () => null,
  );
}
