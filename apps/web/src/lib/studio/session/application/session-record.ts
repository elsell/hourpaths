import type { SessionRecord, SessionRecordPort } from '../ports/session-record';
export type { SessionRecord } from '../ports/session-record';

/** Revisions survive logout so a delayed response cannot restore old access. */
export class SessionRecordCoordinator {
  constructor(private readonly port: SessionRecordPort, private readonly revision: () => string) {}

  read(): SessionRecord {
    return this.port.read() ?? { revision: '', value: null };
  }

  commit(expectedRevision: string, value: string | null, current: () => boolean = () => true): Promise<SessionRecord | null> {
    return this.port.exclusive(() => {
      if (!current() || this.read().revision !== expectedRevision) return null;
      return this.write(value);
    });
  }

  replace(value: string | null): Promise<SessionRecord> {
    return this.port.exclusive(() => this.write(value));
  }

  private write(value: string | null): SessionRecord {
    const record = { revision: this.revision(), value };
    this.port.write(record);
    return record;
  }
}
