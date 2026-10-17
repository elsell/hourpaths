import { createSupportApiClient, type TokenProvider } from '@hourpaths/api-client';
import { SupportFailure } from './support';
export type AdminAttentionNotice = { id: string; kind: 'report' | 'appeal'; createdAt: string; read: boolean };
export type AdminAttentionRecord = { id: string; kind: 'report' | 'appeal'; createdAt: string; reason: string; explanation: string; state: string };
export interface AdminAttentionPort {
 notices(): Promise<AdminAttentionNotice[]>;
 read(id: string): Promise<AdminAttentionRecord>;
}
export function apiAdminAttention(baseURL: string, token: TokenProvider): AdminAttentionPort {
 const api = createSupportApiClient(baseURL, token);
 return {
  async notices() {
   const r = await api.moderationNotices();
   if (!r.response.ok || !r.data) throw new SupportFailure(r.response.status);
   return r.data.data.items.map(n => ({ id: n.id, kind: n.kind, createdAt: n.createdAt, read: n.read }));
  },
  async read(id) {
   const r = await api.moderationRead(id);
   if (!r.response.ok || !r.data) throw new SupportFailure(r.response.status);
   const v = r.data.data;
   return { id: v.id, kind: v.kind, createdAt: v.createdAt, reason: v.reason, explanation: v.explanation, state: v.state };
  },
 };
}
const reportReasonKeys = {
 spam_or_scam: 'reporting.reason.spam_or_scam',
 harassment_or_bullying: 'reporting.reason.harassment_or_bullying',
 hate_or_abusive_content: 'reporting.reason.hate_or_abusive_content',
 sexual_or_inappropriate_content: 'reporting.reason.sexual_or_inappropriate_content',
 impersonation: 'reporting.reason.impersonation',
 privacy_or_personal_information: 'reporting.reason.privacy_or_personal_information',
 dangerous_or_self_harm_content: 'reporting.reason.dangerous_or_self_harm_content',
 something_else: 'reporting.reason.something_else',
} as const;
export function adminReportReasonKey(reason: string) {
 return reportReasonKeys[reason as keyof typeof reportReasonKeys] ?? 'reporting.reason.something_else';
}
