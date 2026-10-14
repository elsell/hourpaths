import { serializedTrackingDatabase } from './serialized-tracking-database';
import { randomUUID } from 'expo-crypto';
import { SQLiteNativeTimerLease } from './sqlite-native-timer-lease';
import { openDatabaseAsync } from 'expo-sqlite';
import { SQLiteAppearanceCache } from './sqlite-appearance-cache';
import { SQLiteMobileHomeCache } from './sqlite-mobile-home-cache';
import { SQLiteTrackingStore } from './sqlite-tracking-store';
import { SQLiteAccountDeletion } from './sqlite-account-deletion';

async function createStorage() {
  const nativeDatabase = await openDatabaseAsync('hourpaths-tracking.db');
  const database = serializedTrackingDatabase(nativeDatabase);
  return { databasePath: nativeDatabase.databasePath, nativeLease: new SQLiteNativeTimerLease(database, randomUUID), tracking: new SQLiteTrackingStore(database), home: new SQLiteMobileHomeCache(database), appearances: new SQLiteAppearanceCache(database), deletion: new SQLiteAccountDeletion(database) };
}
let opening: ReturnType<typeof createStorage> | null = null;
export function openNativeOfflineStorage() {
  if (!opening) opening = createStorage().catch(error => { opening = null; throw error; });
  return opening;
}
export async function openNativeTrackingStore(): Promise<SQLiteTrackingStore> {
  return (await openNativeOfflineStorage()).tracking;
}
