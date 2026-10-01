import { createPathLeaveOperationOwner, reviewPathLeave } from '@hourpaths/client-core';
import type { createSessionApiClient } from '@hourpaths/api-client';
import type { LeaveCommands } from '../ports/leave-commands';
import type { LeaveReview } from '../domain/leave';
type Recovery = { review: LeaveReview; retainActivity: boolean; key: string };
export function createLeaveRecovery() { return new Map<string, Recovery>(); }
class LeaveRequestFailure extends Error { constructor(readonly status: number) { super('leave_unavailable'); } }
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new LeaveRequestFailure(result.response.status);
  return result.data.data;
}
export function sharedLeaveCommands(client: (signal?: AbortSignal) => ReturnType<typeof createSessionApiClient>, key: () => string, recovery = createLeaveRecovery()): LeaveCommands {
  let attempt: Recovery | undefined;
  const owner = createPathLeaveOperationOwner(() => attempt?.key ?? key());
  let disposed = false, epoch = 0, selected: LeaveReview | null = null, admitted = false;
  return {
    dispose() { disposed = true; epoch++; selected = null; owner.cancel(); },
    async review(pathId, signal) {
      if (disposed || admitted) throw new Error('leave_review_unavailable');
      const retained = recovery.get(pathId);
      if (retained) { attempt = retained; selected = retained.review; return selected; }
      const ticket = ++epoch, path = required(await client(signal).path(pathId));
      if (path.id !== pathId || path.archivedAt) throw new Error('leave_review_unavailable');
      const value = reviewPathLeave(path);
      if (disposed || ticket !== epoch) throw new Error('superseded');
      selected = Object.freeze({ pathId: value.pathId, name: value.pathName, participant: path.capabilities.trackTime === true });
      return selected;
    },
    async submit(review, retainActivity, signal) {
      if (disposed || review !== selected || admitted) return { kind: 'superseded' };
      if (!review.participant && !retainActivity) throw new Error('supporter_activity_choice');
      const existing = recovery.get(review.pathId);
      if (existing && existing.retainActivity !== retainActivity) throw new Error('leave_choice_changed');
      attempt = existing ?? { review: Object.freeze({ ...review, retainActivity }), retainActivity, key: key() };
      recovery.set(review.pathId, attempt);
      admitted = true;
      try {
        const result = await owner.submit({ pathId: review.pathId, pathName: review.name, retainActivity }, true, async (pathId, body, id) => required(await client(signal).leavePath(pathId, body, id)));
        if (result.kind === 'applied') { selected = null; recovery.delete(review.pathId); }
        if (result.kind === 'failed' && result.cause instanceof LeaveRequestFailure && [400, 401, 403, 404, 409].includes(result.cause.status)) { recovery.delete(review.pathId); owner.cancel(review.pathId); attempt = undefined; }
        return result.kind === 'applied' ? { kind: 'applied' } : result.kind === 'failed' ? { kind: 'failed' } : { kind: 'superseded' };
      } finally { admitted = false; }
    },
  };
}
