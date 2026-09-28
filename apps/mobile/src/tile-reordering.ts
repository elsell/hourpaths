export type TileFrame = { x: number; y: number; width: number; height: number };

export function nearestTileSlot(frames: readonly TileFrame[], x: number, y: number): number {
  let best = -1;
  let distance = Infinity;
  frames.forEach((frame, index) => {
    const next = (x - frame.x - frame.width / 2) ** 2 + (y - frame.y - frame.height / 2) ** 2;
    if (next < distance) { best = index; distance = next; }
  });
  return best;
}

export function moveTile(ids: readonly string[], id: string, to: number): string[] {
  const from = ids.indexOf(id);
  if (from < 0 || to < 0 || to >= ids.length || from === to) return [...ids];
  const next = ids.filter(value => value !== id);
  next.splice(to, 0, id);
  return next;
}

/** Replace only visible slots; hidden and archived Paths retain their positions. */
export function mergeVisibleTileOrder(all: readonly string[], visible: readonly string[]): string[] {
  const selected = new Set(visible);
  if (selected.size !== visible.length || visible.some(id => !all.includes(id))) return [...all];
  let index = 0;
  return all.map(id => selected.has(id) ? visible[index++]! : id);
}
