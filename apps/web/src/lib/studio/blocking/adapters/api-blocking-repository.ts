import { createSessionApiClient } from '@hourpaths/api-client';
import { blockReviewFromAPI, blockResultFromAPI } from '@hourpaths/client-core';
import type { BlockingRepository } from '../ports/blocking-repository';
class BlockingError extends Error {
  constructor(readonly status: number) { super('blocking_request_failed'); }
}
function required(result: { response: Response; data?: unknown }): unknown {
  if (!result.response.ok || !result.data) throw new BlockingError(result.response.status);
  return result.data;
}
export function apiBlockingRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): BlockingRepository {
  const client = (signal: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  return {
    async review(target, signal) {
      const value = blockReviewFromAPI(required(await client(signal).reviewProfileBlock(target.username)));
      if (value.target.userId !== target.userId || value.target.username !== target.username) throw new BlockingError(502);
      return { target: { userId: value.target.userId, username: value.target.username, displayName: value.target.displayName },
        sharedPaths: value.sharedPaths.map(path => ({ ...path })), acknowledgement: { ...value.acknowledgement } };
    },
    async block(review, key, signal) {
      const value = blockResultFromAPI(required(await client(signal).blockProfile(review.target.username, key, review.acknowledgement)));
      if (value.target.userId !== review.target.userId || value.target.username !== review.target.username) throw new BlockingError(502);
    },
  };
}
