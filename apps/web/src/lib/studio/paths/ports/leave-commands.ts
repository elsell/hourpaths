import type { LeaveReview, LeaveResult } from '../domain/leave';
export interface LeaveCommands {
  review(pathId: string, signal?: AbortSignal): Promise<LeaveReview>;
  submit(review: LeaveReview, retainActivity: boolean, signal?: AbortSignal): Promise<LeaveResult>;
  dispose(): void;
}
