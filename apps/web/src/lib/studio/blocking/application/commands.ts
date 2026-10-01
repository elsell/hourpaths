import type { BlockReview } from '../domain/block';
import type { BlockingRepository } from '../ports/blocking-repository';
export function blockingCommands(repository: BlockingRepository, newKey: () => string) {
  let disposed = false, pending = false;
  let attempt: { review: BlockReview; key: string } | null = null;
  return {
    clear() { if (!pending) attempt = null; },
    dispose() { disposed = true; attempt = null; },
    async submit(review: BlockReview, signal: AbortSignal): Promise<boolean> {
      if (disposed || pending) return false;
      if (!attempt || attempt.review.acknowledgement.token !== review.acknowledgement.token) attempt = { review, key: newKey() };
      const admitted = attempt; pending = true;
      try { await repository.block(admitted.review, admitted.key, signal); return !disposed; }
      finally { pending = false; }
    },
  };
}
