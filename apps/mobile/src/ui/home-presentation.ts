import type { HomeFilter } from './home-organization';
import type { HomePathMode } from './home-header-actions';

export type HomePresentation =
  | { kind: 'loading' }
  | { kind: 'offline' }
  | { kind: 'error' }
  | { kind: 'empty'; mode: HomePathMode }
  | { kind: 'filtered-empty' }
  | { kind: 'ready' };

export type HomePresentationInput = Readonly<{
  failure: 'offline' | 'error' | null;
  filter: HomeFilter;
  homeAvailable: boolean;
  loading: boolean;
  mode: HomePathMode;
  totalCount: number;
  visibleCount: number;
}>;

export function homePresentation(input: HomePresentationInput): HomePresentation {
  if (!input.homeAvailable) {
    if (input.loading) return { kind: 'loading' };
    if (input.failure) return { kind: input.failure };
    return { kind: 'loading' };
  }
  if (input.totalCount === 0) return { kind: 'empty', mode: input.mode };
  if (input.mode === 'active' && input.filter !== 'all' && input.visibleCount === 0) {
    return { kind: 'filtered-empty' };
  }
  return { kind: 'ready' };
}
