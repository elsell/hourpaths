import type { Path, PathAppearance } from '../../paths/domain/path';

export interface CachedHome {
  owner: string;
  paths: readonly Path[];
  appearances: Record<string, PathAppearance>;
}

/** Cached client-domain presentation stays separate from tracking commands and
 * generated API DTOs. All access remains bound to an authenticated account. */
export interface HomeCache {
  saveHome(owner: string, paths: readonly Path[], appearances: Record<string, PathAppearance>): Promise<void>;
  readHome(owner: string): Promise<CachedHome | null>;
  savePaths(owner: string, paths: readonly Path[]): Promise<void>;
  saveAppearance(owner: string, pathId: string, appearance: PathAppearance): Promise<void>;
}
