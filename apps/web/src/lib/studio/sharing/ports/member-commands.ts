import type { MemberAction, MemberReview } from '../domain/members';
export interface MemberCommands {
  review(pathId: string, userId: string, action: MemberAction): Promise<MemberReview>;
  submit(review: MemberReview): Promise<{ kind: 'applied' | 'failed' | 'superseded' | 'cancelled' }>;
  clear(): void;
  dispose(): void;
}
