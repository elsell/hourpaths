import { Platform, Share } from 'react-native';
import { cacheDirectory, makeDirectoryAsync, writeAsStringAsync, deleteAsync, StorageAccessFramework } from 'expo-file-system/legacy';
import { randomUUID } from 'expo-crypto';
import { AccountExportFailure, type AccountExportSink } from '@hourpaths/client-core';

let saving = false;
const directory = cacheDirectory ? cacheDirectory + 'hourpaths-exports/' : null;

/** Remove abandoned share copies on startup and after account cleanup. Files
 * explicitly saved outside the app remain under the user's control. */
export async function clearNativeAccountExportCache() {
  if (directory && !saving) await deleteAsync(directory, { idempotent: true });
}
export const nativeAccountExportJSON: AccountExportSink = {
  async save(document, current) {
    if (saving) throw new AccountExportFailure();
    if (!current()) throw new AccountExportFailure('account_changed');
    saving = true;
    let temporary: string | null = null;
    let createdDocument: string | null = null;
    try {
      const contents = JSON.stringify(document, null, 2) + '\n';
      const filename = 'hourpaths-export-' + new Date(document.collectionCompletedAt).toISOString().replace(/[^0-9]/g, '') + '.json';
      if (Platform.OS === 'android') {
        const permission = await StorageAccessFramework.requestDirectoryPermissionsAsync();
        if (!current()) throw new AccountExportFailure('account_changed');
        if (!permission.granted) return 'cancelled';
        createdDocument = await StorageAccessFramework.createFileAsync(permission.directoryUri, filename, 'application/json');
        if (!current()) throw new AccountExportFailure('account_changed');
        await StorageAccessFramework.writeAsStringAsync(createdDocument, contents);
        if (!current()) throw new AccountExportFailure('account_changed');
        createdDocument = null;
        return 'saved';
      }
      if (Platform.OS !== 'ios' || !directory) throw new AccountExportFailure();
      await deleteAsync(directory, { idempotent: true });
      await makeDirectoryAsync(directory, { intermediates: true });
      if (!current()) throw new AccountExportFailure('account_changed');
      temporary = directory + randomUUID() + '-' + filename;
      await writeAsStringAsync(temporary, contents);
      if (!current()) throw new AccountExportFailure('account_changed');
      const result = await Share.share({ url: temporary });
      return result.action === Share.dismissedAction ? 'cancelled' : 'saved';
    } finally {
      try {
        if (createdDocument) await deleteAsync(createdDocument, { idempotent: true });
        if (temporary) await deleteAsync(temporary, { idempotent: true });
      } finally { saving = false; }
    }
  },
};
