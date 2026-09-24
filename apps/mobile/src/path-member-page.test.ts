import assert from 'node:assert/strict';
import test from 'node:test';
import { pathMemberPageFromAPI } from './path-member-page';

const member = {
  blockedByViewer: false,
  canChangeRole: false,
  canGrantAdministrator: false,
  canLeave: true,
  canRemove: false,
  canRevokeAdministrator: false,
  canStepDownAdministrator: false,
  displayName: 'Second',
  intervalProgress: { accumulatedSeconds: 0, targetSeconds: 600 },
  role: 'supporter',
  sessionCount: 0,
  totalTrackedSeconds: 0,
  userId: 'user-2',
  username: 'second',
} as const;

test('People maps the generated API page envelope before session unwrapping', () => {
  assert.deepEqual(pathMemberPageFromAPI('path-1', 'user-2', {
    data: [member],
    meta: { nextCursor: 'next-page' },
  }), {
    items: [{
      ...member,
      isViewer: true,
      overallProgress: undefined,
      pathId: 'path-1',
    }],
    nextCursor: 'next-page',
  });
});

test('People rejects an already-unwrapped array and malformed pagination metadata', () => {
  assert.throws(() => pathMemberPageFromAPI('path-1', 'user-2', [member]), /invalid Path member page/);
  assert.throws(() => pathMemberPageFromAPI('path-1', 'user-2', {
    data: [member],
    meta: { nextCursor: 7 },
  }), /invalid Path member page/);
});
