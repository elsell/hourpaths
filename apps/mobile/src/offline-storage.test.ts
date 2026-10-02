import { SQLiteAppearanceCache } from './offline/sqlite-appearance-cache';
import assert from 'node:assert/strict';
import test from 'node:test';
import { DatabaseSync } from 'node:sqlite';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { OfflineTracking } from '@hourpaths/client-core';
import { SQLiteMobileHomeCache } from './offline/sqlite-mobile-home-cache';
import type { MobileHomeProfile } from './session-destination';
import { SQLiteTrackingStore, type TrackingSQLDatabase } from './offline/sqlite-tracking-store';

function open(path: string) {
  const db = new DatabaseSync(path);
  const connection = {
    async getFirstAsync<T>(sql: string, ...params: (string | number)[]) { return (db.prepare(sql).get(...params) ?? null) as T | null; },
    async runAsync(sql: string, ...params: (string | number)[]) { return db.prepare(sql).run(...params); },
  };
  const database: TrackingSQLDatabase = {
    ...connection,
    async execAsync(sql) { db.exec(sql); },
    async withExclusiveTransactionAsync(work) {
      db.exec('BEGIN IMMEDIATE');
      try {
        await work(connection);
        db.exec('COMMIT');
      } catch (error) {
        db.exec('ROLLBACK');
        throw error;
      }
    },
  };
  return { appearances: new SQLiteAppearanceCache(database), store: new SQLiteTrackingStore(database), home: new SQLiteMobileHomeCache(database), close: () => db.close() };
}

test('real SQLite reopen preserves pending timers and rejects stale account revisions', async () => {
  const folder = await mkdtemp(join(tmpdir(), 'hourpaths-storage-'));
  let database = open(join(folder, 'tracking.db'));
  try {
    let sequence = 0;
    const core = new OfflineTracking(database.store, 'alice', () => Date.parse('2026-10-01T12:00:00Z'), () => `id-${++sequence}`);
    await core.retainPaths([{
      id: 'path',
      name: 'path',
      timeZone: 'Etc/UTC',
    }]);
    const timer = await core.start('path');
    const before = await core.snapshot();
    database.close();
    database = open(join(folder, 'tracking.db'));
    const after = await database.store.read('alice');
    assert.equal(after?.timers[0].id, timer.id);
    assert.deepEqual(after?.operations, before.operations);
    assert.equal(await database.store.read('bob'), null);
    assert.equal(await database.store.commit('alice', before.revision - 1, { ...before }), false);
    await assert.rejects(database.store.commit('bob', before.revision, { ...before, revision: before.revision + 1 }), /tracking_storage_invalid/);
  } finally { database.close(); await rm(folder, { recursive: true, force: true }); }
});


test('native Home cache survives reopening and cannot cross an account boundary', async () => {
  const folder = await mkdtemp(join(tmpdir(), 'hourpaths-home-storage-'));
  let database = open(join(folder, 'tracking.db'));
  try {
    const profile: MobileHomeProfile = {
      id: 'alice', email: 'alice@example.test', displayName: 'alice', profileVisibility: 'private',
      paths: [], archivedPaths: [], timers: {}, pendingInvitations: { items: [], nextCursor: '' },
      homePreferences: { revision: 0, orderMethod: 'manual', manualPathIds: [], pinnedPathIds: [] },
    };
    await database.home.saveHome({ owner: 'alice', timeZone: 'utc', profile });
    database.close(); database = open(join(folder, 'tracking.db'));
    assert.equal((await database.home.readHome('alice'))?.profile.id, 'alice');
    assert.equal(await database.home.readHome('bob'), null);
    await assert.rejects(database.home.saveHome({ owner: 'bob', timeZone: 'utc', profile }), /owner/);
    assert.equal(await database.home.readHome('bob'), null);
  } finally { database.close(); await rm(folder, { recursive: true, force: true }); }
});


test('personal appearance cache survives SQLite reopen and rejects stale revisions and cross-account reads', async () => {
  const folder = await mkdtemp(join(tmpdir(), 'hourpaths-appearance-'));
  let database = open(join(folder, 'tracking.db'));
  try {
    await database.appearances.write('alice', 'guitar', { revision: 4, color: 'mint', emoji: '🎸' });
    await database.appearances.write('alice', 'guitar', { revision: 3, color: 'gold', emoji: '✨' });
    database.close(); database = open(join(folder, 'tracking.db'));
    assert.deepEqual(await database.appearances.read('alice', 'guitar'), { revision: 4, color: 'mint', emoji: '🎸' });
    assert.equal(await database.appearances.read('bob', 'guitar'), null);
    await database.appearances.write('bob', 'guitar', { revision: 1, color: 'pink', emoji: '💯' });
    assert.equal((await database.appearances.read('alice', 'guitar'))?.color, 'mint');
  } finally { database.close(); await rm(folder, { recursive: true, force: true }); }
});
