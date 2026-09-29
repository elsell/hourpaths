import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
export function useOwnedOperation(d: StudioDependencies) {
  const client = useQueryClient();
  const [key] = useState(() => [d.accountScope, 'ownedOperation', d.operationId()]);
  const active = useRef(true);
  useEffect(() => () => { active.current = false; void client.cancelQueries({ queryKey: key }); client.removeQueries({ queryKey: key }); }, [client, key]);
  return {
    active: () => active.current,
    run: <T,>(operation: (signal: AbortSignal) => Promise<T>): Promise<T> => client.fetchQuery({ queryKey: key, queryFn: async ({ signal }) => ({ result: await operation(signal) }), staleTime: 0, gcTime: 0, retry: false }).then(value => value.result),
  };
}
