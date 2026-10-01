import { createLeaveRecovery, sharedLeaveCommands } from './shared-leave-commands';
import { createSessionApiClient, type SessionPath, type TimerState } from '@hourpaths/api-client';
import type { Path, PathGoals, TrackingSnapshot } from '../domain/path';
import { orderPaths } from '../domain/order';
import type { PathRepository } from '../ports/path-repository';

export class PathRequestError extends Error {
  constructor(readonly status: number) { super('path_request_failed'); }
}
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new PathRequestError(result.response.status);
  return result.data.data;
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
    id: dto.id, name: dto.name, archived: Boolean(dto.archivedAt),
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
    activeSession: dto.running && dto.timer ? { id: dto.timer.id, startedAt: timestamp(dto.timer.startedAt) } : null,
    period: period?.startedAt && period.endedAt ? {
      savedSeconds: seconds(period.accumulatedSeconds), targetSeconds: seconds(period.targetSeconds),
      startsAt: timestamp(period.startedAt), endsAt: timestamp(period.endedAt),
    } : null,
  };
}
export function apiPathRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): PathRepository {
  const leaveRecovery = createLeaveRecovery();
  const accepted = <T>(result: { data?: { data: T }; response: Response }): T => {
    return required(result);
  };
  const client = createSessionApiClient(baseURL, token, undefined, rejected);
  const tracking = async (pathId: string, signal?: AbortSignal) =>
    trackingFromAPI(accepted(await createSessionApiClient(baseURL, token, signal, rejected).currentTimer(pathId)));
  return {
    leaveCommands: key => sharedLeaveCommands(signal => createSessionApiClient(baseURL, token, signal, rejected), key, leaveRecovery),
    async read(pathId, signal) {
      const value = pathFromAPI(accepted(await createSessionApiClient(baseURL, token, signal, rejected).path(pathId)));
      if (value.id !== pathId) throw new PathRequestError(502);
      return value;
    },
    async lifecycle(review) {
      if (review.action === 'delete') {
        const result = accepted(await client.deletePath(review.pathId, { confirmed: true, expectedName: review.name }, review.operationId));
        if (result.pathId !== review.pathId || result.deleted !== true) throw new PathRequestError(502);
      } else {
        const result = accepted(await client.setPathArchiveState(review.pathId, { confirmed: true, expectedArchived: review.expectedArchived, archived: review.action === 'archive' }, review.operationId));
        if (result.id !== review.pathId || Boolean(result.archivedAt) !== (review.action === 'archive')) throw new PathRequestError(502);
      }
    },
    async saveGoals(path, goals, operationId) {
      const encode = (value: PathGoals) => ({
        ...(value.goal ? { intervalGoal: { targetSeconds: value.goal.targetSeconds, recurrence: value.goal.recurrence, alignment: { ...value.goal.alignment } } } : {}),
        ...(value.overallTarget !== null ? { overallTarget: { targetSeconds: value.overallTarget } } : {}),
      });
      const result = accepted(await client.updatePathGoals(path.id, { ...encode(goals), expectedGoals: encode(path), confirmed: true }, operationId));
      return pathFromAPI(result.path);
    },
    async create(path, operationId) { return pathFromAPI(accepted(await client.createPath({ ...path }, operationId))); },
    async rename(path, name, operationId) { return pathFromAPI(accepted(await client.renamePath(path.id, { name, expectedName: path.name }, operationId))); },
    async reorder(paths, operationId) {
      const response = await client.paths();
      accepted(response);
      const preferences = response.data!.meta.homePreferences;
      const ids = paths.map(path => path.id);
      const remaining = preferences.manualPathIds.filter(id => !ids.includes(id));
      const pinned = paths.filter(path => path.pinned).map(path => path.id);
      required(await client.updateHomePreferences({ expectedRevision: preferences.revision, orderMethod: 'manual',
        manualPathIds: [...ids, ...remaining], pinnedPathIds: [...pinned, ...preferences.pinnedPathIds.filter(id => !ids.includes(id))] }, operationId));
    },
    async pin(pathId, pinned, operationId) {
      const response = await client.paths();
      accepted(response);
      const preferences = response.data!.meta.homePreferences;
      const pinnedIds = preferences.pinnedPathIds.filter(id => id !== pathId);
      if (pinned) pinnedIds.push(pathId);
      accepted(await client.updateHomePreferences({ expectedRevision: preferences.revision,
        orderMethod: preferences.orderMethod, manualPathIds: preferences.manualPathIds, pinnedPathIds: pinnedIds }, operationId));
    },
    async saveAppearance(pathId, appearance, operationId) {
      accepted(await client.savePathAppearance(pathId, { color: appearance.color, emoji: appearance.emoji, expectedRevision: appearance.revision }, operationId));
    },
    async appearance(pathID, signal) {
      const dto = accepted(await createSessionApiClient(baseURL, token, signal, rejected).pathAppearance(pathID));
      const colors = ['coral', 'lavender', 'gold', 'mint', 'blue', 'pink'] as const;
      const hash = Array.from(pathID).reduce((value, character) => (value * 31 + character.codePointAt(0)!) >>> 0, 0);
      if (dto.revision === 0) return { color: colors[hash % colors.length]!, emoji: '✨', revision: 0 };
      if (!dto.color || !colors.includes(dto.color) || !dto.emoji) throw new PathRequestError(502);
      return { color: dto.color, emoji: dto.emoji, revision: dto.revision };
    },
    async list(archived, signal) {
      const paths: Path[] = [];
      let cursor: string | undefined;
      let order: 'recent' | 'alphabetical' | 'manual' = 'recent';
      const seen = new Set<string>();
      do {
        const reader = createSessionApiClient(baseURL, token, signal, rejected);
        const response = await (archived ? reader.archivedPaths(cursor) : reader.paths(cursor));
        paths.push(...accepted(response).map(pathFromAPI));
        order = response.data!.meta.homePreferences.orderMethod;
        cursor = response.data?.meta.nextCursor;
        if (cursor && seen.has(cursor)) throw new PathRequestError(502);
        if (cursor) seen.add(cursor);
      } while (cursor);
      return orderPaths(paths, order);
    },
    tracking,
    async start(pathId, operationId) {
      const dto = accepted(await client.startTimer(pathId, operationId));
      return trackingFromAPI(dto);
    },
    async stop(pathId, sessionId, operationId) {
      accepted(await client.stopTimer(pathId, sessionId, operationId));
      return tracking(pathId);
    },
  };
}
