import assert from 'node:assert/strict';
import test from 'node:test';
import {
  derivePathRenamePresentation,
  pathManagementIsBusy,
  reconcilePathManagementScreen,
} from './path-management-presentation';

const base = {
  busy: false,
  currentName: 'Developing',
  draftName: 'Developing',
};

test('rename presentation keeps pristine state quiet and non-actionable', () => {
  assert.deepEqual(derivePathRenamePresentation(base), {
    canSave: false,
    kind: 'pristine',
  });
});

test('rename presentation distinguishes invalid input from unchanged input', () => {
  assert.deepEqual(derivePathRenamePresentation({ ...base, draftName: '   ' }), {
    canSave: false,
    kind: 'invalid',
  });
  assert.deepEqual(derivePathRenamePresentation({
    ...base,
    draftName: `Developing${String.fromCharCode(0)}`,
  }), {
    canSave: false,
    kind: 'invalid',
  });
});

test('rename presentation enables only a valid changed name', () => {
  assert.deepEqual(derivePathRenamePresentation({ ...base, draftName: 'changed-path' }), {
    canSave: true,
    kind: 'dirty',
  });
});

test('rename presentation emits exactly one saving, saved, or error state', () => {
  assert.deepEqual(derivePathRenamePresentation({
    ...base,
    busy: true,
    draftName: 'changed-path',
  }), {
    canSave: false,
    kind: 'saving',
  });
  assert.deepEqual(derivePathRenamePresentation({
    ...base,
    savedName: 'Developing',
  }), {
    canSave: false,
    kind: 'saved',
    name: 'Developing',
  });
  assert.deepEqual(derivePathRenamePresentation({
    ...base,
    errorText: 'Try again.',
  }), {
    canSave: false,
    kind: 'error',
    text: 'Try again.',
  });
});

test('any active Path-management mutation blocks competing work and dismissal', () => {
  assert.equal(pathManagementIsBusy({ deleteBusy: false, goalBusy: false, renameBusy: false }), false);
  assert.equal(pathManagementIsBusy({ deleteBusy: false, goalBusy: true, renameBusy: false }), true);
  assert.equal(pathManagementIsBusy({ deleteBusy: false, goalBusy: false, renameBusy: true }), true);
  assert.equal(pathManagementIsBusy({ deleteBusy: true, goalBusy: false, renameBusy: false }), true);
  assert.equal(pathManagementIsBusy({ deleteBusy: false, goalBusy: false, renameBusy: false, visibilityBusy: true }), true);
});

test('successful goal confirmation returns review to goals without hiding saved feedback', () => {
  assert.equal(reconcilePathManagementScreen({
    goalSaved: false,
    hasReview: true,
    screen: 'goals',
  }), 'review');
  assert.equal(reconcilePathManagementScreen({
    goalSaved: true,
    hasReview: false,
    screen: 'review',
  }), 'goals');
  assert.equal(reconcilePathManagementScreen({
    goalSaved: false,
    hasReview: false,
    screen: 'overview',
  }), 'overview');
});
