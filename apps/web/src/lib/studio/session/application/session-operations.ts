import type { SessionRecordCoordinator } from './session-record';

export interface DurableSessionTicket {
  current(): boolean;
  persist(value: string): Promise<boolean>;
}

/** Both the local operation and the durable origin revision must still own a
 * response. A successful write advances its ticket for callback navigation. */
export class DurableSessionOperations {
  private generation = 0;
  constructor(private readonly records: SessionRecordCoordinator) {}

  invalidate(): void { this.generation++; }

  issue(expectedRevision = this.records.read().revision): DurableSessionTicket {
    const generation = ++this.generation;
    let revision = expectedRevision;
    const current = () => {
      try { return generation === this.generation && revision === this.records.read().revision; }
      catch { return false; }
    };
    return {
      current,
      persist: async value => {
        const record = await this.records.commit(revision, value, current);
        if (!record) return false;
        revision = record.revision;
        return current();
      },
    };
  }
}
