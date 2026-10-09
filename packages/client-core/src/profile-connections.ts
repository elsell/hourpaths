export type ConnectionDirection = 'followers' | 'following';
export interface ConnectionPerson { id: string; username: string; name: string; picture: string | null }
export interface ConnectionsPage { items: ConnectionPerson[]; next: string | null }
export interface ProfileConnectionsRepository {
  list(username: string, direction: ConnectionDirection, cursor?: string, signal?: AbortSignal): Promise<ConnectionsPage>;
  remove(userId: string, key: string): Promise<void>;
}
export class ProfileConnectionsFailure extends Error {
  constructor(readonly kind: 'hidden' | 'rejected' | 'pending' | 'unavailable') { super(`profile_connections_${kind}`); }
}
/** Keeps a confirmed removal's retry identity within one account lifetime. */
export function createFollowerRemovalOwner(ownerId: string, keyFactory: () => string) {
  let epoch = 0, active = false;
  const retries = new Map<string, string>();
  return {
    async remove(currentOwner: string, personId: string, request: ProfileConnectionsRepository['remove']): Promise<
      { kind: 'applied' | 'superseded' | 'busy' } | { kind: 'failed'; cause: unknown }
    > {
      if (active) return { kind: 'busy' };
      if (!ownerId || currentOwner !== ownerId || !personId || personId === ownerId) return { kind: 'failed', cause: new ProfileConnectionsFailure('rejected') };
      active = true;
      const generation = epoch;
      const key = retries.get(personId) ?? keyFactory();
      retries.set(personId, key);
      try {
        await request(personId, key);
        if (epoch !== generation) return { kind: 'superseded' };
        retries.delete(personId);
        return { kind: 'applied' };
      } catch (cause) { return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (epoch === generation) active = false; }
    },
    cancel() { epoch += 1; active = false; retries.clear(); },
  };
}
