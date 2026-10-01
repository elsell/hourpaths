export const presets = ['you_have_got_this', 'lets_go', 'little_progress_counts', 'keep_it_going', 'time_to_work'] as const;
export const audiences = ['nobody', 'path_members', 'followers', 'everyone'] as const;
export type Preset = typeof presets[number];
export type Audience = typeof audiences[number];
export type Eligibility = { eligible: true; pathId: string; recipientUserId: string } | { eligible: false; pathId: string; recipientUserId: string; reason: 'goal_complete' | 'rate_limited' };
export interface AudiencePreference { audience: Audience; pathId: string; userId: string; revision: number }
export class NudgeFailure extends Error {
  constructor(readonly kind: 'conflict' | 'unavailable' = 'unavailable') { super(kind); }
}
export type CommandResult<T> = { kind: 'applied'; value: T } | { kind: 'failed'; conflict: boolean } | { kind: 'busy' | 'superseded' };
