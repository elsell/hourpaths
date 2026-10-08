import { effectivePathCapabilities } from '@hourpaths/client-core';
import { admitMobileSessionPaths } from '../session-destination';
import { homePreferencesFromAPI } from '../home-preference-operation';
import type { MobileHomeCache, RetainedMobileHome } from './mobile-home-cache';
import type { TrackingSQLDatabase } from './sqlite-tracking-store';
import { deletionFenceSchema, visibleAccountSQL } from './sqlite-deletion-fence';

function validate(value: RetainedMobileHome, owner: string): void {
  if (!owner.trim() || value.owner !== owner || value.profile?.id !== owner) throw new Error('tracking_owner_mismatch');
  if (!value.timeZone || typeof value.profile.displayName !== 'string' || typeof value.profile.email !== 'string'
    || !['public', 'private'].includes(value.profile.profileVisibility)) throw new Error('tracking_home_invalid');
  new Intl.DateTimeFormat('en', { timeZone: value.timeZone });
  admitMobileSessionPaths(value.profile.paths, value.profile.archivedPaths);
  homePreferencesFromAPI(value.profile.homePreferences);
}

/** Presentation metadata is evictable independently of the authoritative
 * timer/operation ledger. No credential is stored in this database. */
export class SQLiteMobileHomeCache implements MobileHomeCache {
  private readonly ready: Promise<void>;
  constructor(private readonly database: TrackingSQLDatabase) {
    this.ready = database.execAsync(`CREATE TABLE IF NOT EXISTS tracking_home_v1 (
      owner TEXT PRIMARY KEY NOT NULL, payload TEXT NOT NULL
    ); ${deletionFenceSchema('tracking_home_v1')}`);
  }
  async readHome(owner: string): Promise<RetainedMobileHome | null> {
    if (!owner.trim()) throw new Error('tracking_owner_required');
    await this.ready;
    const row = await this.database.getFirstAsync<{ payload: string }>(`SELECT payload FROM tracking_home_v1 WHERE owner = ? AND ${visibleAccountSQL()}`, owner);
    if (!row) return null;
    const value = JSON.parse(row.payload) as RetainedMobileHome;
    validate(value, owner);
    return value;
  }
  async saveHome(value: RetainedMobileHome): Promise<void> {
    validate(value, value.owner);
    const retained: RetainedMobileHome = { ...value, profile: {
      ...value.profile,
      paths: value.profile.paths.filter(path => !path.archivedAt && effectivePathCapabilities(path).trackTime),
      archivedPaths: [], pendingInvitations: { items: [], nextCursor: '' }, timers: {},
    } };
    const payload = JSON.stringify(retained);
    await this.ready;
    await this.database.runAsync(`INSERT INTO tracking_home_v1(owner, payload) VALUES (?, ?)
      ON CONFLICT(owner) DO UPDATE SET payload = excluded.payload`, value.owner, payload);
  }
}
