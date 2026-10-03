import * as Crypto from 'expo-crypto';
import { AccountDeletion, apiAccountDeletion } from '@hourpaths/client-core';
import type { MobileSession, SerializedMobileSessionStorage } from '../session-state';
import { deletionSurfaceJournal } from '../account-deletion-journal';
import { openNativeOfflineStorage } from './native-tracking-store';

export function nativeAccountDeletion(ports: {
  apiURL: string;
  currentSession(): MobileSession | null;
  sessions: SerializedMobileSessionStorage;
  stopAccount(owner: string): void;
  captureSurfaces(owner: string): Promise<string[]>;
  clearSurfaces(owner: string, identifiers: string[]): Promise<void>;
  clearMemory(owner: string): void;
  completed(): void | Promise<void>;
}) {
  let opening: Promise<AccountDeletion> | undefined;
  const coordinator = () => opening ??= openNativeOfflineStorage().then(local => new AccountDeletion({
    currentOwner: () => ports.currentSession()?.ownerId ?? null,
    newSecret: () => Array.from(Crypto.getRandomBytes(32), value => value.toString(16).padStart(2, '0')).join(''),
    journal: deletionSurfaceJournal({
      currentOwner: () => ports.currentSession()?.ownerId ?? null,
      capture: ports.captureSurfaces,
      local: local.deletion,
    }),
    async fence(owner) {
      ports.stopAccount(owner);
      await local.deletion.fence(owner);
    },
    remote: apiAccountDeletion(ports.apiURL, () => ports.currentSession()?.token ?? null),
    async purge(owner) {
      const identifiers = await local.deletion.readSurfaces(owner);
      if (!identifiers) throw new Error('deletion_surface_snapshot_missing');
      await ports.clearSurfaces(owner, identifiers);
      await local.deletion.purge(owner);
    },
    async clearSession(owner) {
      await ports.sessions.discardOwner(owner);
      ports.clearMemory(owner);
    },
  })).catch(error => { opening = undefined; throw error; });
  return {
    async confirm(owner: string) { await (await coordinator()).confirm(owner); await ports.completed(); },
    async resume(owner: string) { await (await coordinator()).resume(owner); await ports.completed(); },
    async pendingOwners() { return (await openNativeOfflineStorage()).deletion.pendingOwners(); },
  };
}
