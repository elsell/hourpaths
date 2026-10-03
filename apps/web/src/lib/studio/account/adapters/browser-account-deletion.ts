import { createSessionApiClient } from '@hourpaths/api-client';
import type { AccountDeletionService } from '../ports/account-deletion';
import { AccountDeletion, apiAccountDeletion } from '@hourpaths/client-core';
import { browserSessionState } from '../../../browser-session-state';
import { IndexedDBTrackingStore } from '../../offline/adapters/indexeddb-tracking-store';
import type { SessionStore } from '../../session/ports/session-store';

/** Uses fresh account ownership for every request; never captures a bearer token. */
export function browserAccountDeletion(apiURL: string, sessions: SessionStore, local: IndexedDBTrackingStore,
  stopAccount: (owner: string) => void, changed: () => void): AccountDeletionService {
  const deletion = new AccountDeletion({
    currentOwner: () => sessions.read()?.ownerId ?? null,
    newSecret: () => Array.from(crypto.getRandomValues(new Uint8Array(32)), byte => byte.toString(16).padStart(2, '0')).join(''),
    journal: local.deletion,
    async fence(owner) {
      stopAccount(owner);
      await local.deletion.fence(owner);
    },
    remote: apiAccountDeletion(apiURL, () => sessions.read()?.token ?? null),
    purge: owner => local.deletion.purge(owner),
    async clearSession(owner) {
      // Errors propagate: completion requires durable credential removal.
      await browserSessionState().discardOwner(owner);
    },
  });
  async function review() {
      const current = sessions.read();
      if (!current) throw new Error('account_changed');
      const result = await createSessionApiClient(apiURL, () => current.token).profile();
      if (!result.response.ok || !result.data || sessions.read()?.token !== current.token) throw new Error('account_review_failed');
      const profile = result.data.data;
      if (!profile.id || (current.ownerId && current.ownerId !== profile.id)) throw new Error('account_changed');
      await sessions.write({ ...current, ownerId: profile.id }, current.token);
      return { owner: profile.id, name: profile.displayName };
  }
  return {
    review,
    async confirm(owner: string) { await deletion.confirm(owner); changed(); },
    async resume(owner: string) {
      // A provider callback may have a valid credential without its local owner
      // binding yet. Failed profile reads must not prevent receipt-only cleanup.
      if (sessions.read() && !sessions.read()?.ownerId) await review().catch(() => undefined);
      await deletion.resume(owner); changed();
    },
    async pendingOwners() {
      const owners = await local.deletion.pendingOwners();
      if (owners.length && sessions.read() && !sessions.read()?.ownerId) await review().catch(() => undefined);
      const pending = await deletion.prepareRecovery(owners);
      const current = sessions.read();
      return current && current.destination !== 'home' ? [] : pending;
    },
  };
}
