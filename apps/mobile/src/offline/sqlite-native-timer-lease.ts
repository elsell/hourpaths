import type { TrackingSQLDatabase } from './sqlite-tracking-store';

/** Public account metadata only: never stores an application/provider token. */
export class SQLiteNativeTimerLease {
  private readonly ready: Promise<void>;
  constructor(private readonly database: TrackingSQLDatabase, private readonly generation: () => string) {
    this.ready = database.execAsync(`CREATE TABLE IF NOT EXISTS account_deletion_fences_v1 (owner TEXT PRIMARY KEY NOT NULL);
      CREATE TABLE IF NOT EXISTS native_timer_account_v1 (
        singleton INTEGER PRIMARY KEY CHECK(singleton = 1), owner TEXT NOT NULL,
        generation TEXT NOT NULL, expires_at REAL NOT NULL
      );
      CREATE TABLE IF NOT EXISTS native_timer_presentation_v1(singleton INTEGER PRIMARY KEY CHECK(singleton = 1), payload TEXT NOT NULL);`);
  }
  async clear(): Promise<void> {
    await this.ready;
    await this.database.withExclusiveTransactionAsync(async transaction => {
      await transaction.runAsync('DELETE FROM native_timer_account_v1');
      await transaction.runAsync('DELETE FROM native_timer_presentation_v1');
    });
  }
  async bind(owner: string, expiresAt: number): Promise<{ owner: string; generation: string }> {
    if (!owner.trim() || !Number.isFinite(expiresAt)) throw new Error('native_timer_lease_invalid');
    await this.ready;
    let generation = this.generation();
    await this.database.withExclusiveTransactionAsync(async transaction => {
      if (await transaction.getFirstAsync('SELECT owner FROM account_deletion_fences_v1 WHERE owner = ?', owner)) {
        throw new Error('account_deletion_pending');
      }
      const previous = await transaction.getFirstAsync<{ owner: string; generation: string }>(
        'SELECT owner, generation FROM native_timer_account_v1 WHERE singleton = 1');
      if (previous?.owner === owner) generation = previous.generation;
      await transaction.runAsync(`INSERT INTO native_timer_account_v1(singleton,owner,generation,expires_at) VALUES(1,?,?,?)
        ON CONFLICT(singleton) DO UPDATE SET owner=excluded.owner,generation=excluded.generation,expires_at=excluded.expires_at`,
      owner, generation, expiresAt);
    });
    return { owner, generation };
  }
}
