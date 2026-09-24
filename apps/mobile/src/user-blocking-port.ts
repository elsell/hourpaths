import {
  blockedAccountPageFromAPI,
  blockReviewFromAPI,
  blockResultFromAPI,
  unblockResultFromAPI,
  type BlockReviewAcknowledgement,
  type UserBlockingPort,
} from '@hourpaths/client-core';

export type GeneratedUserBlockingResult = Readonly<{
  data?: unknown;
  error?: unknown;
  response: Response;
}>;

export interface GeneratedUserBlockingClient {
  reviewProfileBlock(username: string): Promise<GeneratedUserBlockingResult>;
  blockProfile(username: string, idempotencyKey: string, acknowledgement: BlockReviewAcknowledgement): Promise<GeneratedUserBlockingResult>;
  blockedAccounts(cursor?: string): Promise<GeneratedUserBlockingResult>;
  unblockAccount(userId: string, idempotencyKey: string): Promise<GeneratedUserBlockingResult>;
}

function successfulData(result: GeneratedUserBlockingResult): unknown {
  if (!result.response.ok) {
    const problem = result.error as { code?: unknown } | undefined;
    throw {
      kind: 'http',
      status: result.response.status,
      ...(typeof problem?.code === 'string' ? { code: problem.code } : {}),
    } as const;
  }
  if (result.data === undefined) throw { kind: 'http', status: 502 } as const;
  return result.data;
}

export function createUserBlockingGeneratedPort(
  client: () => GeneratedUserBlockingClient,
): UserBlockingPort {
  return {
    async reviewBlock(username) {
      return blockReviewFromAPI(successfulData(await client().reviewProfileBlock(username)));
    },
    async blockUser(username, idempotencyKey, acknowledgement) {
      return blockResultFromAPI(successfulData(await client().blockProfile(username, idempotencyKey, acknowledgement)));
    },
    async listBlockedAccounts(cursor) {
      return blockedAccountPageFromAPI(successfulData(await client().blockedAccounts(cursor)));
    },
    async unblockUser(userId, idempotencyKey) {
      return unblockResultFromAPI(successfulData(await client().unblockAccount(userId, idempotencyKey)));
    },
  } satisfies UserBlockingPort;
}
