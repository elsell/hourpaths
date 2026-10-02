export type Recurrence = 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly';

export interface GoalAlignment {
  readonly minute?: number;
  readonly hour?: number;
  readonly day?: number;
  readonly month?: number;
  readonly isoWeekday?: number;
}
export interface PathGoals {
  readonly goal: { readonly targetSeconds: number; readonly recurrence: Recurrence; readonly alignment: GoalAlignment } | null;
  readonly overallTarget: number | null;
}
export interface Path extends PathGoals {
  readonly id: string;
  readonly name: string;
  readonly visibility: 'private' | 'followers' | 'public';
  readonly canManageVisibility: boolean;
  readonly archived: boolean;
  readonly pinned: boolean;
  readonly position: number | null;
  readonly pinnedPosition: number | null;
  readonly recentActivityAt: number;
  readonly canTransferOwnership: boolean;
  readonly canLeave: boolean;
  readonly canInvite: boolean;
  readonly canTrack: boolean;
  readonly canEdit: boolean;
  readonly canManageGoals: boolean;
  readonly canManageLifecycle: boolean;
}

export interface TrackingSnapshot {
  readonly pending?: boolean;
  readonly savedTotalSeconds: number;
  readonly activeSession: { readonly id: string; readonly startedAt: number; readonly originalStartedAt?: string; readonly timeZone?: string } | null;
  readonly period: {
    readonly savedSeconds: number;
    readonly targetSeconds: number;
    readonly startsAt: number;
    readonly endsAt: number;
  } | null;
}

export interface PathAppearance {
  readonly revision: number;
  readonly color: 'coral' | 'lavender' | 'gold' | 'mint' | 'blue' | 'pink';
  readonly emoji: string;
}

export interface NewPath {
  readonly name: string;
  readonly visibility: 'private' | 'followers' | 'public';
  readonly intervalGoal?: { readonly targetSeconds: number; readonly recurrence: Recurrence };
  readonly overallTarget?: { readonly targetSeconds: number };
}
