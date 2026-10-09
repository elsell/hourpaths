import assert from 'node:assert/strict';
import test from 'node:test';
import { createProfilePrivacyOperationOwner, type ProfilePrivacy } from './profile-privacy';
const original: ProfilePrivacy = { userId: 'owner', visibility: 'public', revision: 1 };
test('lost response retries the same reviewed privacy operation; account replacement suppresses late results', async () => {
  let sequence = 0;
  const owner = createProfilePrivacyOperationOwner(() => `operation-${++sequence}`);
  const keys: string[] = [];
  assert.equal((await owner.submit(original, 'private', async (_, __, key) => { keys.push(key); throw new Error('response lost'); })).kind, 'failed');
  assert.equal((await owner.submit(original, 'private', async (value, visibility, key) => { keys.push(key); return { ...value, visibility, revision: 2 }; })).kind, 'applied');
  assert.deepEqual(keys, ['operation-1', 'operation-1']);
  let finish!: (value: ProfilePrivacy) => void;
  const pending = owner.submit({ ...original, revision: 3 }, 'private', () => new Promise(resolve => { finish = resolve; }));
  owner.cancel();
  finish({ ...original, visibility: 'private', revision: 4 });
  assert.equal((await pending).kind, 'superseded');
});
