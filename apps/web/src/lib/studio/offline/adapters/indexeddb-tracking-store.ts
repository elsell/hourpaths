import type { CachedHome, HomeCache } from '../ports/home-cache';
import type { Path, PathAppearance } from '../../paths/domain/path';
import type { TrackingSnapshot, TrackingStore } from '@hourpaths/client-core';

/** Transaction completion, rather than request success, is the save boundary. */
export class IndexedDBTrackingStore implements TrackingStore, HomeCache {
  private database: Promise<IDBDatabase> | null = null;
  constructor(private readonly name = 'hourpaths-durable-tracking') {}
  private open(): Promise<IDBDatabase> {
    if (!this.database) {
      this.database = new Promise((resolve, reject) => {
        const request = indexedDB.open(this.name, 2);
        request.onupgradeneeded = () => {
          const database = request.result;
          if (!database.objectStoreNames.contains('accounts')) database.createObjectStore('accounts', { keyPath: 'owner' });
          if (!database.objectStoreNames.contains('home')) database.createObjectStore('home', { keyPath: 'owner' });
        };
        request.onerror = () => { this.database = null; reject(request.error); };
        request.onsuccess = () => {
          const database = request.result;
          database.onversionchange = () => { database.close(); this.database = null; };
          resolve(database);
        };
      });
    }
    return this.database;
  }
  async read(owner: string): Promise<TrackingSnapshot | null> {
    if (!owner.trim()) throw new Error('tracking_owner_required');
    const database = await this.open();
    return new Promise((resolve, reject) => {
      const transaction = database.transaction('accounts', 'readonly');
      const request = transaction.objectStore('accounts').get(owner);
      transaction.oncomplete = () => {
        const value = request.result as TrackingSnapshot | undefined;
        if (value && (value.owner !== owner || !Number.isSafeInteger(value.revision))) {
          reject(new Error('tracking_storage_invalid'));
        } else resolve(value ?? null);
      };
      transaction.onabort = () => reject(transaction.error ?? new Error('tracking_storage_aborted'));
      transaction.onerror = () => reject(transaction.error ?? new Error('tracking_storage_failed'));
    });
  }
  async commit(owner: string, revision: number, snapshot: TrackingSnapshot): Promise<boolean> {
    if (!owner.trim() || snapshot.owner !== owner || !Number.isSafeInteger(revision)
      || revision < 0 || snapshot.revision !== revision + 1) throw new Error('tracking_storage_invalid');
    const database = await this.open();
    return new Promise((resolve, reject) => {
      const transaction = database.transaction('accounts', 'readwrite', { durability: 'strict' });
      const accounts = transaction.objectStore('accounts');
      const request = accounts.get(owner);
      let committed = false;
      request.onsuccess = () => {
        const current = request.result as TrackingSnapshot | undefined;
        if ((current?.revision ?? 0) !== revision) return;
        accounts.put(snapshot);
        committed = true;
      };
      transaction.oncomplete = () => resolve(committed);
      transaction.onabort = () => reject(transaction.error ?? new Error('tracking_storage_aborted'));
      transaction.onerror = () => reject(transaction.error ?? new Error('tracking_storage_failed'));
    });
  }
  async readHome(owner: string): Promise<CachedHome | null> {
    if (!owner.trim()) throw new Error('tracking_owner_required');
    const database = await this.open();
    return new Promise((resolve, reject) => {
      const transaction = database.transaction('home', 'readonly');
      const request = transaction.objectStore('home').get(owner);
      transaction.oncomplete = () => {
        const value = request.result as CachedHome | undefined;
        if (value && (value.owner !== owner || !Array.isArray(value.paths) || !value.appearances)) {
          reject(new Error('tracking_storage_invalid'));
        } else resolve(value ?? null);
      };
      transaction.onabort = () => reject(transaction.error ?? new Error('tracking_storage_aborted'));
      transaction.onerror = () => reject(transaction.error ?? new Error('tracking_storage_failed'));
    });
  }
  private async updateHome(owner: string, update: (home: CachedHome) => void): Promise<void> {
    if (!owner.trim()) throw new Error('tracking_owner_required');
    const database = await this.open();
    return new Promise((resolve, reject) => {
      const transaction = database.transaction('home', 'readwrite', { durability: 'strict' });
      const homes = transaction.objectStore('home');
      const request = homes.get(owner);
      request.onsuccess = () => {
        const value = request.result as CachedHome | undefined;
        if (value && value.owner !== owner) { transaction.abort(); return; }
        const home = value ?? { owner, paths: [], appearances: {} };
        update(home);
        homes.put(home);
      };
      transaction.oncomplete = () => resolve();
      transaction.onabort = () => reject(transaction.error ?? new Error('tracking_storage_aborted'));
      transaction.onerror = () => reject(transaction.error ?? new Error('tracking_storage_failed'));
    });
  }
  saveHome(owner: string, paths: readonly Path[], appearances: Record<string, PathAppearance>): Promise<void> {
    const retained = JSON.parse(JSON.stringify({ paths, appearances })) as Pick<CachedHome, 'paths' | 'appearances'>;
    return this.updateHome(owner, home => { home.paths = retained.paths; home.appearances = retained.appearances; });
  }
  savePaths(owner: string, paths: readonly Path[]): Promise<void> {
    const retained = JSON.parse(JSON.stringify(paths)) as Path[];
    return this.updateHome(owner, home => {
      home.paths = retained;
      const visible = new Set(retained.map(path => path.id));
      home.appearances = Object.fromEntries(Object.entries(home.appearances).filter(([id]) => visible.has(id)));
    });
  }
  saveAppearance(owner: string, pathId: string, appearance: PathAppearance): Promise<void> {
    const retained = { ...appearance };
    return this.updateHome(owner, home => { home.appearances = { ...home.appearances, [pathId]: retained }; });
  }
  async close(): Promise<void> {
    const database = this.database;
    this.database = null;
    if (database) (await database).close();
  }
}
