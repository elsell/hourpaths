import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createPathInvitationCancelOwner,
  mergeManagedPendingInvitationPage,
  type ManagedPendingPathInvitation,
  type PathInvitation,
} from './index.js';

const recipient = {
  userId: 'recipient-1',
  username: 'Reader.One',
  displayName: 'Reader One',
};

const pending: PathInvitation = {
  id: 'invitation-1',
  pathId: 'path-1',
  inviterUserId: 'creator',
  recipientUserId: recipient.userId,
  offeredRole: 'participant',
  createdAt: '2026-07-23T20:00:00Z',
};

const managedPendingProjection: ManagedPendingPathInvitation = {
  invitation: pending,
  inviter: {
    userId: 'creator',
    username: 'Book.Owner',
    displayName: 'Book Owner',
  },
  recipient,
};

test('manager pending pages validate joined public identities and replace stale first-page state', () => {
  const signed = 'manager-snapshot.signature+/=';
  const first = mergeManagedPendingInvitationPage(
    { items: [], nextCursor: '' },
    {
      items: [
        managedPendingProjection,
        { ...managedPendingProjection, invitation: { ...pending, id: 'invitation-2' } },
      ],
      nextCursor: signed,
    },
    '',
  );
  assert.equal(first.nextCursor, signed);
  const replacement = mergeManagedPendingInvitationPage(
    first,
    { items: [{ ...managedPendingProjection, invitation: { ...pending, id: 'invitation-3' } }], nextCursor: '' },
    '',
  );
  assert.deepEqual(replacement.items.map(({ invitation }) => invitation.id), ['invitation-3']);
  for (const malformed of [
    { ...managedPendingProjection, recipient: { ...recipient, userId: 'another-user' } },
    { ...managedPendingProjection, inviter: { ...managedPendingProjection.inviter, email: 'private@example.com' } },
    { ...managedPendingProjection, invitation: { ...pending, acceptedAt: '2026-07-23T20:01:00Z' } },
  ]) {
    assert.throws(() => mergeManagedPendingInvitationPage(
      { items: [], nextCursor: '' },
      { items: [malformed as ManagedPendingPathInvitation], nextCursor: '' },
      '',
    ), /invalid managed pending invitation/);
  }
});

test('manager cancellation confirms, retains its retry key, and validates exact terminal result', async () => {
  const keys = ['cancel-invitation-key-01', 'unused-cancel-key-02'];
  const owner = createPathInvitationCancelOwner(() => keys.shift()!);
  const calls: Array<{ pathId: string; invitationId: string; key: string }> = [];
  const request = async (pathId: string, invitationId: string, key: string): Promise<unknown> => {
    calls.push({ pathId, invitationId, key });
    if (calls.length === 1) throw { kind: 'network' } as const;
    return { invitationId, canceledAt: '2026-08-08T15:00:00Z' };
  };

  assert.deepEqual(await owner.submit('path-1', 'invitation-1', false, request), { kind: 'cancelled' });
  assert.equal(calls.length, 0);
  assert.deepEqual(await owner.submit('path-1', 'invitation-1', true, request), {
    kind: 'failed', failure: { kind: 'network' },
  });
  assert.deepEqual(await owner.submit('path-1', 'invitation-1', true, request), {
    kind: 'canceled',
    cancellation: { invitationId: 'invitation-1', canceledAt: '2026-08-08T15:00:00Z' },
  });
  assert.deepEqual(calls, [
    { pathId: 'path-1', invitationId: 'invitation-1', key: 'cancel-invitation-key-01' },
    { pathId: 'path-1', invitationId: 'invitation-1', key: 'cancel-invitation-key-01' },
  ]);
  assert.deepEqual(await owner.submit('path-1', 'invitation-2', true, async () => ({
    invitationId: 'another-invitation', canceledAt: '2026-08-08T15:00:00Z',
  })), { kind: 'failed', failure: { kind: 'invalid_response' } });
});

test('manager cancellation supersedes stale completion and clears owned retry intent', async () => {
  const keys = ['cancel-invitation-key-01', 'cancel-invitation-key-02'];
  const owner = createPathInvitationCancelOwner(() => keys.shift()!);
  let resolve!: (value: unknown) => void;
  const response = new Promise<unknown>((done) => { resolve = done; });
  const stale = owner.submit('path-1', 'invitation-1', true, async () => response);
  owner.cancel('invitation-1');
  resolve({ invitationId: 'invitation-1', canceledAt: '2026-08-08T15:00:00Z' });
  assert.deepEqual(await stale, { kind: 'superseded' });
  const seen: string[] = [];
  assert.equal((await owner.submit('path-1', 'invitation-1', true, async (_path, id, key) => {
    seen.push(key);
    return { invitationId: id, canceledAt: '2026-08-08T15:01:00Z' };
  })).kind, 'canceled');
  assert.deepEqual(seen, ['cancel-invitation-key-02']);
});
