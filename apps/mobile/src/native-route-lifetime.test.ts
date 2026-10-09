import assert from 'node:assert/strict';
import test from 'node:test';
import { createNativeRouteLifetime } from './native-route-lifetime';

function fixture() {
  const deferred: (() => void)[] = [];
  const dismissed: string[] = [];
  const lifetime = createNativeRouteLifetime(key => () => dismissed.push(key), work => deferred.push(work));
  const flush = () => { for (const work of deferred.splice(0)) work(); };
  return { lifetime, dismissed, flush };
}

test('deep-link stack normalization preserves the surviving history screen and ordinary Back dismisses once', () => {
  const { lifetime, dismissed, flush } = fixture();
  const leaveAncestor = lifetime.mount('history');
  const leaveIncoming = lifetime.mount('history');
  leaveAncestor(); flush();
  assert.deepEqual(dismissed, []);
  leaveIncoming(); flush();
  assert.deepEqual(dismissed, ['history']);
  leaveIncoming(); flush();
  assert.deepEqual(dismissed, ['history']);
});

test('a replacement mounted during teardown keeps its content across the passive-effect gap', () => {
  const { lifetime, dismissed, flush } = fixture();
  const leaveOld = lifetime.mount('activity');
  leaveOld();
  const leaveNew = lifetime.mount('activity');
  flush(); assert.deepEqual(dismissed, []);
  leaveNew(); flush(); assert.deepEqual(dismissed, ['activity']);
});

test('cleanup captures the current presentation only for the latest final departure', () => {
  const deferred: (() => void)[] = [];
  let presentation = { owner: 'old' };
  const removed: string[] = [];
  const lifetime = createNativeRouteLifetime(() => {
    const captured = presentation;
    return () => { if (presentation === captured) removed.push(captured.owner); };
  }, work => deferred.push(work));
  const leaveOld = lifetime.mount('path'); leaveOld();
  presentation = { owner: 'new' };
  const leaveNew = lifetime.mount('path'); leaveNew();
  for (const work of deferred.splice(0)) work();
  assert.deepEqual(removed, ['new']);
});

test('a different route does not keep an abandoned route alive', () => {
  const { lifetime, dismissed, flush } = fixture();
  const leave = lifetime.mount('first'); lifetime.mount('second');
  leave(); flush(); assert.deepEqual(dismissed, ['first']);
});
