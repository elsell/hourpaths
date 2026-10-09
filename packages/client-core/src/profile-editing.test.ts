import test from 'node:test';
import assert from 'node:assert/strict';
import { createProfileEditOperationOwner, type EditableProfile } from './profile-editing';

test('a profile retry retains its key and a replaced account discards its result', async () => {
  let key = 0;
  const operation = createProfileEditOperationOwner(() => `profile-key-${++key}`);
  const profile: EditableProfile = { userId: 'owner', username: 'owner', displayName: 'Changed', description: '', revision: 1 };
  const keys: string[] = [];
  const failed = await operation.submit(profile, async (_, id) => { keys.push(id); throw new Error('offline'); });
  assert.equal(failed.kind, 'failed');
  let finish!: (value: EditableProfile) => void;
  const old = operation.submit(profile, (_, id) => { keys.push(id); return new Promise(resolve => { finish = resolve; }); });
  operation.cancel();
  const fresh = await operation.submit({ ...profile, userId: 'replacement' }, async value => ({ ...value, revision: 2 }));
  finish({ ...profile, revision: 2 });
  assert.equal(keys[0], keys[1]);
  assert.equal((await old).kind, 'superseded');
  assert.equal(fresh.kind, 'applied');
});
