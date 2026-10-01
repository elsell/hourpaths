import { authenticatedProfileFromAPI, createPathVisibilityOperationOwner, pathVisibilityOptions, reviewPathVisibilityChange } from '@hourpaths/client-core';
import type { createSessionApiClient } from '@hourpaths/api-client';
import type { VisibilityCommands } from '../ports/visibility-commands';
import type { VisibilityContext, VisibilityReview } from '../domain/visibility';
import { pathFromAPI, PathRequestError } from './path-mapping';
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new PathRequestError(result.response.status);
  return result.data.data;
}
export function sharedVisibilityCommands(client: (signal?: AbortSignal) => ReturnType<typeof createSessionApiClient>, key: () => string): VisibilityCommands {
  const owner = createPathVisibilityOperationOwner(key);
  let disposed = false, epoch = 0, admitted = false;
  let context: VisibilityContext | null = null, selected: VisibilityReview | null = null;
  let privacy: 'private' | 'public' = 'private';
  return {
    dispose() { disposed = true; epoch++; context = null; selected = null; owner.cancel(); },
    async load(pathId, signal) {
      if (disposed || admitted) throw new Error('visibility_review_unavailable');
      const ticket = ++epoch; context = null; selected = null; owner.cancel();
      const [pathResult, profileResult] = await Promise.all([client(signal).path(pathId), client(signal).profile()]);
      const path = pathFromAPI(required(pathResult));
      const profile = authenticatedProfileFromAPI(required(profileResult));
      if (disposed || ticket !== epoch || path.id !== pathId || path.archived || !path.canManageVisibility) throw new Error('visibility_review_unavailable');
      privacy = profile.profileVisibility;
      context = Object.freeze({ path: Object.freeze(path), options: pathVisibilityOptions(privacy) });
      return context;
    },
    review(value, proposed) {
      if (disposed || admitted || value !== context || !value.path.canManageVisibility || value.path.archived) throw new Error('visibility_review_unavailable');
      const review = reviewPathVisibilityChange(value.path, proposed, privacy);
      if (review.kind === 'not-permitted') throw new Error('visibility_choice_unavailable');
      if (review.kind === 'unchanged') { selected = null; return null; }
      selected = Object.freeze({ pathId: review.pathId, name: review.pathName, current: review.current, proposed: review.proposed, broader: review.broader });
      return selected;
    },
    async submit(review, signal) {
      if (disposed || admitted || !context || review !== selected) return { kind: 'superseded' };
      admitted = true;
      try {
        const result = await owner.submit({ kind: 'ready', pathId: review.pathId, pathName: review.name, current: review.current, proposed: review.proposed, broader: review.broader }, true,
          async (pathId, body, id) => { const dto = required(await client(signal).setPathVisibility(pathId, body, id)); pathFromAPI(dto); return dto; });
        if (disposed) return { kind: 'superseded' };
        if (result.kind === 'applied') {
          const path = pathFromAPI(result.path);
          context = Object.freeze({ ...context, path: Object.freeze(path) }); selected = null;
          return { kind: 'applied', value: context };
        }
        if (result.kind === 'failed') {
          const requiresReview = result.cause instanceof PathRequestError && [400, 401, 403, 404, 409].includes(result.cause.status);
          if (requiresReview) { selected = null; owner.cancel(review.pathId); }
          return { kind: 'failed', requiresReview };
        }
        return { kind: 'superseded' };
      } finally { admitted = false; }
    },
  };
}
