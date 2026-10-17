import { createSupportApiClient, type TokenProvider } from '@hourpaths/api-client';
export type SupportMessage = { id: string; body: string; operator: boolean; createdAt: string };
export type SupportConversation = { id: string; subject: string; createdAt: string; expiresAt: string; messages: SupportMessage[] };
export type SupportNotice = { id: string; requestId: string; createdAt: string; read: boolean };
export interface SupportPort {
 create(id: string, key: string, subject: string, message: string): Promise<SupportConversation>;
 read(id: string, key: string): Promise<SupportConversation>;
 reply(id: string, key: string, messageId: string, message: string): Promise<SupportConversation>;
 inbox(cursor?: string): Promise<{ items: SupportConversation[]; nextCursor: string }>;
 adminRead(id: string): Promise<SupportConversation>;
 adminReply(id: string, messageId: string, message: string): Promise<SupportConversation>;
 notices(): Promise<SupportNotice[]>;
}
export class SupportFailure extends Error { constructor(readonly status: number) { super('support_request_failed'); } }
function required<T>(r: { response: Response; data?: { data: T } }): T { if (!r.response.ok || !r.data) throw new SupportFailure(r.response.status); return r.data.data; }
function conversation(c: SupportConversation): SupportConversation { return { id: c.id, subject: c.subject, createdAt: c.createdAt, expiresAt: c.expiresAt, messages: c.messages.map(m => ({ id: m.id, body: m.body, operator: m.operator, createdAt: m.createdAt })) }; }
export function apiSupport(baseURL: string, token: TokenProvider = () => null): SupportPort {
 const publicAPI = createSupportApiClient(baseURL, () => null), admin = createSupportApiClient(baseURL, token);
 return {
  async create(id,key,subject,message) { return conversation(required(await publicAPI.create(id,key,subject,message))); },
  async read(id,key) { return conversation(required(await publicAPI.read(id,key))); },
  async reply(id,key,messageId,message) { return conversation(required(await publicAPI.reply(id,key,messageId,message))); },
  async inbox(cursor) { const r=await admin.inbox(cursor);return {items:required(r).items.map(conversation),nextCursor:r.data?.meta.nextCursor??''}; },
  async adminRead(id) { return conversation(required(await admin.adminRead(id))); },
  async adminReply(id,messageId,message) { return conversation(required(await admin.adminReply(id,messageId,message))); },
  async notices() { return required(await admin.notices()).items.map(n=>({id:n.id,requestId:n.requestId,createdAt:n.createdAt,read:n.read})); },
 };
}
