import type { BlockIdentity, BlockReview } from '../domain/block';
export interface BlockingRepository {
  review(target: BlockIdentity, signal: AbortSignal): Promise<BlockReview>;
  block(review: BlockReview, operationId: string, signal: AbortSignal): Promise<void>;
}
