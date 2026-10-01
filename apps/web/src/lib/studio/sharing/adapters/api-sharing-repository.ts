import { createSessionApiClient } from '@hourpaths/api-client';
import { mergeManagedPendingInvitationPage, pathInvitationFailureFromProblem } from '@hourpaths/client-core';
import type { SharingRepository } from '../ports/sharing-repository';
function required<T>(result: { data?: { data: T }; error?: unknown; response: Response }): T {
  if (!result.response.ok || !result.data) throw pathInvitationFailureFromProblem(result.response.status, result.error);
  return result.data.data;
}
export function apiSharingRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): SharingRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  return {
    async context(pathId, signal) {
      const value = required(await client(signal).path(pathId));
      if (value.id !== pathId || !value.name || value.archivedAt || value.capabilities.inviteMembers !== true) throw { kind: 'opaque' };
      return { id: value.id, name: value.name };
    },
    async recipient(pathId, username) {
      const value = required(await client().reviewPathInvitationRecipient(pathId, username));
      return { userId: value.userId, username: value.username, displayName: value.displayName };
    },
    async send(pathId, body, key) { return required(await client().sendPathInvitation(pathId, body, key)); },
    async cancel(pathId, invitationId, key) { return required(await client().cancelPathInvitation(pathId, invitationId, key)); },
    async pending(pathId, cursor, signal) {
      const result = await client(signal).managedPathInvitations(pathId, cursor || undefined);
      const values = required(result);
      const nextCursor = result.data?.meta.nextCursor ?? '';
      if (nextCursor && (nextCursor === cursor || !values.length)) throw { kind: 'invalid_response' };
      const items = values.map(value => ({
        invitation: { id: value.invitation.id, pathId: value.invitation.pathId, inviterUserId: value.invitation.inviterUserId, recipientUserId: value.invitation.recipientUserId, offeredRole: value.invitation.offeredRole, createdAt: value.invitation.createdAt },
        inviter: { userId: value.inviter.userId, username: value.inviter.username, displayName: value.inviter.displayName },
        recipient: { userId: value.recipient.userId, username: value.recipient.username, displayName: value.recipient.displayName },
      }));
      if (items.some(value => value.invitation.pathId !== pathId)) throw { kind: 'invalid_response' };
      return mergeManagedPendingInvitationPage({ items: [], nextCursor: cursor }, { items, nextCursor }, cursor);
    },
  };
}
