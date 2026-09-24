export type PracticeCommentAuthor = Readonly<{
  userId: string;
  username: string;
  displayName: string;
  profilePictureURL?: string;
}>;

export type PracticeComment = Readonly<{
  id: string;
  eventId: string;
  authorUserId: string;
  author: PracticeCommentAuthor;
  text: string;
  version: number;
  createdAt: string;
  updatedAt: string;
  edited: boolean;
  heartCount: number;
  heartedByViewer: boolean;
  heartPending: boolean;
  pending: boolean;
}>;

export type PracticeCommentPage = Readonly<{
  items: readonly PracticeComment[];
  nextCursor: string;
}>;

export type PracticeCommentVersion = Readonly<{ commentId: string; createdAt: string; text: string; version: number }>;
export type PracticeCommentHistoryPage = Readonly<{
  commentId: string;
  versions: readonly PracticeCommentVersion[];
  nextCursor: string;
}>;

function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid practice comment');
  return value as Record<string, unknown>;
}

function text(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value && !/[\u0000-\u0009\u000b-\u001f\u007f]/u.test(value);
}

function instant(value: unknown): value is string {
  return text(value) && Number.isFinite(Date.parse(value));
}

function exact(value: Record<string, unknown>, keys: readonly string[]) {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  return actual.length === expected.length && actual.every((key, index) => key === expected[index]);
}

function exactEnvelope(value: Record<string, unknown>, keys: readonly string[]): boolean {
  const allowed = new Set([...keys, '$schema']);
  return keys.every((key) => Object.hasOwn(value, key)) &&
    Object.keys(value).every((key) => allowed.has(key)) &&
    (value.$schema === undefined || (text(value.$schema) && value.$schema.length <= 2048));
}

function authorFromAPI(value: unknown): PracticeCommentAuthor {
  const candidate = record(value);
  const keys = ['displayName', 'followerCount', 'followingCount', 'id', 'relationship', 'username'];
  if (Object.hasOwn(candidate, 'description')) keys.push('description');
  if (Object.hasOwn(candidate, 'profilePictureUrl')) keys.push('profilePictureUrl');
  if (!exact(candidate, keys) || !text(candidate.id) || !text(candidate.username) || !text(candidate.displayName) ||
    !Number.isSafeInteger(candidate.followerCount) || (candidate.followerCount as number) < 0 ||
    !Number.isSafeInteger(candidate.followingCount) || (candidate.followingCount as number) < 0 ||
    !['self', 'none', 'requested', 'following'].includes(candidate.relationship as string) ||
    (candidate.description !== undefined && !text(candidate.description)) ||
    (candidate.profilePictureUrl !== undefined && !text(candidate.profilePictureUrl))) throw new Error('invalid practice comment');
  return Object.freeze({
    userId: candidate.id,
    username: candidate.username,
    displayName: candidate.displayName,
    ...(candidate.profilePictureUrl === undefined ? {} : { profilePictureURL: candidate.profilePictureUrl }),
  });
}

function commentFromAPI(
  value: unknown,
  author: PracticeCommentAuthor,
  heartCount: unknown,
  heartedByViewer: unknown,
): PracticeComment {
  const comment = record(value);
  if (!exact(comment, ['authorUserId', 'createdAt', 'edited', 'eventId', 'id', 'text', 'updatedAt', 'version']) ||
    !text(comment.id) || !text(comment.eventId) || !text(comment.authorUserId) || comment.authorUserId !== author.userId ||
    !text(comment.text) || !Number.isSafeInteger(comment.version) || (comment.version as number) < 1 ||
    !instant(comment.createdAt) || !instant(comment.updatedAt) || typeof comment.edited !== 'boolean' ||
    !Number.isSafeInteger(heartCount) || (heartCount as number) < 0 ||
    typeof heartedByViewer !== 'boolean' ||
    (heartedByViewer && heartCount === 0)) throw new Error('invalid practice comment');
  return Object.freeze({
    id: comment.id, eventId: comment.eventId, authorUserId: comment.authorUserId, author: Object.freeze({ ...author }),
    text: comment.text, version: comment.version as number, createdAt: comment.createdAt, updatedAt: comment.updatedAt,
    edited: comment.edited, heartCount: heartCount as number,
    heartedByViewer, heartPending: false, pending: false,
  });
}

