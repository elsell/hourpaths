export const NUDGE_PRESETS = Object.freeze([
  'you_have_got_this',
  'lets_go',
  'little_progress_counts',
  'keep_it_going',
  'time_to_work',
] as const);

export const NUDGE_AUDIENCES = Object.freeze([
  'nobody',
  'path_members',
  'followers',
  'everyone',
] as const);

export const NUDGE_UNAVAILABLE_REASONS = Object.freeze([
  'goal_complete',
  'rate_limited',
] as const);

export type NudgePreset = (typeof NUDGE_PRESETS)[number];
export type NudgeAudience = (typeof NUDGE_AUDIENCES)[number];
export type NudgeUnavailableReason = (typeof NUDGE_UNAVAILABLE_REASONS)[number];

export type NudgeEligibility = Readonly<
  | { eligible: true; pathId: string; recipientUserId: string }
  | { eligible: false; pathId: string; reason: NudgeUnavailableReason; recipientUserId: string }
>;

export type NudgeAudiencePreference = Readonly<{
  audience: NudgeAudience;
  pathId: string;
  revision: number;
  userId: string;
}>;

export type NudgeChannelPreference = Readonly<{
  channel: 'nudges';
  enabled: boolean;
  revision: number;
}>;

export type NudgeContent = Readonly<{
  kind: 'preset';
  preset: NudgePreset;
}>;

export type NudgeReceipt = Readonly<{
  content: NudgeContent;
  id: string;
  pathId: string;
  recipientUserId: string;
  senderUserId: string;
  sentAt: string;
}>;

export type NudgeSendBody = Readonly<{ content: NudgeContent }>;
export type NudgeSendReview = Readonly<{
  body: NudgeSendBody;
  pathId: string;
  recipientUserId: string;
  signature: string;
}>;

export type NudgeAudienceUpdateBody = Readonly<{
  audience: NudgeAudience;
  expectedRevision: number;
}>;

export type NudgeAudienceReview = Readonly<{
  body: NudgeAudienceUpdateBody;
  changed: boolean;
  pathId: string;
  signature: string;
  userId: string;
}>;

export type NudgeChannelUpdateBody = Readonly<{
  enabled: boolean;
  expectedRevision: number;
}>;

export type NudgeChannelReview = Readonly<{
  body: NudgeChannelUpdateBody;
  changed: boolean;
  signature: string;
}>;

export type NudgeSendResult =
  | Readonly<{ kind: 'applied'; receipt: NudgeReceipt }>
  | Readonly<{ kind: 'failed'; cause: unknown }>
  | Readonly<{ kind: 'busy' }>
  | Readonly<{ kind: 'superseded' }>;

export type NudgeAudienceResult =
  | Readonly<{ kind: 'applied'; preference: NudgeAudiencePreference }>
  | Readonly<{ kind: 'failed'; cause: unknown }>
  | Readonly<{ kind: 'busy' }>
  | Readonly<{ kind: 'superseded' }>;

export type NudgeChannelResult =
  | Readonly<{ kind: 'applied'; preference: NudgeChannelPreference }>
  | Readonly<{ kind: 'failed'; cause: unknown }>
  | Readonly<{ kind: 'busy' }>
  | Readonly<{ kind: 'superseded' }>;

function hasExactKeys(value: Record<string, unknown>, expected: readonly string[]): boolean {
  const actual = Object.keys(value).sort();
  const keys = [...expected].sort();
  return actual.length === keys.length && actual.every((key, index) => key === keys[index]);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function validIdentifier(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

function validInstant(value: unknown): value is string {
  return validIdentifier(value) &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u.test(value) &&
    Number.isFinite(Date.parse(value));
}

function validRevision(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0;
}

function validPreset(value: unknown): value is NudgePreset {
  return typeof value === 'string' && NUDGE_PRESETS.includes(value as NudgePreset);
}

function validAudience(value: unknown): value is NudgeAudience {
  return typeof value === 'string' && NUDGE_AUDIENCES.includes(value as NudgeAudience);
}

function validUnavailableReason(value: unknown): value is NudgeUnavailableReason {
  return typeof value === 'string' && NUDGE_UNAVAILABLE_REASONS.includes(value as NudgeUnavailableReason);
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/u.test(value);
}

export function nudgeEligibilityFromAPI(value: unknown): NudgeEligibility {
  if (!isRecord(value) || !validIdentifier(value.pathId) || !validIdentifier(value.recipientUserId)) {
    throw new Error('invalid nudge eligibility');
  }
  if (value.eligible === true && hasExactKeys(value, ['eligible', 'pathId', 'recipientUserId'])) {
    return Object.freeze({ eligible: true, pathId: value.pathId, recipientUserId: value.recipientUserId });
  }
  if (
    value.eligible === false &&
    hasExactKeys(value, ['eligible', 'pathId', 'reason', 'recipientUserId']) &&
    validUnavailableReason(value.reason)
  ) {
    return Object.freeze({
      eligible: false,
      pathId: value.pathId,
      reason: value.reason,
      recipientUserId: value.recipientUserId,
    });
  }
  throw new Error('invalid nudge eligibility');
}

export function nudgeAudiencePreferenceFromAPI(value: unknown): NudgeAudiencePreference {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['audience', 'pathId', 'revision', 'userId']) ||
    !validAudience(value.audience) ||
    !validIdentifier(value.pathId) ||
    !validRevision(value.revision) ||
    !validIdentifier(value.userId) ||
    (value.revision === 0 && value.audience !== 'path_members')
  ) throw new Error('invalid nudge audience preference');
  return Object.freeze({
    audience: value.audience,
    pathId: value.pathId,
    revision: value.revision,
    userId: value.userId,
  });
}

