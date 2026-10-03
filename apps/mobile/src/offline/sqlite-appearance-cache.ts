import { parsePathAppearance, type SavedPathAppearance } from '@hourpaths/client-core';
import type { TrackingSQLDatabase } from './sqlite-tracking-store';
import { deletionFenceSchema, visibleAccountSQL } from './sqlite-deletion-fence';

/** Personal appearance data stays partitioned by account even on shared devices. */
export class SQLiteAppearanceCache {
  private readonly ready: Promise<void>;
  constructor(private readonly database: TrackingSQLDatabase) {
    this.ready = database.execAsync(`CREATE TABLE IF NOT EXISTS tracking_appearances_v1 (
      owner TEXT NOT NULL, path_id TEXT NOT NULL, revision INTEGER NOT NULL, payload TEXT NOT NULL,
      PRIMARY KEY(owner, path_id)
    ); ${deletionFenceSchema('tracking_appearances_v1')}`);
  }
  private requireIdentity(owner: string, pathId: string) {
    if (!owner.trim() || owner.trim() !== owner || !pathId.trim() || pathId.trim() !== pathId) throw new Error('tracking_owner_required');
  }
  async read(owner: string, pathId: string): Promise<SavedPathAppearance | null> {
    this.requireIdentity(owner, pathId); await this.ready;
    const row = await this.database.getFirstAsync<{ revision: number; payload: string }>(
      `SELECT revision, payload FROM tracking_appearances_v1 WHERE owner = ? AND path_id = ? AND ${visibleAccountSQL()}`, owner, pathId);
    if (!row) return null;
    const value = parsePathAppearance(JSON.parse(row.payload));
    if (row.revision !== value.revision) throw new Error('tracking_storage_invalid');
    return value;
  }
  async write(owner: string, pathId: string, appearance: SavedPathAppearance): Promise<void> {
    this.requireIdentity(owner, pathId);
    const value = parsePathAppearance(appearance); await this.ready;
    await this.database.runAsync(`INSERT INTO tracking_appearances_v1(owner, path_id, revision, payload) VALUES (?, ?, ?, ?)
      ON CONFLICT(owner, path_id) DO UPDATE SET revision = excluded.revision, payload = excluded.payload
      WHERE excluded.revision >= tracking_appearances_v1.revision`, owner, pathId, value.revision, JSON.stringify(value));
  }
}
