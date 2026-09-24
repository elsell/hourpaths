export type ProfileRelationship = 'self' | 'none' | 'requested' | 'following';

export type PublicProfile = Readonly<{
  userId: string;
  username: string;
  displayName: string;
  profilePictureUrl?: string;
  description?: string;
  followerCount: number;
  followingCount: number;
  relationship: ProfileRelationship;
}>;

export type ProfileSearchState = Readonly<{
  query: string;
  items: readonly PublicProfile[];
  nextCursor: string;
}>;

export type ProfileSearchPage = Readonly<{
  items: readonly PublicProfile[];
  nextCursor: string;
}>;

export type ProfileSearchResult =
  | Readonly<{ kind: 'cleared'; state: ProfileSearchState }>
  | Readonly<{ kind: 'loaded'; state: ProfileSearchState }>
  | Readonly<{ kind: 'superseded' }>
  | Readonly<{ kind: 'failed'; reason: 'invalid_response' | 'request' | 'stale' }>;

const emptyProfileSearchState = (): ProfileSearchState => Object.freeze({
  query: '',
  items: Object.freeze([]),
  nextCursor: '',
});

function validText(value: unknown, maximum: number): value is string {
  return typeof value === 'string' && value.length > 0 && value.length <= maximum &&
    value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function validCursor(value: unknown): value is string {
  return typeof value === 'string' && value.length <= 4096 && value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function validCount(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0;
}

function validRelationship(value: unknown): value is ProfileRelationship {
  return value === 'self' || value === 'none' || value === 'requested' || value === 'following';
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

function exactKeys(record: Record<string, unknown>, required: readonly string[]): boolean {
  const optional = new Set(['description', 'profilePictureUrl']);
  const keys = Object.keys(record);
  return required.every((key) => keys.includes(key)) &&
    keys.every((key) => required.includes(key) || optional.has(key));
}

function exactEnvelopeKeys(record: Record<string, unknown>, required: readonly string[]): boolean {
  const keys = Object.keys(record);
  return required.every((key) => keys.includes(key)) &&
    keys.every((key) => required.includes(key) || key === '$schema') &&
    (record.$schema === undefined || validText(record.$schema, 2048));
}

function publicProfile(value: unknown): PublicProfile | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  const required = [
    'displayName', 'followerCount', 'followingCount', 'relationship', 'userId', 'username',
  ] as const;
  if (
    !exactKeys(record, required) ||
    !validText(record.userId, 256) ||
    !validText(record.username, 64) ||
    !/^[A-Za-z0-9_.]{3,64}$/u.test(record.username) ||
    !validText(record.displayName, 100) ||
    !validCount(record.followerCount) ||
    !validCount(record.followingCount) ||
    !validRelationship(record.relationship) ||
    (record.description !== undefined && !validText(record.description, 500)) ||
    (record.profilePictureUrl !== undefined && !safePictureURL(record.profilePictureUrl))
  ) return undefined;
  return Object.freeze({
    userId: record.userId,
    username: record.username,
    displayName: record.displayName,
    ...(record.profilePictureUrl === undefined ? {} : { profilePictureUrl: record.profilePictureUrl }),
    ...(record.description === undefined ? {} : { description: record.description }),
    followerCount: record.followerCount,
    followingCount: record.followingCount,
    relationship: record.relationship,
  });
}

function publicProfileTransport(value: unknown): PublicProfile | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  if (!exactKeys(record, ['displayName', 'followerCount', 'followingCount', 'id', 'relationship', 'username'])) {
    return undefined;
  }
  const { id, ...projection } = record;
  return publicProfile({ userId: id, ...projection });
}

export function profileSearchPageFromAPI(value: unknown): ProfileSearchPage {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid profile response');
  }
  const envelope = value as Record<string, unknown>;
  if (
    !exactEnvelopeKeys(envelope, ['data', 'meta']) ||
    !Array.isArray(envelope.data) ||
    !envelope.meta || typeof envelope.meta !== 'object' || Array.isArray(envelope.meta)
  ) throw new Error('invalid profile response');
  const meta = envelope.meta as Record<string, unknown>;
  if (
    Object.keys(meta).some((key) => key !== 'nextCursor') ||
    (meta.nextCursor !== undefined && !validCursor(meta.nextCursor))
  ) throw new Error('invalid profile response');
  const items = envelope.data.map(publicProfileTransport);
  if (items.some((item) => item === undefined)) throw new Error('invalid profile response');
  return Object.freeze({
    items: Object.freeze(items as PublicProfile[]),
    nextCursor: (meta.nextCursor as string | undefined) ?? '',
  });
}

