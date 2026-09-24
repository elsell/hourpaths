import { publicProfileFromAPI, type PublicProfile } from './profile-discovery';

export type FollowRequest = Readonly<{
  id: string;
  requester: PublicProfile;
  createdAt: string;
}>;

export type FollowRequestState = Readonly<{
  items: readonly FollowRequest[];
  nextCursor: string;
}>;

export type RelationshipMutationResult = Readonly<{
  profile: PublicProfile;
  requestId?: string;
}>;

export type FollowRequestReviewResult = Readonly<{
  request: FollowRequest;
  decision: 'accepted' | 'rejected';
}>;

function exactKeys(record: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []): boolean {
  const allowed = new Set([...required, ...optional]);
  return required.every((key) => Object.hasOwn(record, key)) && Object.keys(record).every((key) => allowed.has(key));
}

function validOpaqueText(value: unknown, maximum: number): value is string {
  return typeof value === 'string' && value.length > 0 && value.length <= maximum && value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function followRequest(value: unknown): FollowRequest | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  if (!exactKeys(record, ['createdAt', 'id', 'requester']) || !validOpaqueText(record.id, 128) ||
    typeof record.createdAt !== 'string' || !Number.isFinite(Date.parse(record.createdAt))) return undefined;
  let requester: PublicProfile;
  try {
    requester = publicProfileFromAPI({ data: record.requester });
  } catch {
    return undefined;
  }
  if (requester.relationship === 'self') return undefined;
  return Object.freeze({ id: record.id, requester, createdAt: record.createdAt });
}

export function relationshipMutationResultFromAPI(value: unknown): RelationshipMutationResult {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid relationship response');
  const envelope = value as Record<string, unknown>;
  if (!exactKeys(envelope, ['data']) || !envelope.data || typeof envelope.data !== 'object' || Array.isArray(envelope.data)) {
    throw new Error('invalid relationship response');
  }
  const data = envelope.data as Record<string, unknown>;
  if (!exactKeys(data, ['profile'], ['requestId']) || (data.requestId !== undefined && !validOpaqueText(data.requestId, 128))) {
    throw new Error('invalid relationship response');
  }
  const profile = publicProfileFromAPI({ data: data.profile });
  if ((profile.relationship === 'requested') !== (data.requestId !== undefined)) throw new Error('invalid relationship response');
  return Object.freeze({ profile, ...(data.requestId === undefined ? {} : { requestId: data.requestId }) });
}

export function followRequestPageFromAPI(value: unknown): FollowRequestState {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid follow request response');
  const envelope = value as Record<string, unknown>;
  if (!exactKeys(envelope, ['data', 'meta']) || !Array.isArray(envelope.data) || !envelope.meta || typeof envelope.meta !== 'object' || Array.isArray(envelope.meta)) {
    throw new Error('invalid follow request response');
  }
  const meta = envelope.meta as Record<string, unknown>;
  if (!exactKeys(meta, [], ['nextCursor']) || (meta.nextCursor !== undefined && typeof meta.nextCursor !== 'string')) {
    throw new Error('invalid follow request response');
  }
  const items = envelope.data.map(followRequest);
  if (items.some((item) => !item) || new Set(items.map((item) => item?.id)).size !== items.length) {
    throw new Error('invalid follow request response');
  }
  return Object.freeze({ items: Object.freeze(items as FollowRequest[]), nextCursor: (meta.nextCursor as string | undefined) ?? '' });
}

export function followRequestReviewResultFromAPI(value: unknown): FollowRequestReviewResult {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid follow request review');
  const envelope = value as Record<string, unknown>;
  if (!exactKeys(envelope, ['data']) || !envelope.data || typeof envelope.data !== 'object' || Array.isArray(envelope.data)) {
    throw new Error('invalid follow request review');
  }
  const data = envelope.data as Record<string, unknown>;
  const request = followRequest(data.request);
  if (!exactKeys(data, ['decision', 'request']) || !request || (data.decision !== 'accepted' && data.decision !== 'rejected')) {
    throw new Error('invalid follow request review');
  }
  return Object.freeze({ request, decision: data.decision });
}

export function mergeFollowRequestPage(state: FollowRequestState, page: FollowRequestState, requestedCursor: string): FollowRequestState {
  if (requestedCursor !== state.nextCursor) throw new Error('stale follow request cursor');
  const incoming = new Map(page.items.map((item) => [item.id, item]));
  return Object.freeze({
    items: Object.freeze([
      ...state.items.map((item) => incoming.get(item.id) ?? item),
      ...page.items.filter((item) => !state.items.some(({ id }) => id === item.id)),
    ]),
    nextCursor: page.nextCursor,
  });
}

export function removeResolvedFollowRequest(state: FollowRequestState, requestId: string): FollowRequestState {
  if (!state.items.some(({ id }) => id === requestId)) throw new Error('unknown follow request');
  return Object.freeze({ ...state, items: Object.freeze(state.items.filter(({ id }) => id !== requestId)) });
}
