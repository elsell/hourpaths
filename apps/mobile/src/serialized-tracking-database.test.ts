import assert from 'node:assert/strict';
import test from 'node:test';
import { DatabaseSync } from 'node:sqlite';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { serializedTrackingDatabase } from './offline/serialized-tracking-database';
import type { TrackingSQLConnection } from './offline/sqlite-tracking-store';

function connection(db: DatabaseSync): TrackingSQLConnection {
  return {
    async getFirstAsync<T>(sql: string, ...params: (string | number)[]) { return (db.prepare(sql).get(...params) ?? null) as T | null; },
    async runAsync(sql: string, ...params: (string | number)[]) { return db.prepare(sql).run(...params); },
  };
}

test('concurrent local writes preserve transactions and recover after rollback', async () => {
  const folder = await mkdtemp(join(tmpdir(), 'hourpaths-contention-'));
  const path = join(folder, 'tracking.db');
  const raw = new DatabaseSync(path);
  const database = serializedTrackingDatabase({
    ...connection(raw),
    async execAsync(sql) { raw.exec(sql); },
    async withExclusiveTransactionAsync(work) {
      // Expo opens a separate connection for each exclusive transaction.
      const transaction = new DatabaseSync(path);
      transaction.exec('BEGIN');
      try { await work(connection(transaction)); transaction.exec('COMMIT'); }
      catch (error) { transaction.exec('ROLLBACK'); throw error; }
      finally { transaction.close(); }
    },
  });
  try {
    await database.execAsync('PRAGMA journal_mode=WAL; CREATE TABLE values_v1(value TEXT NOT NULL)');
    const results = await Promise.allSettled([
      database.withExclusiveTransactionAsync(async transaction => {
        await transaction.runAsync("INSERT INTO values_v1 VALUES ('timer')");
        await new Promise(resolve => setImmediate(resolve));
        await transaction.runAsync("INSERT INTO values_v1 VALUES ('queue')");
      }),
      database.withExclusiveTransactionAsync(async transaction => {
        await transaction.runAsync("INSERT INTO values_v1 VALUES ('lease')");
      }),
      database.runAsync("INSERT INTO values_v1 VALUES ('home')"),
    ]);
    assert.deepEqual(results.map(result => result.status), ['fulfilled', 'fulfilled', 'fulfilled']);
    await assert.rejects(database.withExclusiveTransactionAsync(async transaction => {
      await transaction.runAsync("INSERT INTO values_v1 VALUES ('rolled-back')");
      throw new Error('storage_failure');
    }), /storage_failure/);
    await database.runAsync("INSERT INTO values_v1 VALUES ('recovered')");
    assert.equal((await database.getFirstAsync<{ count: number }>('SELECT count(*) AS count FROM values_v1'))?.count, 5);
  } finally { raw.close(); await rm(folder, { recursive: true, force: true }); }
});
