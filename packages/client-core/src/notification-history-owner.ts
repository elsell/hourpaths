import { applyNotificationMutation, mergeNotificationHistoryPage, type NotificationHistoryState, type NotificationMutation } from './notification-history';
export interface NotificationHistoryPort {
  page(cursor: string): Promise<unknown>;
  mutate(mutation: NotificationMutation): Promise<unknown>;
  changed(): void;
  cancel(): void;
}
export interface NotificationHistorySnapshot {
  readonly history: NotificationHistoryState;
  readonly loading: boolean;
  readonly mutating: boolean;
  readonly error: 'history' | 'mutation' | null;
  readonly loaded: boolean;
}
export function createNotificationHistoryOwner(port: NotificationHistoryPort) {
  const empty = (): NotificationHistoryState => ({ items: [], nextCursor: '', unreadCount: 0 });
  let state: NotificationHistorySnapshot = { history: empty(), loading: false, mutating: false, error: null, loaded: false };
  const listeners = new Set<() => void>(), cursors = new Set<string>();
  let active = true, working = false, requested = false, generation = 0;
  let completion: { promise: Promise<void>; resolve(): void } | null = null;
  const publish = (value: Partial<NotificationHistorySnapshot>) => { if (!active) return; state = { ...state, ...value }; listeners.forEach(listener => listener()); };
  const settle = () => { const current = completion; completion = null; current?.resolve(); };
  async function drain() {
    if (working || !active) return;
    working = true;
    try {
      while (active && requested) {
        requested = false; const ownedGeneration = generation; publish({ loading: true, error: null });
        try {
          const value = await port.page('');
          if (!active) return;
          if (generation !== ownedGeneration) continue;
          const history = mergeNotificationHistoryPage(empty(), value, '');
          if (history.nextCursor && !history.items.length) throw Error('invalid_notification_cursor');
          cursors.clear(); publish({ history, loaded: true });
        } catch { if (generation === ownedGeneration) publish({ error: 'history' }); }
      }
    } finally { working = false; publish({ loading: false }); settle(); }
  }
  const refresh = () => {
    if (!active) return Promise.resolve();
    requested = true;
    if (!completion) {
      let resolve!: () => void;
      const promise = new Promise<void>(done => { resolve = done; });
      completion = { promise, resolve };
    }
    const result = completion.promise;
    void drain(); return result;
  };
  return {
    snapshot: () => state,
    subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    refresh,
    invalidate() {
      if (!active) return Promise.resolve();
      generation++; cursors.clear(); publish({ history: empty(), loaded: false, error: null });
      port.cancel(); return refresh();
    },
    async loadMore() {
      const cursor = state.history.nextCursor;
      if (!active || working || !cursor) return;
      working = true; const ownedGeneration = generation; publish({ loading: true, error: null });
      try {
        const page = await port.page(cursor);
        if (!active || generation !== ownedGeneration) return;
        const history = mergeNotificationHistoryPage(state.history, page, cursor);
        if (history.nextCursor && (history.nextCursor === cursor || cursors.has(history.nextCursor) || history.items.length <= state.history.items.length)) throw Error('invalid_notification_cursor');
        cursors.add(cursor); publish({ history });
      } catch { if (generation === ownedGeneration) publish({ error: 'history' }); }
      finally { working = false; publish({ loading: false }); if (requested) void drain(); }
    },
    async mutate(mutation: NotificationMutation): Promise<boolean> {
      if (!active || working || (mutation.kind !== 'read-all' && !state.history.items.some(item => item.id === mutation.notificationId))) return false;
      working = true; const ownedGeneration = generation; publish({ mutating: true, error: null });
      try {
        const result = await port.mutate(mutation);
        if (!active || generation !== ownedGeneration) return false;
        const applied = applyNotificationMutation(state.history, mutation, result);
        publish({ history: applied.history });
        port.changed(); requested = true; return true;
      } catch { if (generation === ownedGeneration) publish({ error: 'mutation' }); return false; }
      finally { working = false; publish({ mutating: false }); if (requested) void drain(); }
    },
    dispose() { active = false; port.cancel(); requested = false; listeners.clear(); settle(); },
  };
}