export function practiceCommentFromAPI(value: unknown): PracticeComment {
  const item = record(value);
  if (!exact(item, ['author', 'comment', 'heartCount', 'heartedByViewer'])) throw new Error('invalid practice comment');
  const author = authorFromAPI(item.author);
  return commentFromAPI(item.comment, author, item.heartCount, item.heartedByViewer);
}

export function practiceCommentMutationFromAPI(
  value: unknown,
  author: PracticeCommentAuthor,
  currentHeart?: Pick<PracticeComment, 'heartCount' | 'heartedByViewer'>,
): PracticeComment {
  const candidate = record(author);
  const keys = Object.hasOwn(candidate, 'profilePictureURL')
    ? ['displayName', 'profilePictureURL', 'userId', 'username']
    : ['displayName', 'userId', 'username'];
  if (!exact(candidate, keys) || !text(author.userId) || !text(author.username) || !text(author.displayName) ||
    (author.profilePictureURL !== undefined && !text(author.profilePictureURL))) throw new Error('invalid practice comment');
  return commentFromAPI(
    value,
    author,
    currentHeart?.heartCount ?? 0,
    currentHeart?.heartedByViewer ?? false,
  );
}

export function practiceCommentPageFromAPI(value: unknown): PracticeCommentPage {
  const envelope = record(value);
  const data = record(envelope.data);
  const meta = record(envelope.meta);
  if (!exactEnvelope(envelope, ['data', 'meta']) || !exact(data, ['items']) || !Array.isArray(data.items) ||
    !Object.keys(meta).every((key) => key === 'nextCursor') ||
    (meta.nextCursor !== undefined && (typeof meta.nextCursor !== 'string' || meta.nextCursor.trim() !== meta.nextCursor))) {
    throw new Error('invalid practice comment page');
  }
  const items = data.items.map(practiceCommentFromAPI);
  const ids = new Set<string>();
  for (let index = 0; index < items.length; index += 1) {
    const item = items[index]!;
    if (ids.has(item.id)) throw new Error('invalid practice comment page');
    ids.add(item.id);
    if (index > 0 && Date.parse(items[index - 1]!.createdAt) > Date.parse(item.createdAt)) {
      throw new Error('comments are not chronological');
    }
  }
  return Object.freeze({ items: Object.freeze(items), nextCursor: meta.nextCursor as string | undefined ?? '' });
}

export function mergePracticeCommentPage(current: PracticeCommentPage, incoming: PracticeCommentPage, requestedCursor: string): PracticeCommentPage {
  if (requestedCursor !== current.nextCursor) throw new Error('stale comment cursor');
  if (!requestedCursor) {
    const currentByID = new Map(current.items.map((comment) => [comment.id, comment]));
    return Object.freeze({
      ...incoming,
      items: Object.freeze(incoming.items.map((comment) => {
        const existing = currentByID.get(comment.id);
        return existing?.heartPending
          ? Object.freeze({
              ...comment,
              heartCount: existing.heartCount,
              heartedByViewer: existing.heartedByViewer,
              heartPending: true,
            })
          : comment;
      })),
    });
  }
  const ids = new Set(current.items.map(({ id }) => id));
  return Object.freeze({
    items: Object.freeze([...current.items, ...incoming.items.filter(({ id }) => !ids.has(id))]),
    nextCursor: incoming.nextCursor,
  });
}

export function appendOptimisticPracticeComment(page: PracticeCommentPage, input: {
  author: PracticeCommentAuthor;
  createdAt: string;
  eventId: string;
  temporaryId: string;
  text: string;
}): PracticeCommentPage {
  const item: PracticeComment = Object.freeze({
    id: input.temporaryId,
    eventId: input.eventId,
    authorUserId: input.author.userId,
    author: Object.freeze({ ...input.author }),
    text: input.text,
    version: 1,
    createdAt: input.createdAt,
    updatedAt: input.createdAt,
    edited: false,
    heartCount: 0,
    heartedByViewer: false,
    heartPending: false,
    pending: true,
  });
  return Object.freeze({ ...page, items: Object.freeze([...page.items, item]) });
}

export function replacePracticeComment(page: PracticeCommentPage, id: string, replacement: PracticeComment): PracticeCommentPage {
  if (replacement.eventId !== page.items.find((item) => item.id === id)?.eventId) throw new Error('mismatched practice comment');
  return Object.freeze({ ...page, items: Object.freeze(page.items.map((item) => item.id === id ? replacement : item)) });
}

