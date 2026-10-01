import { readInbox, inboxCommands } from './api-inbox';
import { sharedMemberCommands } from './shared-member-commands';
import { sharedInvitationCommands } from './shared-invitation-commands';
import { createSessionApiClient } from '@hourpaths/api-client';
import { mergeManagedPendingInvitationPage, pathInvitationFailureFromProblem } from '@hourpaths/client-core';
import type { SharingRepository } from '../ports/sharing-repository';
function required<T>(result: { data?: { data: T }; error?: unknown; response: Response }): T {
  if (!result.response.ok || !result.data) throw pathInvitationFailureFromProblem(result.response.status, result.error);
  return result.data.data;
}
function progress(value: { accumulatedSeconds: number; targetSeconds: number } | undefined) {
  if (value === undefined) return undefined;
  if (!Number.isSafeInteger(value.accumulatedSeconds) || value.accumulatedSeconds < 0 || !Number.isSafeInteger(value.targetSeconds) || value.targetSeconds <= 0) throw { kind: 'invalid_response' };
  return { accumulatedSeconds: value.accumulatedSeconds, targetSeconds: value.targetSeconds };
}
export function apiSharingRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): SharingRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  const repository: SharingRepository = {
    inbox: (cursor, signal) => readInbox(client(signal), cursor),
    inboxCommands: key => inboxCommands(() => client(), key),
    memberCommands: key => sharedMemberCommands(() => client(), (pathId, cursor) => repository.members(pathId, cursor), key),
    async members(pathId, cursor, signal) {
      const result = await client(signal).pathMembers(pathId, cursor || undefined);
      const values = required(result), nextCursor = result.data?.meta.nextCursor ?? '';
      if (nextCursor && (nextCursor === cursor || !values.length)) throw { kind: 'invalid_response' };
      const items = values.map(value => ({
        userId: value.userId, username: value.username, displayName: value.displayName, role: value.role,
        sessionCount: value.sessionCount, totalTrackedSeconds: value.totalTrackedSeconds,
        blockedByViewer: value.blockedByViewer === true, intervalProgress: progress(value.intervalProgress), overallProgress: progress(value.overallProgress),
        canRemove: value.canRemove === true, canChangeRole: value.canChangeRole === true,
        canGrantAdministrator: value.canGrantAdministrator === true, canRevokeAdministrator: value.canRevokeAdministrator === true,
        canStepDownAdministrator: value.canStepDownAdministrator === true,
      }));
      if (items.some(value => !value.userId || !value.username || !value.displayName || !['creator', 'administrator', 'participant', 'supporter'].includes(value.role)
        || !Number.isSafeInteger(value.sessionCount) || value.sessionCount < 0 || !Number.isSafeInteger(value.totalTrackedSeconds) || value.totalTrackedSeconds < 0)) throw { kind: 'invalid_response' };
      return { items, nextCursor };
    },
    commands: key => sharedInvitationCommands(repository, key),
    async context(pathId, signal) {
      const response = await client(signal).path(pathId).catch(() => { throw { kind: 'network' }; });
      const value = required(response);
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
  return repository;
}
