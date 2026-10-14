import type { TrackingSQLDatabase } from './sqlite-tracking-store';

/** Expo exclusive transactions use separate connections, not a shared writer
 * queue. Keep all in-process stores behind one queue; transaction statements
 * use their supplied connection directly and retain their atomic boundary. */
export function serializedTrackingDatabase(database: TrackingSQLDatabase): TrackingSQLDatabase {
  let tail: Promise<unknown> = Promise.resolve();
  function enqueue<T>(work: () => Promise<T>): Promise<T> {
    const result = tail.then(work);
    tail = result.catch(() => undefined);
    return result;
  }
  return {
    execAsync: sql => enqueue(() => database.execAsync(sql)),
    getFirstAsync: <T>(sql: string, ...params: (string | number)[]) => enqueue(() => database.getFirstAsync<T>(sql, ...params)),
    runAsync: (sql, ...params) => enqueue(() => database.runAsync(sql, ...params)),
    withExclusiveTransactionAsync: work => enqueue(() => database.withExclusiveTransactionAsync(work)),
  };
}
