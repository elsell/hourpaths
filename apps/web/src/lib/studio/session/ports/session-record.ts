export interface SessionRecord {
  revision: string;
  family: string;
  value: string | null;
}

/** Shared per origin. The lock covers both the comparison and the write. */
export interface SessionRecordPort {
  read(): SessionRecord | null;
  write(record: SessionRecord): void;
  exclusive<T>(operation: () => T): Promise<T>;
}
