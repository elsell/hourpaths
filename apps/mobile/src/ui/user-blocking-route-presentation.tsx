import type { BlockableIdentity, UserBlockingPort } from '@hourpaths/client-core';
import { useEffect, useRef, useSyncExternalStore } from 'react';

export type UserBlockingRoutePresentation = UserBlockingPort & { sessionKey: string };
type PublishedUserBlockingPort = UserBlockingRoutePresentation & { owner: object };

let currentPort: PublishedUserBlockingPort | null = null;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function UserBlockingRouteSource({
  isCurrent,
  onBlocked,
  port,
  sessionKey,
}: {
  isCurrent: () => boolean;
  onBlocked: (target: BlockableIdentity) => void;
  port: UserBlockingPort;
  sessionKey: string;
}) {
  const owner = useRef({});
  const onBlockedRef = useRef(onBlocked);
  const portRef = useRef(port);
  onBlockedRef.current = onBlocked;
  portRef.current = port;

  useEffect(() => {
    const published: PublishedUserBlockingPort = {
      owner: owner.current,
      sessionKey,
      ...createOwnedUserBlockingPort(() => portRef.current, isCurrent, (target) => onBlockedRef.current(target)),
    };
    currentPort = published;
    emitChange();
    return () => {
      if (currentPort?.owner === owner.current) {
        currentPort = null;
        emitChange();
      }
    };
  }, [sessionKey]);

  return null;
}

export function useUserBlockingRoutePresentation(): UserBlockingRoutePresentation | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => currentPort,
    () => null,
  );
}

export function createOwnedUserBlockingPort(
  currentPort: () => UserBlockingPort,
  isCurrent: () => boolean,
  onBlocked: (target: BlockableIdentity) => void,
): UserBlockingPort {
  const owned = async <T,>(operation: () => Promise<T>): Promise<T> => {
    if (!isCurrent()) throw new Error('blocking_superseded');
    const result = await operation();
    if (!isCurrent()) throw new Error('blocking_superseded');
    return result;
  };
  return {
    blockUser: async (...arguments_) => {
      const result = await owned(() => currentPort().blockUser(...arguments_));
      if (!isCurrent()) throw new Error('blocking_superseded');
      onBlocked(result.target);
      return result;
    },
    listBlockedAccounts: (...arguments_) => owned(() => currentPort().listBlockedAccounts(...arguments_)),
    reviewBlock: (...arguments_) => owned(() => currentPort().reviewBlock(...arguments_)),
    unblockUser: (...arguments_) => owned(() => currentPort().unblockUser(...arguments_)),
  };
}
