import type { Path } from './path';
export function orderPaths(paths: readonly Path[], order: 'recent' | 'alphabetical' | 'manual'): Path[] {
  const rank = (value: number | null) => value ?? Number.MAX_SAFE_INTEGER;
  return [...paths].sort((left, right) => {
    if (left.pinned !== right.pinned) return left.pinned ? -1 : 1;
    if (left.pinned) {
      const difference = rank(left.pinnedPosition) - rank(right.pinnedPosition);
      if (difference) return difference;
    }
    if (order === 'manual') {
      const difference = rank(left.position) - rank(right.position);
      if (difference) return difference;
    }
    if (order === 'alphabetical') return left.name.localeCompare(right.name) || left.id.localeCompare(right.id);
    return right.recentActivityAt - left.recentActivityAt || left.id.localeCompare(right.id);
  });
}
export function movePath(paths: readonly Path[], id: string, direction: -1 | 1): Path[] {
  const result = [...paths];
  const index = result.findIndex(path => path.id === id);
  const destination = index + direction;
  if (index < 0 || destination < 0 || destination >= result.length || result[index]!.pinned !== result[destination]!.pinned) return result;
  [result[index], result[destination]] = [result[destination]!, result[index]!];
  return result;
}
