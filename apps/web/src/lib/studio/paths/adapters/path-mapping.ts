import type { SessionPath, TimerState } from '@hourpaths/api-client';
import { pathVisibilityFromAPI } from '@hourpaths/client-core';
import type { Path, TrackingSnapshot } from '../domain/path';

export class PathRequestError extends Error {
  constructor(readonly status: number) { super('path_request_failed'); }
}
function timestamp(value: string): number {
  const result = Date.parse(value);
  if (!Number.isFinite(result)) throw new PathRequestError(502);
  return result;
}
function seconds(value: number): number {
  if (!Number.isFinite(value) || value < 0) throw new PathRequestError(502);
  return value;
}
export function pathFromAPI(dto: SessionPath): Path {
  if (!dto.id || typeof dto.name !== 'string' || !dto.capabilities) throw new PathRequestError(502);
  return {
    id: dto.id, name: dto.name, visibility: pathVisibilityFromAPI(dto.visibility), canManageVisibility: dto.capabilities.manageVisibility === true, archived: Boolean(dto.archivedAt),
    pinned: dto.home?.pinned ?? false, position: dto.home?.manualPosition ?? null,
    pinnedPosition: dto.home?.pinnedPosition ?? null, recentActivityAt: dto.home?.recentActivityAt ? timestamp(dto.home.recentActivityAt) : 0,
    goal: dto.intervalGoal ? { targetSeconds: seconds(dto.intervalGoal.targetSeconds), recurrence: dto.intervalGoal.recurrence, alignment: { minute: dto.intervalGoal.alignment.minute, hour: dto.intervalGoal.alignment.hour, day: dto.intervalGoal.alignment.day, month: dto.intervalGoal.alignment.month, isoWeekday: dto.intervalGoal.alignment.isoWeekday } } : null,
    overallTarget: dto.overallTarget ? seconds(dto.overallTarget.targetSeconds) : null,
    canManageGoals: dto.capabilities.manageGoals,
    canLeave: dto.capabilities.leavePath === true,
    canTransferOwnership: dto.capabilities.transferOwnership === true,
    canInvite: dto.capabilities.inviteMembers === true,
    canManageLifecycle: dto.capabilities.manageLifecycle === true,
    canTrack: dto.capabilities.trackTime, canEdit: dto.capabilities.renamePath,
  };
}
export function trackingFromAPI(dto: TimerState): TrackingSnapshot {
  if (dto.running && !dto.timer) throw new PathRequestError(502);
  const period = dto.intervalProgress;
  if (dto.timer && !dto.timer.id) throw new PathRequestError(502);
  if (period?.startedAt && period.endedAt && timestamp(period.endedAt) <= timestamp(period.startedAt)) throw new PathRequestError(502);
  return {
    savedTotalSeconds: seconds(dto.accumulatedSeconds),
    activeSession: dto.running && dto.timer ? { id: dto.timer.id, startedAt: timestamp(dto.timer.startedAt), originalStartedAt: dto.timer.startedAt, timeZone: dto.timer.occurrenceTimeZone } : null,
    period: period?.startedAt && period.endedAt ? {
      savedSeconds: seconds(period.accumulatedSeconds), targetSeconds: seconds(period.targetSeconds),
      startsAt: timestamp(period.startedAt), endsAt: timestamp(period.endedAt),
    } : null,
  };
}
