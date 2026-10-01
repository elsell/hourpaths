import test from 'node:test';
import assert from 'node:assert/strict';
import { createNotificationHistoryOwner } from './notification-history-owner';
const item = { id: 'n', type: 'new_follower', presentation: 'informational', read: false, createdAt: '2026-10-01T00:00:00Z', actor: { userId: 'other', username: 'other', displayName: 'other' } };
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done; }); return { resolve, promise }; }
test('notification mutations queue a trailing authoritative refresh; failed refresh preserves history and count', async () => {
  let rows = [item], count = 1, reads = 0, published = 0, fail = false;
  const gate = deferred<unknown>();
  const owner = createNotificationHistoryOwner({
    page: async () => { reads++; if (fail) throw Error('offline'); return { items: rows, nextCursor: '', unreadCount: count }; },
    mutate: async () => { const result = await gate.promise; rows = [{ ...item, read: true }]; count = 0; return result; },
    changed: () => { published++; }, cancel() {},
  });
  await owner.refresh(); assert.equal(owner.snapshot().history.unreadCount, 1);
  const mutation = owner.mutate({ kind: 'read', notificationId: 'n' });
  const pendingRefresh = owner.refresh();
  assert.equal(owner.refresh(), pendingRefresh);
  assert.equal(reads, 1);
  assert.equal(await owner.mutate({ kind: 'delete', notificationId: 'n' }), false);
  gate.resolve({ unreadCount: 0 }); assert.equal(await mutation, true);
  await owner.refresh(); assert.equal(owner.snapshot().history.items[0]?.read, true);
  assert.equal(owner.snapshot().history.unreadCount, 0); assert.equal(published, 1);
  const before = owner.snapshot().history; fail = true; await owner.refresh();
  assert.equal(owner.snapshot().history, before); assert.equal(owner.snapshot().error, 'history');
  owner.dispose();
});
test('disposal aborts reads and suppresses late mutation receipts and convergence signals', async () => {
  const gate = deferred<unknown>(); let published = 0; let canceled = false;
  const owner = createNotificationHistoryOwner({page: async () => ({items:[item],nextCursor:'',unreadCount:1}), mutate: async () => gate.promise, changed: () => { published++; }, cancel() { canceled = true; }});
  await owner.refresh(); const mutation = owner.mutate({kind:'delete',notificationId:'n'});
  owner.dispose(); assert.equal(canceled, true); gate.resolve({unreadCount:0});
  assert.equal(await mutation, false); assert.equal(published, 0);
  assert.equal(await owner.mutate({kind:'read-all'}), false);
});