export function publicProfileFromAPI(value: unknown): PublicProfile {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid profile response');
  }
  const envelope = value as Record<string, unknown>;
  if (!exactEnvelopeKeys(envelope, ['data'])) throw new Error('invalid profile response');
  const profile = publicProfileTransport(envelope.data);
  if (!profile) throw new Error('invalid profile response');
  return profile;
}

export function profileSearchQuery(value: string): string | undefined {
  const query = value.normalize('NFC').trim();
  const nonWhitespaceCharacters = Array.from(query).filter((character) => !/\s/u.test(character));
  return nonWhitespaceCharacters.length >= 2 ? query : undefined;
}

export function mergeProfileSearchPage(
  state: ProfileSearchState,
  value: unknown,
  requestedQuery: string,
  requestedCursor: string,
): ProfileSearchState {
  const query = profileSearchQuery(requestedQuery);
  if (!query || query !== state.query) throw new Error('stale profile query');
  if (!validCursor(requestedCursor) || requestedCursor !== state.nextCursor) {
    throw new Error('stale profile cursor');
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid profile page');
  }
  const page = value as Record<string, unknown>;
  if (
    Object.keys(page).sort().join(',') !== 'items,nextCursor' ||
    !Array.isArray(page.items) ||
    !validCursor(page.nextCursor)
  ) throw new Error('invalid profile page');

  const items = page.items.map((item) => {
    const validated = publicProfile(item);
    if (!validated) throw new Error('invalid public profile');
    return validated;
  });
  const pageIDs = new Set(items.map(({ userId }) => userId));
  if (pageIDs.size !== items.length) throw new Error('invalid profile page');

  const incoming = new Map(items.map((item) => [item.userId, item]));
  return Object.freeze({
    query,
    items: Object.freeze([
      ...state.items.map((item) => incoming.get(item.userId) ?? item),
      ...items.filter((item) => !state.items.some(({ userId }) => userId === item.userId)),
    ]),
    nextCursor: page.nextCursor,
  });
}

export function createProfileSearchOwner() {
  let epoch = 0;
  let current = emptyProfileSearchState();

  async function requestPage(
    query: string,
    cursor: string,
    state: ProfileSearchState,
    request: (query: string, cursor: string) => Promise<unknown>,
  ): Promise<ProfileSearchResult> {
    const operationEpoch = ++epoch;
    try {
      const value = await request(query, cursor);
      if (operationEpoch !== epoch) return { kind: 'superseded' };
      try {
        current = mergeProfileSearchPage(state, value, query, cursor);
      } catch {
        return { kind: 'failed', reason: 'invalid_response' };
      }
      return { kind: 'loaded', state: current };
    } catch {
      return operationEpoch === epoch
        ? { kind: 'failed', reason: 'request' }
        : { kind: 'superseded' };
    }
  }

  return {
    async search(
      value: string,
      request: (query: string, cursor: string) => Promise<unknown>,
    ): Promise<ProfileSearchResult> {
      const query = profileSearchQuery(value);
      if (!query) {
        epoch += 1;
        current = emptyProfileSearchState();
        return { kind: 'cleared', state: current };
      }
      const state: ProfileSearchState = Object.freeze({
        query,
        items: Object.freeze([]),
        nextCursor: '',
      });
      return requestPage(query, '', state, request);
    },
    async loadMore(
      state: ProfileSearchState,
      request: (query: string, cursor: string) => Promise<unknown>,
    ): Promise<ProfileSearchResult> {
      if (
        !state.nextCursor || state.query !== current.query ||
        state.nextCursor !== current.nextCursor || state.items !== current.items
      ) return { kind: 'failed', reason: 'stale' };
      return requestPage(state.query, state.nextCursor, state, request);
    },
    cancel(): void {
      epoch += 1;
      current = emptyProfileSearchState();
    },
  };
}