export function updatePracticeCommentOptimistically(page: PracticeCommentPage, id: string, expectedVersion: number, value: string): PracticeCommentPage {
  const target = page.items.find((item) => item.id === id);
  if (!target || target.version !== expectedVersion || target.pending) throw new Error('stale comment version');
  return Object.freeze({
    ...page,
    items: Object.freeze(page.items.map((item) => item.id === id
      ? Object.freeze({ ...item, text: value, edited: true, pending: true })
      : item)),
  });
}

export function removePracticeComment(page: PracticeCommentPage, id: string): PracticeCommentPage {
  return Object.freeze({ ...page, items: Object.freeze(page.items.filter((item) => item.id !== id)) });
}

export function rollbackPracticeCommentEdit(page: PracticeCommentPage, previous: PracticeComment, optimisticText: string): PracticeCommentPage {
  const current = page.items.find(({ id }) => id === previous.id);
  if (!current || !current.pending || current.version !== previous.version || current.text !== optimisticText) return page;
  return Object.freeze({ ...page, items: Object.freeze(page.items.map((item) => item.id === previous.id ? previous : item)) });
}

export function restoreDeletedPracticeComment(page: PracticeCommentPage, deleted: PracticeComment): PracticeCommentPage {
  if (page.items.some(({ id }) => id === deleted.id)) return page;
  const items = [...page.items, deleted].sort((left, right) =>
    Date.parse(left.createdAt) - Date.parse(right.createdAt) || left.id.localeCompare(right.id));
  return Object.freeze({ ...page, items: Object.freeze(items) });
}

export function practiceCommentHistoryPageFromAPI(value: unknown, commentId: string): PracticeCommentHistoryPage {
  const envelope = record(value);
  const data = record(envelope.data);
  const meta = record(envelope.meta);
  if (!exactEnvelope(envelope, ['data', 'meta']) || !exact(data, ['versions']) || !Array.isArray(data.versions) ||
    !Object.keys(meta).every((key) => key === 'nextCursor') ||
    (meta.nextCursor !== undefined && (typeof meta.nextCursor !== 'string' || meta.nextCursor.trim() !== meta.nextCursor))) {
    throw new Error('invalid comment history');
  }
  const versions = data.versions.map((value): PracticeCommentVersion => {
    const candidate = record(value);
    if (!exact(candidate, ['commentId', 'createdAt', 'text', 'version']) || candidate.commentId !== commentId ||
      !text(candidate.text) || !instant(candidate.createdAt) || !Number.isSafeInteger(candidate.version) || (candidate.version as number) < 1) {
      throw new Error('invalid comment history');
    }
    return Object.freeze({ commentId, createdAt: candidate.createdAt, text: candidate.text, version: candidate.version as number });
  });
  const seen = new Set<number>();
  for (let index = 0; index < versions.length; index += 1) {
    const version = versions[index]!;
    if (seen.has(version.version) || (index > 0 && versions[index - 1]!.version >= version.version) ||
      (index > 0 && Date.parse(versions[index - 1]!.createdAt) > Date.parse(version.createdAt))) throw new Error('invalid comment history');
    seen.add(version.version);
  }
  return Object.freeze({ commentId, versions: Object.freeze(versions), nextCursor: meta.nextCursor as string | undefined ?? '' });
}

export function mergePracticeCommentHistoryPage(
  current: PracticeCommentHistoryPage,
  incoming: PracticeCommentHistoryPage,
  requestedCursor: string,
): PracticeCommentHistoryPage {
  if (incoming.commentId !== current.commentId || requestedCursor !== current.nextCursor) throw new Error('stale comment history cursor');
  if (!requestedCursor) return incoming;
  const previous = current.versions.at(-1);
  const first = incoming.versions[0];
  if (previous && first && (first.version <= previous.version || Date.parse(first.createdAt) < Date.parse(previous.createdAt))) {
    throw new Error('stale comment history page');
  }
  const seen = new Set(current.versions.map(({ version }) => version));
  return Object.freeze({
    commentId: current.commentId,
    versions: Object.freeze([...current.versions, ...incoming.versions.filter(({ version }) => !seen.has(version))]),
    nextCursor: incoming.nextCursor,
  });
}
