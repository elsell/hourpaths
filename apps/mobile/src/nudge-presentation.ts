import type { NudgeEligibility } from '@hourpaths/client-core';

export type NudgeActionState =
  | Readonly<{ kind: 'send' }>
  | Readonly<{ kind: 'goal_complete' | 'rate_limited' }>
  | Readonly<{ kind: 'hidden' }>;

export function nudgeActionState(member: {
  isViewer: boolean;
  role: 'creator' | 'administrator' | 'participant' | 'supporter';
  nudgeEligibility?: NudgeEligibility;
}): NudgeActionState {
  if (member.isViewer || member.role === 'supporter' || !member.nudgeEligibility) return { kind: 'hidden' };
  return member.nudgeEligibility.eligible
    ? { kind: 'send' }
    : { kind: member.nudgeEligibility.reason };
}
