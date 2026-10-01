import type { Path } from './path';
export type Visibility = Path['visibility'];
export interface VisibilityContext { readonly path: Path; readonly options: readonly Visibility[] }
export interface VisibilityReview {
  readonly pathId: string;
  readonly name: string;
  readonly current: Visibility;
  readonly proposed: Visibility;
  readonly broader: boolean;
}
export type VisibilityResult =
  | { readonly kind: 'applied'; readonly value: VisibilityContext }
  | { readonly kind: 'failed'; readonly requiresReview: boolean }
  | { readonly kind: 'superseded' };
