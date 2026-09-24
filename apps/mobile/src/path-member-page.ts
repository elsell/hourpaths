import type { PathMember } from '@hourpaths/api-client';
import { pathMemberProgressFromAPI } from './path-member-progress';
import type { PathMemberSummary } from './ui/path-member-management-view';

export type PathMemberPage = Readonly<{
  items: PathMemberSummary[];
  nextCursor: string | null;
}>;

function validText(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value
    && !/[\u0000-\u001f\u007f]/u.test(value);
}

export function pathMemberPageFromAPI(pathId: string, viewerId: string, value: unknown): PathMemberPage {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid Path member page');
  const page = value as { data?: unknown; meta?: unknown };
  if (!Array.isArray(page.data) || !page.meta || typeof page.meta !== 'object' || Array.isArray(page.meta)) {
    throw new Error('invalid Path member page');
  }
  const nextCursor = (page.meta as { nextCursor?: unknown }).nextCursor;
  if (nextCursor !== undefined && typeof nextCursor !== 'string') throw new Error('invalid Path member page');
  const items = page.data.map((candidate): PathMemberSummary => {
    if (!candidate || typeof candidate !== 'object' || Array.isArray(candidate)) throw new Error('invalid Path member page');
    const member = candidate as Record<string, unknown>;
    if (
      !validText(member.userId)
      || !validText(member.username)
      || !validText(member.displayName)
      || (member.role !== 'creator' && member.role !== 'administrator' && member.role !== 'participant' && member.role !== 'supporter')
      || typeof member.sessionCount !== 'number' || !Number.isSafeInteger(member.sessionCount) || member.sessionCount < 0
      || typeof member.totalTrackedSeconds !== 'number' || !Number.isSafeInteger(member.totalTrackedSeconds) || member.totalTrackedSeconds < 0
      || typeof member.blockedByViewer !== 'boolean'
      || typeof member.canGrantAdministrator !== 'boolean'
      || typeof member.canChangeRole !== 'boolean'
      || typeof member.canRevokeAdministrator !== 'boolean'
      || typeof member.canRemove !== 'boolean'
      || typeof member.canStepDownAdministrator !== 'boolean'
      || typeof member.canLeave !== 'boolean'
    ) throw new Error('invalid Path member page');
    const typed = member as PathMember;
    return Object.freeze({
      blockedByViewer: typed.blockedByViewer,
      canGrantAdministrator: typed.canGrantAdministrator,
      canChangeRole: typed.canChangeRole,
      canLeave: typed.canLeave,
      canRevokeAdministrator: typed.canRevokeAdministrator,
      canRemove: typed.canRemove,
      canStepDownAdministrator: typed.canStepDownAdministrator,
      displayName: typed.displayName,
      intervalProgress: pathMemberProgressFromAPI(typed.intervalProgress),
      isViewer: typed.userId === viewerId,
      overallProgress: pathMemberProgressFromAPI(typed.overallProgress),
      pathId,
      role: typed.role,
      sessionCount: typed.sessionCount,
      totalTrackedSeconds: typed.totalTrackedSeconds,
      userId: typed.userId,
      username: typed.username,
    });
  });
  return Object.freeze({ items, nextCursor: nextCursor ?? null });
}
