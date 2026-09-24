import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createPathAdministrationTarget,
  currentPathAdministrationCredential,
  ownsPathAdministrationTarget,
  rotatePathAdministrationTarget,
} from './path-administration-target';

test('same-owner rotations retain every admitted request token without changing captured identity', () => {
  const original = { token: 'a' };
  const target = createPathAdministrationTarget('owner', 'path', original);
  assert.equal(rotatePathAdministrationTarget(target, 'owner', { token: 'b' }), true);
  assert.equal(rotatePathAdministrationTarget(target, 'owner', { token: 'c' }), true);
  assert.equal(target.session, original);
  assert.deepEqual(target.sessionTokens, ['a', 'b', 'c']);
  assert.equal(ownsPathAdministrationTarget(target, 'owner', 'path', 'a'), true);
  assert.equal(ownsPathAdministrationTarget(target, 'owner', 'path', 'c'), true);
});

test('current credential is admitted only for the retained owner, Path, and token lineage', () => {
  const target = createPathAdministrationTarget('owner', 'path', { token: 'a' });
  const rotated = { token: 'b' };
  rotatePathAdministrationTarget(target, 'owner', rotated);
  assert.equal(currentPathAdministrationCredential(target, 'owner', 'path', rotated), rotated);
  assert.equal(currentPathAdministrationCredential(target, 'other', 'path', rotated), null);
  assert.equal(currentPathAdministrationCredential(target, 'owner', 'other', rotated), null);
  assert.equal(currentPathAdministrationCredential(target, 'owner', 'path', { token: 'c' }), null);
});

test('replacement owners and unrelated Paths cannot inherit an administration target', () => {
  const target = createPathAdministrationTarget('owner-a', 'path-a', { token: 'a' });
  assert.equal(rotatePathAdministrationTarget(target, 'owner-b', { token: 'b' }), false);
  assert.equal(ownsPathAdministrationTarget(target, 'owner-b', 'path-a', 'a'), false);
  assert.equal(ownsPathAdministrationTarget(target, 'owner-a', 'path-b', 'a'), false);
  assert.equal(ownsPathAdministrationTarget(target, 'owner-a', 'path-a', 'b'), false);
});
