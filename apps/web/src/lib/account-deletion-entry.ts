const key = 'hourpaths_account_deletion_entry';

/** A fixed destination, never an arbitrary return URL from query parameters. */
export function requestAccountDeletionEntry(storage: Pick<Storage, 'setItem'>): void {
  storage.setItem(key, '1');
}
export function consumeAccountDeletionEntry(storage: Pick<Storage, 'getItem' | 'removeItem'>): boolean {
  if (storage.getItem(key) !== '1') return false;
  storage.removeItem(key);
  return true;
}

/** Capture only an explicit deletion entry at the point sign-in actually starts. */
export function prepareAccountDeletionSignIn(storage: Pick<Storage, 'setItem' | 'removeItem'>, pathname: string): void {
  if (pathname === '/studio/delete-account' || pathname === '/studio/delete-account/' || pathname === '/delete-account') requestAccountDeletionEntry(storage);
  else storage.removeItem(key);
}
