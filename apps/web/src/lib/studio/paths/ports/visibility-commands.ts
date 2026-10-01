import type { Visibility, VisibilityContext, VisibilityReview, VisibilityResult } from '../domain/visibility';
export interface VisibilityCommands {
  load(pathId: string, signal?: AbortSignal): Promise<VisibilityContext>;
  review(context: VisibilityContext, proposed: Visibility): VisibilityReview | null;
  submit(review: VisibilityReview, signal?: AbortSignal): Promise<VisibilityResult>;
  dispose(): void;
}
