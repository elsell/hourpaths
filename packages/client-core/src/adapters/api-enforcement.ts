import { createSessionApiClient } from '@hourpaths/api-client';
import { EnforcementFailure, type EnforcementAppeal, type EnforcementNotice, type EnforcementRepository } from '../enforcement';

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new EnforcementFailure();
  return value as Record<string, unknown>;
}
function text(value: unknown): string { if (typeof value !== 'string') throw new EnforcementFailure(); return value; }
function date(value: unknown): string { const result = text(value); if (!Number.isFinite(Date.parse(result))) throw new EnforcementFailure(); return result; }
export function enforcementAppealFromAPI(value: unknown): EnforcementAppeal {
  const row = object(value), id = text(row.id);
  if (!id || id.trim() !== id) throw new EnforcementFailure();
  const result: EnforcementAppeal = { id, explanation: text(row.explanation), submittedAt: date(row.submittedAt) };
  if (row.outcome !== undefined) {
    if (row.outcome !== 'upheld' && row.outcome !== 'reversed') throw new EnforcementFailure();
    result.outcome = row.outcome; result.decisionReason = text(row.decisionReason); result.decidedAt = date(row.decidedAt);
    if (!result.decisionReason.trim() || Date.parse(result.decidedAt) < Date.parse(result.submittedAt)) throw new EnforcementFailure();
  } else if (row.decisionReason !== undefined || row.decidedAt !== undefined) throw new EnforcementFailure();
  return result;
}
export function enforcementNoticeFromAPI(value: unknown): EnforcementNotice {
  const row = object(value), id = text(row.id), action = row.action;
  if (!id || id.trim() !== id || !['warning', 'content_removal', 'suspension', 'ban'].includes(String(action))) throw new EnforcementFailure();
  const result: EnforcementNotice = { id, action: action as EnforcementNotice['action'], policyReason: text(row.policyReason), issuedAt: date(row.issuedAt), appealDeadline: date(row.appealDeadline) };
  if (!result.policyReason.trim() || Date.parse(result.appealDeadline) <= Date.parse(result.issuedAt)) throw new EnforcementFailure();
  if (action === 'suspension') { result.until = date(row.until); if (Date.parse(result.until) <= Date.parse(result.issuedAt)) throw new EnforcementFailure(); }
  else if (row.until !== undefined) throw new EnforcementFailure();
  if (row.appeal !== undefined) result.appeal = enforcementAppealFromAPI(row.appeal);
  return result;
}
function requireSuccess(response: Response, data: unknown): void {
  if (response.ok && data) return;
  const status = response.status;
  throw new EnforcementFailure(status === 401 || status === 403 ? 'rejected' : status === 404 ? 'not_found' : status === 409 ? 'conflict' : status === 400 || status === 422 ? 'invalid' : 'unavailable');
}
export function apiEnforcement(baseURL: string, token: string | null, rejected?: (token: string | null) => void, signal?: AbortSignal): EnforcementRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async list(cursor) {
      const result = await api.listEnforcementNotices(cursor); requireSuccess(result.response, result.data);
      if (!result.data || !Array.isArray(result.data.data.items)) throw new EnforcementFailure();
      return { items: result.data.data.items.map(enforcementNoticeFromAPI), ...(result.data.meta.nextCursor ? { nextCursor: result.data.meta.nextCursor } : {}) };
    },
    async get(id) {
      const result = await api.getEnforcementNotice(id); requireSuccess(result.response, result.data);
      return enforcementNoticeFromAPI(result.data?.data);
    },
    async appeal(id, explanation, key) {
      const result = await api.submitEnforcementAppeal(id, explanation, key); requireSuccess(result.response, result.data);
      return enforcementAppealFromAPI(result.data?.data);
    },
  };
}
