import { createPathInvitationAcceptOwner, createPathInvitationRejectOwner, mergePendingInvitationPage, reviewPendingPathInvitationAcceptance, pathInvitationFailureFromProblem, pathInvitationFailureMessageKey } from '@hourpaths/client-core';
import type { createSessionApiClient } from '@hourpaths/api-client';
import type { InboxCommands } from '../ports/inbox-commands';
import type { InvitationInboxPage } from '../domain/inbox';
type Client = ReturnType<typeof createSessionApiClient>;
function required<T>(response: { response: Response; data?: { data: T }; error?: unknown }): T {
  if (!response.response.ok || !response.data) throw pathInvitationFailureFromProblem(response.response.status, response.error);
  return response.data.data;
}
export async function readInbox(client: Client, cursor: string): Promise<InvitationInboxPage> {
  const result = await client.pendingPathInvitations(cursor || undefined);
  const values = required(result), nextCursor = result.data?.meta.nextCursor ?? '';
  if (nextCursor && (nextCursor === cursor || !values.length)) throw { kind: 'invalid_response' };
  const items = values.map(value => ({
    invitation: { id: value.invitation.id, pathId: value.invitation.pathId, inviterUserId: value.invitation.inviterUserId, recipientUserId: value.invitation.recipientUserId, offeredRole: value.invitation.offeredRole, createdAt: value.invitation.createdAt, ...(value.invitation.acceptedAt ? { acceptedAt: value.invitation.acceptedAt } : {}) },
    pathName: value.pathName,
    inviter: { userId: value.inviter.userId, username: value.inviter.username, displayName: value.inviter.displayName },
    ...(value.warning ? { warning: { pathVisibility: value.warning.pathVisibility, hasRetainedActivity: value.warning.hasRetainedActivity } } : {}),
  }));
  return mergePendingInvitationPage({ items: [], nextCursor: cursor }, { items, nextCursor }, cursor);
}
export function inboxCommands(client: () => Client, key: () => string): InboxCommands {
  const accept = createPathInvitationAcceptOwner(key), reject = createPathInvitationRejectOwner(key);
  let disposed = false;
  const clear = () => { accept.cancel(); reject.cancel(); };
  return {
    clear,
    dispose() { disposed = true; clear(); },
    failureMessage: failure => pathInvitationFailureMessageKey(failure.kind === 'http' ? pathInvitationFailureFromProblem(failure.status, { code: failure.code }) : failure),
    async respond(pending, decision) {
      if (disposed) return { kind: 'superseded' };
      const review = reviewPendingPathInvitationAcceptance(pending);
      if (decision === 'decline') {
        const result = await reject.submit(review.invitationId, true, async (id, operationId) => required(await client().rejectPathInvitation(id, operationId)));
        return result.kind === 'rejected' ? { kind: 'rejected' } : result;
      }
      const result = await accept.submit(review, true, async (id, operationId, body) => {
        const value = required(await client().acceptPathInvitation(id, operationId, body));
        const expected = pending.invitation;
        if (value.pathId !== expected.pathId || value.offeredRole !== expected.offeredRole || value.inviterUserId !== expected.inviterUserId || value.recipientUserId !== expected.recipientUserId) throw { kind: 'invalid_response' };
        return value;
      });
      return result.kind === 'accepted' ? { kind: 'accepted', pathId: result.invitation.pathId, role: result.invitation.offeredRole } : result;
    },
  };
}
