import { openDatabaseAsync } from 'expo-sqlite';
import { SQLiteAppearanceCache } from './sqlite-appearance-cache';
import { SQLiteMobileHomeCache } from './sqlite-mobile-home-cache';
import { SQLiteTrackingStore } from './sqlite-tracking-store';

async function createStorage() {
  const database = await openDatabaseAsync('hourpaths-tracking.db');
  return { tracking: new SQLiteTrackingStore(database), home: new SQLiteMobileHomeCache(database), appearances: new SQLiteAppearanceCache(database) };
}
let opening: ReturnType<typeof createStorage> | null = null;
export function openNativeOfflineStorage() {
  if (!opening) opening = createStorage().catch(error => { opening = null; throw error; });
  return opening;
}
export async function openNativeTrackingStore(): Promise<SQLiteTrackingStore> {
  return (await openNativeOfflineStorage()).tracking;
}
