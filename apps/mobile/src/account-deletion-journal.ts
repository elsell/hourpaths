import type { AccountDeletionPorts } from '@hourpaths/client-core';

type Journal = AccountDeletionPorts['journal'];

/** Recovery must have native cleanup identifiers even if the credential expires. */
export function deletionSurfaceJournal(ports: {
  currentOwner(): string | null;
  capture(owner: string): Promise<string[]>;
  local: Journal & {
    readSurfaces(owner: string): Promise<string[] | null>;
    saveSurfaces(owner: string, identifiers: string[]): Promise<void>;
  };
}): Journal {
  return {
    read: owner => ports.local.read(owner),
    forget: owner => ports.local.forget(owner),
    async save(intent) {
      if (await ports.local.readSurfaces(intent.owner) === null) {
        if (ports.currentOwner() !== intent.owner) throw new Error('account_changed');
        const identifiers = await ports.capture(intent.owner);
        if (ports.currentOwner() !== intent.owner) throw new Error('account_changed');
        await ports.local.saveSurfaces(intent.owner, identifiers);
      }
      return ports.local.save(intent);
    },
  };
}
