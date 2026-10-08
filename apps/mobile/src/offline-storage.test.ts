import { SQLiteAccountDeletion } from './offline/sqlite-account-deletion';
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
  return { connection, deletion: new SQLiteAccountDeletion(database), appearances: new SQLiteAppearanceCache(database), store: new SQLiteTrackingStore(database), home: new SQLiteMobileHomeCache(database), close: () => db.close() };
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


test('deletion intent survives restart, isolates owners, and fences late writes after cleanup', async () => {
  const folder = await mkdtemp(join(tmpdir(), 'hourpaths-deletion-'));
  let database = open(join(folder, 'tracking.db'));
  try {
    const profile: MobileHomeProfile = {
      id: 'alice', email: 'alice@example.test', displayName: 'Alice', profileVisibility: 'private',
      paths: [], archivedPaths: [], timers: {}, pendingInvitations: { items: [], nextCursor: '' },
      homePreferences: { revision: 0, orderMethod: 'manual', manualPathIds: [], pinnedPathIds: [] },
    };
    const home = { owner: 'alice', timeZone: 'utc', profile };
    const core = new OfflineTracking(database.store, 'alice', () => 1000, () => 'operation');
    await core.retainPaths([{ id: 'guitar', name: 'guitar', timeZone: 'utc' }]);
    const snapshot = await core.snapshot();
    await database.home.saveHome(home);
    await database.appearances.write('alice', 'guitar', { revision: 1, color: 'mint', emoji: '🎸' });
    await database.appearances.write('bob', 'guitar', { revision: 1, color: 'gold', emoji: '✨' });
    const intent = { owner: 'alice', receiptSecret: 'a'.repeat(64), phase: 'pending' as const };
    await database.deletion.save(intent);
    await database.deletion.saveSurfaces('alice', ['notice-a']);
    await database.deletion.saveSurfaces('alice', ['notice-a', 'notice-b']);
    assert.equal(await database.store.read('alice'), null);
    assert.equal(await database.home.readHome('alice'), null);
    assert.equal(await database.appearances.read('alice', 'guitar'), null);
    await assert.rejects(database.home.saveHome(home), /account_deletion_pending/);
    database.close(); database = open(join(folder, 'tracking.db'));
    assert.deepEqual(await database.deletion.read('alice'), intent);
    assert.deepEqual(await database.deletion.readSurfaces('alice'), ['notice-a', 'notice-b']);
    assert.deepEqual(await database.deletion.pendingOwners(), ['alice']);
    assert.deepEqual(await database.deletion.save({ ...intent, receiptSecret: 'b'.repeat(64) }), intent);
    const confirmed = { ...intent, phase: 'confirmed' as const };
    await database.deletion.save(confirmed);
    assert.deepEqual(await database.deletion.save(intent), confirmed);
    await database.deletion.purge('alice');
    assert.deepEqual(await database.deletion.readSurfaces('alice'), ['notice-a', 'notice-b']);
    await database.deletion.forget('alice');
    for (const table of ['tracking_accounts_v1', 'tracking_home_v1', 'tracking_appearances_v1']) {
      assert.equal((await database.connection.getFirstAsync<{ count: number }>(`SELECT COUNT(*) AS count FROM ${table} WHERE owner = ?`, 'alice'))?.count, 0);
    }
    database.close(); database = open(join(folder, 'tracking.db'));
    assert.deepEqual(await database.deletion.pendingOwners(), []);
    assert.equal(await database.deletion.read('alice'), null);
    assert.equal(await database.deletion.readSurfaces('alice'), null);
    assert.equal(await database.deletion.isFenced('alice'), true);
    assert.equal(await database.deletion.isFenced('bob'), false);
    await assert.rejects(database.store.commit('alice', 0, { ...snapshot, revision: 1 }), /account_deletion_pending/);
    await assert.rejects(database.home.saveHome(home), /account_deletion_pending/);
    await assert.rejects(database.appearances.write('alice', 'guitar', { revision: 2, color: 'mint', emoji: '🎸' }), /account_deletion_pending/);
    assert.equal((await database.appearances.read('bob', 'guitar'))?.color, 'gold');
  } finally { database.close(); await rm(folder, { recursive: true, force: true }); }
});
