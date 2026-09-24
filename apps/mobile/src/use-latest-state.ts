import { useCallback, useRef, useState, type Dispatch, type MutableRefObject, type SetStateAction } from 'react';

export function commitLatestStateUpdate<T>(
  latest: MutableRefObject<T>,
  update: SetStateAction<T>,
): T {
  const next = typeof update === 'function'
    ? (update as (current: T) => T)(latest.current)
    : update;
  latest.current = next;
  return next;
}

export function useLatestState<T>(initial: T): readonly [T, Dispatch<SetStateAction<T>>, MutableRefObject<T>] {
  const [state, setState] = useState(initial);
  const latest = useRef(state);
  const publish: Dispatch<SetStateAction<T>> = useCallback((update) => {
    setState(commitLatestStateUpdate(latest, update));
  }, []);
  return [state, publish, latest] as const;
}
