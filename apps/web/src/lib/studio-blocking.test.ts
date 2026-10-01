import assert from 'node:assert/strict';
import { test } from 'node:test';
import { blockingCommands } from './studio/blocking/application/commands';
import type { BlockingRepository } from './studio/blocking/ports/blocking-repository';
import type { BlockReview } from './studio/blocking/domain/block';
const review: BlockReview = { target: { userId: 'person', username: 'person', displayName: 'person' }, sharedPaths: [], acknowledgement: { version: 1, token: 'review', expiresAt: '2026-10-01T00:00:00Z' } };
test('unresolved blocking retries retain the reviewed command and operation key', async () => {
  const calls: { review: BlockReview; key: string }[] = [];
  const repository: BlockingRepository = { review: async () => review, async block(value,key) { calls.push({review:value,key}); if(calls.length === 1) throw Error('transport'); } };
  let keys=0;
  const commands=blockingCommands(repository,()=>String(++keys));
  const signal=new AbortController().signal;
  await assert.rejects(commands.submit(review,signal));
  assert.equal(await commands.submit({...review},signal),true);
  assert.equal(keys,1); assert.equal(calls[0].review,calls[1].review);
});
test('disposal suppresses admitted completion and prevents another command', async () => {
  let finish!: () => void;
  const repository: BlockingRepository={review:async()=>review,block:()=>new Promise<void>(resolve=>{finish=resolve})};
  const commands=blockingCommands(repository,()=> 'operation'); const signal=new AbortController().signal;
  const pending=commands.submit(review,signal);
  assert.equal(await commands.submit(review,signal),false);
  commands.dispose();finish();assert.equal(await pending,false);
  assert.equal(await commands.submit(review,signal),false);
});
