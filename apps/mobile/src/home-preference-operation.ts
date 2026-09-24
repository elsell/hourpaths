export type HomePreferenceSnapshot = {
  manualPathIds: string[];
  orderMethod: 'recent' | 'alphabetical' | 'manual';
  pinnedPathIds: string[];
  revision: number;
  updatedAt?: string;
};

export type HomePreferenceUpdate = {
  expectedRevision: number;
  manualPathIds: string[];
  orderMethod: HomePreferenceSnapshot['orderMethod'];
  pinnedPathIds: string[];
};

export type HomePreferenceMutationResult =
  | { kind: 'applied'; preferences: HomePreferenceSnapshot }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' };

function exactKeys(value: Record<string, unknown>, expected: readonly string[]): boolean {
  const keys = Object.keys(value).sort();
  const wanted = [...expected].sort();
  return keys.length === wanted.length && keys.every((key, index) => key === wanted[index]);
}

function uniqueIDs(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((id) => typeof id === 'string' && id.length > 0) && new Set(value).size === value.length;
}

function validInstant(value: unknown): value is string {
  return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value)
    && Number.isFinite(Date.parse(value));
}

export function homePreferencesFromAPI(value: unknown): HomePreferenceSnapshot {
  if (!value || typeof value !== 'object') throw new Error('invalid Home preferences');
  const candidate = value as Record<string, unknown>;
  const expectedKeys = candidate.updatedAt === undefined
    ? ['manualPathIds', 'orderMethod', 'pinnedPathIds', 'revision']
    : ['manualPathIds', 'orderMethod', 'pinnedPathIds', 'revision', 'updatedAt'];
  if (!exactKeys(candidate, expectedKeys)) throw new Error('invalid Home preferences');
  if (!['recent', 'alphabetical', 'manual'].includes(String(candidate.orderMethod))
    || !Number.isSafeInteger(candidate.revision) || (candidate.revision as number) < 0
    || !uniqueIDs(candidate.pinnedPathIds) || !uniqueIDs(candidate.manualPathIds)
    || !(candidate.pinnedPathIds as string[]).every((id) => (candidate.manualPathIds as string[]).includes(id))
    || (candidate.updatedAt === undefined ? candidate.revision !== 0 : !validInstant(candidate.updatedAt))) throw new Error('invalid Home preferences');
  return {
    manualPathIds: [...candidate.manualPathIds as string[]],
    orderMethod: candidate.orderMethod as HomePreferenceSnapshot['orderMethod'],
    pinnedPathIds: [...candidate.pinnedPathIds as string[]],
    revision: candidate.revision as number,
    ...(typeof candidate.updatedAt === 'string' ? { updatedAt: candidate.updatedAt } : {}),
  };
}

function updateFor(current: HomePreferenceSnapshot, requested: HomePreferenceSnapshot): HomePreferenceUpdate {
  return {
    expectedRevision: current.revision,
    manualPathIds: [...requested.manualPathIds],
    orderMethod: requested.orderMethod,
    pinnedPathIds: [...requested.pinnedPathIds],
  };
}

export function createHomePreferenceOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let retry: { signature: string; key: string } | null = null;
  return {
    async submit(
      current: HomePreferenceSnapshot,
      requested: HomePreferenceSnapshot,
      request: (body: HomePreferenceUpdate, idempotencyKey: string) => Promise<unknown>,
    ): Promise<HomePreferenceMutationResult> {
      const operation = ++epoch;
      const body = updateFor(current, requested);
      const signature = JSON.stringify(body);
      const key = retry?.signature === signature ? retry.key : keyFactory();
      if (typeof key !== 'string' || key.length < 16 || key.length > 128 || !/^[\x20-\x7e]+$/.test(key)) {
        throw new Error('invalid idempotency key');
      }
      retry = { signature, key };
      try {
        const preferences = homePreferencesFromAPI(await request(body, key));
        if (epoch !== operation) return { kind: 'superseded' };
        retry = null;
        return { kind: 'applied', preferences };
      } catch (cause) {
        return epoch === operation ? { kind: 'failed', cause } : { kind: 'superseded' };
      }
    },
    cancel() {
      epoch += 1;
      retry = null;
    },
  };
}
