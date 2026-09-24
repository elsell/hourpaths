export type BlockableIdentity = Readonly<{
  userId: string;
  username: string;
  displayName: string;
  profilePictureUrl?: string;
}>;

export type SharedPathSummary = Readonly<{ id: string; name: string }>;

export type BlockReviewAcknowledgement = Readonly<{
  version: 1;
  token: string;
  expiresAt: string;
}>;

export type BlockReview = Readonly<{
  target: BlockableIdentity;
  sharedPaths: readonly SharedPathSummary[];
  acknowledgement: BlockReviewAcknowledgement;
}>;

export type BlockResult = Readonly<{ blocked: true; target: BlockableIdentity }>;

export type BlockedAccount = Readonly<{
  identity: BlockableIdentity;
  blockedAt: string;
}>;

export type BlockedAccountPage = Readonly<{
  items: readonly BlockedAccount[];
  nextCursor: string;
}>;

export type UnblockResult = Readonly<{ blocked: false; target: BlockableIdentity }>;

export interface UserBlockingPort {
  reviewBlock(username: string): Promise<BlockReview>;
  blockUser(username: string, idempotencyKey: string, acknowledgement: BlockReviewAcknowledgement): Promise<BlockResult>;
  listBlockedAccounts(cursor?: string): Promise<BlockedAccountPage>;
  unblockUser(userId: string, idempotencyKey: string): Promise<UnblockResult>;
}

function exactKeys(record: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []): boolean {
  const allowed = new Set([...required, ...optional]);
  return required.every((key) => Object.hasOwn(record, key)) &&
    Object.keys(record).every((key) => allowed.has(key));
}

function validText(value: unknown, maximum: number): value is string {
  return typeof value === 'string' && value.length > 0 && value.length <= maximum &&
    value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function validCursor(value: unknown): value is string {
  return typeof value === 'string' && value.length <= 4096 && value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function safePictureURL(value: unknown): value is string {
  if (!validText(value, 2048)) return false;
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && !url.username && !url.password;
  } catch {
    return false;
  }
}

function identity(value: unknown): BlockableIdentity | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  if (
    !exactKeys(record, ['displayName', 'userId', 'username'], ['profilePictureUrl']) ||
    !validText(record.userId, 256) ||
    !validText(record.username, 64) ||
    !/^[A-Za-z0-9_.]{3,64}$/u.test(record.username) ||
    !validText(record.displayName, 100) ||
    (record.profilePictureUrl !== undefined && !safePictureURL(record.profilePictureUrl))
  ) return undefined;
  return Object.freeze({
    userId: record.userId,
    username: record.username,
    displayName: record.displayName,
    ...(record.profilePictureUrl === undefined ? {} : { profilePictureUrl: record.profilePictureUrl }),
  });
}

function envelopeData(value: unknown, failure: string): unknown {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(failure);
  const envelope = value as Record<string, unknown>;
  if (!exactKeys(envelope, ['data'], ['$schema'])) throw new Error(failure);
  return envelope.data;
}

export function blockReviewFromAPI(value: unknown): BlockReview {
  const raw = envelopeData(value, 'invalid block review');
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) throw new Error('invalid block review');
  const data = raw as Record<string, unknown>;
  const target = identity(data.target);
  const rawAcknowledgement = data.acknowledgement;
  if (!exactKeys(data, ['acknowledgement', 'sharedPaths', 'target']) || !target || !Array.isArray(data.sharedPaths) ||
    !rawAcknowledgement || typeof rawAcknowledgement !== 'object' || Array.isArray(rawAcknowledgement)) {
    throw new Error('invalid block review');
  }
  const sharedPaths = data.sharedPaths.map((value) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
    const path = value as Record<string, unknown>;
    return exactKeys(path, ['id', 'name']) && validText(path.id, 256) && validText(path.name, 100)
      ? Object.freeze({ id: path.id, name: path.name })
      : undefined;
  });
  if (sharedPaths.some((path) => !path) || new Set(sharedPaths.map((path) => path?.id)).size !== sharedPaths.length) {
    throw new Error('invalid block review');
  }
  const acknowledgement = rawAcknowledgement as Record<string, unknown>;
  if (!exactKeys(acknowledgement, ['expiresAt', 'token', 'version']) || acknowledgement.version !== 1 ||
    !validText(acknowledgement.token, 8192) || typeof acknowledgement.expiresAt !== 'string' ||
    !Number.isFinite(Date.parse(acknowledgement.expiresAt))) throw new Error('invalid block review');
  return Object.freeze({
    target,
    sharedPaths: Object.freeze(sharedPaths as SharedPathSummary[]),
    acknowledgement: Object.freeze({ version: 1 as const, token: acknowledgement.token, expiresAt: acknowledgement.expiresAt }),
  });
}

