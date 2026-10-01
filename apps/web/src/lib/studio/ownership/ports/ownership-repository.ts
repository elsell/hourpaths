import type { Candidate, Transfer, TransferCommand, TransferReview } from '../domain/transfer';
export interface OwnershipRepository {
  pending(pathId: string, signal?: AbortSignal): Promise<Transfer | null>;
  candidates(pathId: string, cursor: string, signal?: AbortSignal): Promise<{ items: readonly Candidate[]; nextCursor: string }>;
  review(pathId: string, recipientId: string): Promise<TransferReview>;
  execute(command: TransferCommand, operationId: string): Promise<Transfer>;
}
