import { createSessionApiClient } from '@hourpaths/api-client';
import { normalizeReportDraft, ReportFailure, type ReportReceipt, type ReportingRepository } from '../reporting';

export function apiReporting(baseURL: string, token: string | null, rejected?: (token: string | null) => void, signal?: AbortSignal): ReportingRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async submit(draft, key) {
      const value = normalizeReportDraft(draft);
      const result = await api.submitReport({ targetKind: value.target.kind, targetId: value.target.id, reason: value.reason, explanation: value.explanation }, key);
      if (!result.response.ok || !result.data) {
        const status = result.response.status;
        throw new ReportFailure(status === 400 || status === 422 ? 'invalid' : status === 404 ? 'not_found' : status === 401 || status === 403 ? 'rejected' : 'unavailable');
      }
      return reportReceiptFromAPI(result.data.data);
    },
  };
}

export function reportReceiptFromAPI(value: unknown): ReportReceipt {
  if (!value || typeof value !== 'object') throw new ReportFailure();
  const row = value as Partial<ReportReceipt>;
  if (typeof row.id !== 'string' || !row.id || row.id.trim() !== row.id) throw new ReportFailure();
  const target = row.blockTarget;
  if (target && (typeof target.userId !== 'string' || !target.userId || typeof target.username !== 'string' || !target.username || typeof target.displayName !== 'string' || !target.displayName)) throw new ReportFailure();
  return { id: row.id, ...(target ? { blockTarget: { userId: target.userId, username: target.username, displayName: target.displayName } } : {}) };
}