export function blockResultFromAPI(value: unknown): BlockResult {
  const raw = envelopeData(value, 'invalid block result');
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) throw new Error('invalid block result');
  const data = raw as Record<string, unknown>;
  const target = identity(data.target);
  if (!exactKeys(data, ['blocked', 'target']) || data.blocked !== true || !target) {
    throw new Error('invalid block result');
  }
  return Object.freeze({ blocked: true, target });
}

export function blockedAccountPageFromAPI(value: unknown): BlockedAccountPage {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid blocked accounts');
  const envelope = value as Record<string, unknown>;
  if (!exactKeys(envelope, ['data', 'meta'], ['$schema']) || !Array.isArray(envelope.data) ||
    !envelope.meta || typeof envelope.meta !== 'object' || Array.isArray(envelope.meta)) {
    throw new Error('invalid blocked accounts');
  }
  const meta = envelope.meta as Record<string, unknown>;
  if (!exactKeys(meta, [], ['nextCursor']) ||
    (meta.nextCursor !== undefined && !validCursor(meta.nextCursor))) {
    throw new Error('invalid blocked accounts');
  }
  const items = envelope.data.map((value) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
    const record = value as Record<string, unknown>;
    const { blockedAt, ...identityValue } = record;
    const parsedIdentity = identity(identityValue);
    return exactKeys(record, ['blockedAt', 'displayName', 'userId', 'username'], ['profilePictureUrl']) &&
      parsedIdentity && typeof blockedAt === 'string' && Number.isFinite(Date.parse(blockedAt))
      ? Object.freeze({ identity: parsedIdentity, blockedAt })
      : undefined;
  });
  if (items.some((item) => !item) || new Set(items.map((item) => item?.identity.userId)).size !== items.length) {
    throw new Error('invalid blocked accounts');
  }
  return Object.freeze({
    items: Object.freeze(items as BlockedAccount[]),
    nextCursor: (meta.nextCursor as string | undefined) ?? '',
  });
}

export function unblockResultFromAPI(value: unknown): UnblockResult {
  const raw = envelopeData(value, 'invalid unblock result');
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) throw new Error('invalid unblock result');
  const data = raw as Record<string, unknown>;
  const target = identity(data.target);
  if (!exactKeys(data, ['blocked', 'target']) || data.blocked !== false || !target) {
    throw new Error('invalid unblock result');
  }
  return Object.freeze({ blocked: false, target });
}

export function mergeBlockedAccountPage(
  state: BlockedAccountPage,
  page: BlockedAccountPage,
  requestedCursor: string,
): BlockedAccountPage {
  if (requestedCursor !== state.nextCursor) throw new Error('stale blocked-account cursor');
  const incoming = new Map(page.items.map((item) => [item.identity.userId, item]));
  return Object.freeze({
    items: Object.freeze([
      ...state.items.map((item) => incoming.get(item.identity.userId) ?? item),
      ...page.items.filter((item) => !state.items.some(({ identity }) => identity.userId === item.identity.userId)),
    ]),
    nextCursor: page.nextCursor,
  });
}
