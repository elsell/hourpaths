import { createSessionApiClient, type OwnershipTransferResult } from '@hourpaths/api-client';
import { decodeOwnershipTransferReview, validViewerTimeZone } from '../../../path-ownership-transfer';
import type { OwnershipRepository } from '../ports/ownership-repository';
import type { Transfer } from '../domain/transfer';
class OwnershipError extends Error {
  constructor(readonly status: number) { super('ownership_request_failed'); }
}
function required<T>(result: { response: Response; data?: { data: T } }): T {
  if (!result.response.ok || !result.data) throw new OwnershipError(result.response.status);
  return result.data.data;
}
function text(value: unknown): value is string { return typeof value === 'string' && value.trim() !== '' && !/[\u0000-\u001f\u007f]/u.test(value); }
function mapTransfer(value: OwnershipTransferResult, pathId: string): Transfer {
  const t = value.transfer, identity = value.counterpart;
  if (!t || t.pathId !== pathId || !text(t.id) || !text(t.creatorUserId) || !text(t.recipientUserId) || t.creatorUserId === t.recipientUserId
    || !['pending', 'accepted', 'declined', 'canceled'].includes(t.state) || !Number.isFinite(Date.parse(t.reviewedAt)) || !Number.isFinite(Date.parse(t.expiresAt))
    || Date.parse(t.expiresAt) <= Date.parse(t.reviewedAt) || !validViewerTimeZone(value.viewerTimeZone)
    || !identity || !text(identity.displayName) || !text(identity.username)
    || !['creator', 'recipient'].includes(value.counterpartRole) || identity.userId !== (value.counterpartRole === 'creator' ? t.creatorUserId : t.recipientUserId)) throw new OwnershipError(502);
  return { id: t.id, pathId: t.pathId, creatorUserId: t.creatorUserId, recipientUserId: t.recipientUserId, reviewedAt: t.reviewedAt, expiresAt: t.expiresAt, state: t.state,
    counterpart: { userId: identity.userId, username: identity.username, displayName: identity.displayName }, counterpartRole: value.counterpartRole, viewerTimeZone: value.viewerTimeZone };
}
export function apiOwnershipRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): OwnershipRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  return {
    async pending(pathId, signal) {
      const response = await client(signal).pendingOwnershipTransfer(pathId);
      if (response.response.status === 404) return null;
      const value = mapTransfer(required(response), pathId);
      if (value.state !== 'pending') throw new OwnershipError(502);
      return value;
    },
    async candidates(pathId, cursor, signal) {
      const response = await client(signal).ownershipTransferCandidates(pathId, cursor || undefined);
      const values = required(response), nextCursor = response.data?.meta.nextCursor ?? '';
      if (nextCursor && (nextCursor === cursor || !values.length)) throw new OwnershipError(502);
      if (values.some(v => !text(v.userId) || !text(v.username) || !text(v.displayName) || typeof v.administrator !== 'boolean')) throw new OwnershipError(502);
      return { items: values.map(v => ({ userId: v.userId, username: v.username, displayName: v.displayName, administrator: v.administrator })), nextCursor };
    },
    async review(pathId, recipientId) {
      const value = decodeOwnershipTransferReview(required(await client().reviewOwnershipTransfer(pathId, { recipientUserId: recipientId })), pathId, recipientId);
      if (!value) throw new OwnershipError(502);
      return { ...value, recipient: { ...value.recipient } };
    },
    async execute(command, operationId) {
      const c = client();
      if (command.kind === 'initiate') {
        const value = mapTransfer(required(await c.initiateOwnershipTransfer(command.pathId, { reservationToken: command.review.reservationToken }, operationId)), command.pathId);
        if (value.state !== 'pending' || value.recipientUserId !== command.review.recipient.userId || value.counterpartRole !== 'recipient'
          || value.reviewedAt !== command.review.reviewedAt || value.expiresAt !== command.review.expiresAt) throw new OwnershipError(502);
        return value;
      }
      const expected = command.transfer;
      const response = command.kind === 'accept' ? await c.acceptOwnershipTransfer(expected.id, operationId)
        : command.kind === 'decline' ? await c.declineOwnershipTransfer(expected.id, operationId) : await c.cancelOwnershipTransfer(expected.id, operationId);
      const value = mapTransfer(required(response), expected.pathId);
      const state = command.kind === 'accept' ? 'accepted' : command.kind === 'decline' ? 'declined' : 'canceled';
      if (value.id !== expected.id || value.state !== state || value.creatorUserId !== expected.creatorUserId || value.recipientUserId !== expected.recipientUserId
        || value.expiresAt !== expected.expiresAt || value.reviewedAt !== expected.reviewedAt || value.counterpartRole !== expected.counterpartRole) throw new OwnershipError(502);
      return value;
    },
  };
}
