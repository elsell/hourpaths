import type { OwnershipTransfer, OwnershipTransferCandidate } from '@hourpaths/api-client';
import type { Translator } from '@hourpaths/i18n';

const minuteMilliseconds = 60_000;
const minutesPerHour = 60;
const minutesPerDay = 24 * minutesPerHour;
const minutesPerYear = 365 * minutesPerDay;

export type OwnershipTransferExpiration = Readonly<{
  exact: string;
  relative: string;
  summary: string;
}>;

export type OwnershipTransferReview = Readonly<{
  reservationToken: string;
  expiresAt: string;
  recipient: Readonly<{ displayName: string; userId: string; username: string }>;
  reviewedAt: string;
  viewerTimeZone: string;
}>;

function exactKeys(value: object, expected: readonly string[]): boolean {
  const keys = Object.keys(value).sort();
  return keys.length === expected.length && keys.every((key, index) => key === expected[index]);
}

function validReviewText(value: unknown): value is string {
  return typeof value === 'string' && value !== '' && value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

export function decodeOwnershipTransferReview(
  value: unknown,
  expectedPathID: string,
  expectedRecipientID: string,
): OwnershipTransferReview | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const review = value as Partial<OwnershipTransferReview>;
  const recipient = review.recipient;
  if (
    !exactKeys(value, ['expiresAt', 'recipient', 'reservationToken', 'reviewedAt', 'viewerTimeZone']) ||
    expectedPathID === '' || !recipient || !exactKeys(recipient, ['displayName', 'userId', 'username']) ||
    recipient.userId !== expectedRecipientID || !validReviewText(recipient.userId) ||
    !validReviewText(recipient.displayName) || !validReviewText(recipient.username) ||
    !validReviewText(review.reviewedAt) || !Number.isFinite(Date.parse(review.reviewedAt)) ||
    !validReviewText(review.expiresAt) || !Number.isFinite(Date.parse(review.expiresAt)) ||
    Date.parse(review.expiresAt) <= Date.parse(review.reviewedAt) ||
    !validReviewText(review.reservationToken) || !validViewerTimeZone(review.viewerTimeZone)
  ) return undefined;
  return Object.freeze(review as OwnershipTransferReview);
}

export function validViewerTimeZone(value: unknown): value is string {
  if (!validReviewText(value)) return false;
  try {
    new Intl.DateTimeFormat('en', { timeZone: value }).format(0);
    return true;
  } catch (cause) {
    if (cause instanceof RangeError) return false;
    throw cause;
  }
}

export function mergeOwnershipTransferCandidates(
  current: readonly OwnershipTransferCandidate[],
  incoming: readonly OwnershipTransferCandidate[],
  replace: boolean,
): OwnershipTransferCandidate[] {
  const candidates = replace ? [] : [...current];
  const seen = new Set(candidates.map(({ userId }) => userId));
  for (const candidate of incoming) {
    if (!seen.has(candidate.userId)) candidates.push(candidate);
    seen.add(candidate.userId);
  }
  return candidates;
}

export function ownershipTransferExpiration(
  createdAt: string,
  expiresAt: string,
  timeZone: string,
  translator: Translator,
): OwnershipTransferExpiration | undefined {
  const created = Date.parse(createdAt);
  const expires = Date.parse(expiresAt);
  if (!Number.isFinite(created) || !Number.isFinite(expires) || expires <= created || !validViewerTimeZone(timeZone)) return undefined;
  const duration = expires - created;
  if (duration % minuteMilliseconds !== 0) return undefined;

  let minutes = duration / minuteMilliseconds;
  const units = [
    ['duration.years', Math.floor(minutes / minutesPerYear)],
    ['duration.days', Math.floor((minutes %= minutesPerYear) / minutesPerDay)],
    ['duration.hours', Math.floor((minutes %= minutesPerDay) / minutesPerHour)],
    ['duration.minutes', minutes % minutesPerHour],
  ] as const;
  const relative = units
    .filter(([, value]) => value > 0)
    .map(([key, value]) => translator.t(key, { count: value }))
    .join(' ');
  if (!relative) return undefined;

  try {
    const exact = translator.t('pathOwnership.exactExpiration', {
      date: translator.date(expires, { dateStyle: 'medium', timeZone }),
      time: translator.time(expires, { timeStyle: 'short', timeZone }),
    });
    return { exact, relative, summary: translator.t('pathOwnership.expiration', { exact, relative }) };
  } catch (cause) {
    if (cause instanceof RangeError) return undefined;
    throw cause;
  }
}

export function currentPendingOwnershipTransfers<T extends OwnershipTransfer>(
  transfers: readonly T[],
  now = Date.now(),
): T[] {
  const seenPaths = new Set<string>();
  return transfers.filter((transfer) => {
    const expires = Date.parse(transfer.expiresAt);
    if (transfer.state !== 'pending' || !Number.isFinite(expires) || expires <= now || seenPaths.has(transfer.pathId)) return false;
    seenPaths.add(transfer.pathId);
    return true;
  });
}
