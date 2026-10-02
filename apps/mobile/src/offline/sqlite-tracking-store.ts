import type { TrackingSnapshot, TrackingStore } from '@hourpaths/client-core';

export interface TrackingSQLConnection {
  getFirstAsync<T>(sql: string, ...params: (string | number)[]): Promise<T | null>;
  runAsync(sql: string, ...params: (string | number)[]): Promise<unknown>;
}
export interface TrackingSQLDatabase extends TrackingSQLConnection {
  execAsync(sql: string): Promise<void>;
  withExclusiveTransactionAsync(task: (transaction: TrackingSQLConnection) => Promise<void>): Promise<void>;
}
export class SQLiteTrackingStore implements TrackingStore {
  private readonly ready: Promise<void>;
  constructor(private readonly database: TrackingSQLDatabase) {
    this.ready = database.execAsync(`PRAGMA journal_mode = WAL;
      PRAGMA synchronous = FULL;
      CREATE TABLE IF NOT EXISTS tracking_accounts_v1 (
        owner TEXT PRIMARY KEY NOT NULL, revision INTEGER NOT NULL, payload TEXT NOT NULL
      );`);
  }
  async read(owner: string): Promise<TrackingSnapshot | null> {
    if (!owner.trim()) throw new Error('tracking_owner_required');
    await this.ready;
    const row = await this.database.getFirstAsync<{ payload: string; revision: number }>(
      'SELECT payload, revision FROM tracking_accounts_v1 WHERE owner = ?', owner);
    if (!row) return null;
    const snapshot = JSON.parse(row.payload) as TrackingSnapshot;
    if (snapshot.owner !== owner || snapshot.revision !== row.revision || !Number.isSafeInteger(row.revision)) {
      throw new Error('tracking_storage_invalid');
    }
    return snapshot;
  }
  async commit(owner: string, revision: number, snapshot: TrackingSnapshot): Promise<boolean> {
    if (!owner.trim() || snapshot.owner !== owner || !Number.isSafeInteger(revision)
      || revision < 0 || snapshot.revision !== revision + 1) throw new Error('tracking_storage_invalid');
    await this.ready;
    // Serialize before opening the transaction so the stored payload is fixed.
    const payload = JSON.stringify(snapshot);
    let committed = false;
    await this.database.withExclusiveTransactionAsync(async transaction => {
      const current = await transaction.getFirstAsync<{ revision: number }>(
        'SELECT revision FROM tracking_accounts_v1 WHERE owner = ?', owner);
      if ((current?.revision ?? 0) !== revision) return;
      await transaction.runAsync(`INSERT INTO tracking_accounts_v1(owner, revision, payload) VALUES (?, ?, ?)
        ON CONFLICT(owner) DO UPDATE SET revision = excluded.revision, payload = excluded.payload`,
      owner, snapshot.revision, payload);
      committed = true;
    });
    return committed;
  }
}