export function nudgeChannelPreferenceFromAPI(value: unknown): NudgeChannelPreference {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['channel', 'enabled', 'revision']) ||
    value.channel !== 'nudges' ||
    typeof value.enabled !== 'boolean' ||
    !validRevision(value.revision) ||
    (value.revision === 0 && value.enabled !== true)
  ) throw new Error('invalid nudge notification channel preference');
  return Object.freeze({ channel: 'nudges', enabled: value.enabled, revision: value.revision });
}

export function nudgeReceiptFromAPI(value: unknown): NudgeReceipt {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['content', 'id', 'pathId', 'recipientUserId', 'senderUserId', 'sentAt']) ||
    !isRecord(value.content) ||
    !hasExactKeys(value.content, ['kind', 'preset']) ||
    value.content.kind !== 'preset' ||
    !validPreset(value.content.preset) ||
    !validIdentifier(value.id) ||
    !validIdentifier(value.pathId) ||
    !validIdentifier(value.recipientUserId) ||
    !validIdentifier(value.senderUserId) ||
    value.recipientUserId === value.senderUserId ||
    !validInstant(value.sentAt)
  ) throw new Error('invalid nudge receipt');
  return Object.freeze({
    content: Object.freeze({ kind: 'preset', preset: value.content.preset }),
    id: value.id,
    pathId: value.pathId,
    recipientUserId: value.recipientUserId,
    senderUserId: value.senderUserId,
    sentAt: value.sentAt,
  });
}

export function reviewNudgeSend(
  pathId: string,
  recipientUserId: string,
  preset: NudgePreset,
): NudgeSendReview {
  if (!validIdentifier(pathId) || !validIdentifier(recipientUserId) || !validPreset(preset)) {
    throw new Error('invalid nudge send review');
  }
  return Object.freeze({
    body: Object.freeze({ content: Object.freeze({ kind: 'preset', preset }) }),
    pathId,
    recipientUserId,
    signature: `${pathId}\0${recipientUserId}\0${preset}`,
  });
}

export function reviewNudgeAudienceChange(
  preference: NudgeAudiencePreference,
  audience: NudgeAudience,
): NudgeAudienceReview {
  const current = nudgeAudiencePreferenceFromAPI(preference);
  if (!validAudience(audience)) throw new Error('invalid nudge audience review');
  return Object.freeze({
    body: Object.freeze({ audience, expectedRevision: current.revision }),
    changed: current.audience !== audience,
    pathId: current.pathId,
    signature: `${current.pathId}\0${current.userId}\0${current.revision}\0${audience}`,
    userId: current.userId,
  });
}

export function reviewNudgeChannelChange(
  preference: NudgeChannelPreference,
  enabled: boolean,
): NudgeChannelReview {
  const current = nudgeChannelPreferenceFromAPI(preference);
  if (typeof enabled !== 'boolean') throw new Error('invalid nudge notification channel review');
  return Object.freeze({
    body: Object.freeze({ enabled, expectedRevision: current.revision }),
    changed: current.enabled !== enabled,
    signature: `nudges\0${current.revision}\0${enabled}`,
  });
}

type Retry<Body> = Readonly<{ body: Body; idempotencyKey: string; signature: string }>;

