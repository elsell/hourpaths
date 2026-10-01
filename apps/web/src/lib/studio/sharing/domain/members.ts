export type MemberRole = 'creator' | 'administrator' | 'participant' | 'supporter';
export type MemberAction = Exclude<MemberRole, 'creator'> | 'remove';
export interface PathMember {
  userId: string; username: string; displayName: string; role: MemberRole;
  sessionCount: number; totalTrackedSeconds: number;
  canRemove: boolean; canChangeRole: boolean; canGrantAdministrator: boolean;
  canRevokeAdministrator: boolean; canStepDownAdministrator: boolean;
}
export interface MemberReview {
  pathId: string; action: MemberAction;
  member: Readonly<PathMember & { runningTimer: boolean }>;
}
export function allowsMemberAction(member: PathMember, action: MemberAction): boolean {
  if (action === 'remove') return (member.role === 'participant' || member.role === 'supporter') && member.canRemove;
  if (action === 'administrator') return member.role === 'participant' && member.canGrantAdministrator;
  if (member.role === 'administrator') return action === 'participant' && (member.canRevokeAdministrator || member.canStepDownAdministrator);
  return member.canChangeRole && ((member.role === 'participant' && action === 'supporter') || (member.role === 'supporter' && action === 'participant'));
}
