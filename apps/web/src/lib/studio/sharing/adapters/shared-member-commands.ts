import { createPathMemberRemovalOperationOwner, createPathMemberRoleChangeOperationOwner, reviewPathMemberRemoval, reviewPathMemberRoleChange } from '@hourpaths/client-core';
import type { MemberCommands } from '../ports/member-commands';
import type { MemberReview, PathMember } from '../domain/members';
import { allowsMemberAction } from '../domain/members';
import type { createSessionApiClient } from '@hourpaths/api-client';
type Client = ReturnType<typeof createSessionApiClient>;
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new Error('member operation unavailable');
  return result.data.data;
}
export function sharedMemberCommands(client: () => Client, members: (pathId: string, cursor: string) => Promise<{ items: readonly PathMember[]; nextCursor: string }>, key: () => string): MemberCommands {
  const roles = createPathMemberRoleChangeOperationOwner(key), removals = createPathMemberRemovalOperationOwner(key);
  let disposed = false, epoch = 0, selected: MemberReview | null = null;
  const clear = () => { epoch++; selected = null; roles.cancel(); removals.cancel(); };
  return {
    clear,
    dispose() { disposed = true; clear(); },
    async review(pathId, userId, action) {
      if (disposed) throw new Error('disposed');
      clear(); const ticket = epoch;
      let cursor = ''; const seen = new Set<string>(); let member: PathMember | undefined;
      do {
        const page = await members(pathId, cursor);
        member = page.items.find(value => value.userId === userId);
        if (member) break;
        cursor = page.nextCursor;
        if (cursor && seen.has(cursor)) throw new Error('repeated member cursor');
        seen.add(cursor);
      } while (cursor);
      if (!member || !allowsMemberAction(member, action)) throw new Error('member action unavailable');
      let runningTimer = false;
      if (action === 'remove' || action === 'supporter') {
        const fresh = required(await client().reviewPathMemberRemoval(pathId, userId));
        if (fresh.userId !== userId || fresh.role !== member.role || fresh.username !== member.username) throw new Error('member review changed');
        const valid = reviewPathMemberRemoval({ ...fresh, pathId });
        member = { ...member, displayName: valid.displayName, sessionCount: valid.sessionCount, totalTrackedSeconds: valid.totalTrackedSeconds };
        runningTimer = valid.runningTimer;
      }
      if (disposed || epoch !== ticket) throw new Error('superseded');
      selected = Object.freeze({ pathId, action, member: Object.freeze({ ...member, runningTimer }) });
      return selected;
    },
    async submit(review) {
      if (disposed || review !== selected) return { kind: 'superseded' };
      const source = { ...review.member, pathId: review.pathId };
      if (review.action === 'remove') return removals.submit(reviewPathMemberRemoval(source), true, async (pathId, userId, body, id) => required(await client().removePathMember(pathId, userId, body, id)));
      return roles.submit(reviewPathMemberRoleChange(source, review.action), true, async (pathId, userId, body, id) => required(await client().changePathMemberRole(pathId, userId, body, id)));
    },
  };
}
