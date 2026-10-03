import type { SessionRecord, SessionRecordPort } from '../ports/session-record';
export type { SessionRecord } from '../ports/session-record';

/** Revisions survive logout so a delayed response cannot restore old access. */
export class SessionRecordCoordinator {
  constructor(private readonly port: SessionRecordPort, private readonly revision: () => string) {}

  read(): SessionRecord {
    return this.port.read() ?? { revision: '', family: '', value: null };
  }

  commit(expectedRevision: string, value: string | null, current: () => boolean = () => true, newFamily = false): Promise<SessionRecord | null> {
    return this.port.exclusive(() => {
      if (!current() || this.read().revision !== expectedRevision) return null;
      return this.write(value, newFamily ? undefined : this.read().family);
    });
  }

  replace(value: string | null): Promise<SessionRecord> {
    return this.port.exclusive(() => this.write(value));
  }

  discardFamily(family: string, value: string | null): Promise<SessionRecord | null> {
    return this.port.exclusive(() => {
      if (this.read().family !== family) return null;
      return this.write(value);
    });
  }

  private write(value: string | null, family?: string): SessionRecord {
    const revision = this.revision();
    const record = { revision, family: family || revision, value };
    this.port.write(record);
    return record;
  }
}
