import { createSessionApiClient } from '@hourpaths/api-client';
import { AccountExportFailure, type AccountExportRepository, type ExportPath } from '../account-export';

export function apiAccountExport(baseURL: string, credential: () => string | null, rejected?: (token: string | null) => void): AccountExportRepository {
  function session() {
    const token = credential();
    if (!token) throw new AccountExportFailure('account_changed');
    return {
      api: createSessionApiClient(baseURL, () => token, undefined, rejected),
      current() { if (credential() !== token) throw new AccountExportFailure('account_changed'); },
    };
  }
  return {
    async profile() {
      const current = session(), response = await current.api.exportOwnProfile(); current.current();
      if (!response.response.ok || !response.data) throw new AccountExportFailure();
      const p = response.data.data;
      return { id: p.userId, email: p.email, username: p.username, displayName: p.displayName, description: p.description,
        visibility: p.visibility, timeZone: p.timeZone, firstDayOfWeek: p.firstDayOfWeek, createdAt: p.createdAt, updatedAt: p.updatedAt };
    },
    async paths(archived, cursor) {
      const current = session(), response = await current.api.exportOwnPaths(cursor, archived); current.current();
      if (!response.response.ok || !response.data) throw new AccountExportFailure();
      const items: ExportPath[] = response.data.data.map(row => {
        const p = row.path, alignment = p.intervalGoal?.alignment;
        return { id: p.id, name: p.name, visibility: p.visibility, archivedAt: p.archivedAt ?? null,
          createdAt: row.createdAt, updatedAt: row.updatedAt, overallTargetSeconds: p.overallTarget?.targetSeconds ?? null,
          intervalGoal: p.intervalGoal ? { targetSeconds: p.intervalGoal.targetSeconds, recurrence: p.intervalGoal.recurrence,
            alignment: { minute: alignment?.minute, hour: alignment?.hour, isoWeekday: alignment?.isoWeekday, month: alignment?.month, day: alignment?.day } } : null };
      });
      return { items, nextCursor: response.data.meta.nextCursor ?? '' };
    },
    async activities(pathId, cursor) {
      const current = session(), response = await current.api.exportOwnActivities(pathId, cursor); current.current();
      if (!response.response.ok || !response.data) throw new AccountExportFailure();
      return { items: response.data.data.map(row => {
        const a = row.activity;
        return { id: a.id, pathId: a.pathId, participantId: a.participantId, startedAt: a.startedAt, endedAt: a.endedAt,
          timeZone: a.occurrenceTimeZone, note: a.note ?? '', createdAt: a.createdAt, updatedAt: a.updatedAt, version: row.version };
      }), nextCursor: response.data.meta.nextCursor ?? '' };
    },
  };
}
