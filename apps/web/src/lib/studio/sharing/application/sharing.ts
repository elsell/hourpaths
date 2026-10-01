import type { SharingRepository } from '../ports/sharing-repository';
export function sharingCommands(repository: SharingRepository, key: () => string) { return repository.commands(key); }
export function retainSharingDraft(cause: unknown): boolean {
  if (!cause || typeof cause !== 'object') return false;
  const failure = cause as { kind?: string; status?: number };
  return failure.kind === 'network' || (failure.kind === 'http' && (failure.status === 429 || (typeof failure.status === 'number' && failure.status >= 500)));
}