export function createNudgeSendOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let activeEpoch: number | null = null;
  let retry: Retry<NudgeSendBody> | undefined;

  return {
    async submit(
      review: NudgeSendReview,
      request: (pathId: string, recipientUserId: string, body: NudgeSendBody, idempotencyKey: string) => Promise<unknown>,
    ): Promise<NudgeSendResult> {
      if (activeEpoch !== null) return { kind: 'busy' };
      const operationEpoch = ++epoch;
      activeEpoch = operationEpoch;
      const attempt = retry?.signature === review.signature
        ? retry
        : Object.freeze({ body: review.body, idempotencyKey: keyFactory(), signature: review.signature });
      if (!validIdempotencyKey(attempt.idempotencyKey)) {
        activeEpoch = null;
        throw new Error('invalid nudge idempotency key');
      }
      retry = attempt;
      try {
        const receipt = nudgeReceiptFromAPI(await request(
          review.pathId,
          review.recipientUserId,
          attempt.body,
          attempt.idempotencyKey,
        ));
        if (epoch !== operationEpoch) return { kind: 'superseded' };
        if (
          receipt.pathId !== review.pathId ||
          receipt.recipientUserId !== review.recipientUserId ||
          receipt.content.preset !== review.body.content.preset
        ) throw new Error('invalid nudge send result');
        retry = undefined;
        return { kind: 'applied', receipt };
      } catch (cause) {
        return epoch === operationEpoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally {
        if (activeEpoch === operationEpoch) activeEpoch = null;
      }
    },
    cancel(): void {
      epoch += 1;
      activeEpoch = null;
      retry = undefined;
    },
  };
}

export function createNudgeAudienceOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let activeEpoch: number | null = null;
  let retry: Retry<NudgeAudienceUpdateBody> | undefined;

  return {
    async submit(
      pathId: string,
      review: NudgeAudienceReview,
      request: (pathId: string, body: NudgeAudienceUpdateBody, idempotencyKey: string) => Promise<unknown>,
    ): Promise<NudgeAudienceResult> {
      if (pathId !== review.pathId || !validIdentifier(pathId)) throw new Error('invalid nudge audience target');
      if (!review.changed) throw new Error('unchanged nudge audience cannot be submitted');
      if (activeEpoch !== null) return { kind: 'busy' };
      const operationEpoch = ++epoch;
      activeEpoch = operationEpoch;
      const attempt = retry?.signature === review.signature
        ? retry
        : Object.freeze({ body: review.body, idempotencyKey: keyFactory(), signature: review.signature });
      if (!validIdempotencyKey(attempt.idempotencyKey)) {
        activeEpoch = null;
        throw new Error('invalid nudge audience idempotency key');
      }
      retry = attempt;
      try {
        const preference = nudgeAudiencePreferenceFromAPI(await request(pathId, attempt.body, attempt.idempotencyKey));
        if (epoch !== operationEpoch) return { kind: 'superseded' };
        if (
          preference.pathId !== review.pathId ||
          preference.userId !== review.userId ||
          preference.audience !== review.body.audience ||
          preference.revision <= review.body.expectedRevision
        ) throw new Error('invalid nudge audience result');
        retry = undefined;
        return { kind: 'applied', preference };
      } catch (cause) {
        return epoch === operationEpoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally {
        if (activeEpoch === operationEpoch) activeEpoch = null;
      }
    },
    cancel(): void {
      epoch += 1;
      activeEpoch = null;
      retry = undefined;
    },
  };
}

export function createNudgeChannelOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let activeEpoch: number | null = null;
  let retry: Retry<NudgeChannelUpdateBody> | undefined;

  return {
    async submit(
      review: NudgeChannelReview,
      request: (body: NudgeChannelUpdateBody, idempotencyKey: string) => Promise<unknown>,
    ): Promise<NudgeChannelResult> {
      if (!review.changed) throw new Error('unchanged nudge notification channel cannot be submitted');
      if (activeEpoch !== null) return { kind: 'busy' };
      const operationEpoch = ++epoch;
      activeEpoch = operationEpoch;
      const attempt = retry?.signature === review.signature
        ? retry
        : Object.freeze({ body: review.body, idempotencyKey: keyFactory(), signature: review.signature });
      if (!validIdempotencyKey(attempt.idempotencyKey)) {
        activeEpoch = null;
        throw new Error('invalid nudge notification channel idempotency key');
      }
      retry = attempt;
      try {
        const preference = nudgeChannelPreferenceFromAPI(await request(attempt.body, attempt.idempotencyKey));
        if (epoch !== operationEpoch) return { kind: 'superseded' };
        if (
          preference.enabled !== review.body.enabled ||
          preference.revision !== review.body.expectedRevision + 1
        ) throw new Error('invalid nudge notification channel result');
        retry = undefined;
        return { kind: 'applied', preference };
      } catch (cause) {
        return epoch === operationEpoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally {
        if (activeEpoch === operationEpoch) activeEpoch = null;
      }
    },
    cancel(): void {
      epoch += 1;
      activeEpoch = null;
      retry = undefined;
    },
  };
}
