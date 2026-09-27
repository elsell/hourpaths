// Shared by Path tiles, active shortcuts, and the appearance picker.
export const pathPalette = {
  coral: { background: '#FFD8D6', foreground: '#351E25', accent: '#A82139', track: '#EDB9BE' },
  lavender: { background: '#DFDEFF', foreground: '#252044', accent: '#5142A0', track: '#C5C0ED' },
  gold: { background: '#FFF0B8', foreground: '#372C10', accent: '#755400', track: '#E4CF8E' },
  mint: { background: '#CCF4DE', foreground: '#173C2C', accent: '#176B48', track: '#A9DCC0' },
  blue: { background: '#D5E9FF', foreground: '#19324E', accent: '#235F9E', track: '#B2D1F0' },
  pink: { background: '#F0DBFF', foreground: '#392044', accent: '#783797', track: '#DAB8EB' },
} as const;
export type PathColor = keyof typeof pathPalette;
export type PathAppearance = { color: PathColor; emoji: string };
export type PathTone = { background: string; foreground: string; accent: string; track: string };
export const pathColorKeys = Object.keys(pathPalette) as PathColor[];
export function defaultPathAppearance(pathID: string): PathAppearance {
  const hash = Array.from(pathID).reduce((value, character) => (value * 31 + character.codePointAt(0)!) >>> 0, 0);
  return { color: pathColorKeys[hash % pathColorKeys.length]!, emoji: '✨' };
}

// One pictographic emoji (including skin-tone/ZWJ/tag sequences), flag, or
// keycap. Accept native keyboard emoji without accepting arbitrary Path copy.
const emoji = /^(?:\p{Regional_Indicator}{2}|[#*0-9]\uFE0F?\u20E3|\p{Extended_Pictographic}\uFE0F?\p{Emoji_Modifier}?(?:[\u{E0020}-\u{E007E}]+\u{E007F})?(?:\u200D\p{Extended_Pictographic}\uFE0F?\p{Emoji_Modifier}?)*)$/u;
export function validPathEmoji(value: string): boolean {
  return Array.from(value).length <= 32 && emoji.test(value);
}

export type SavedPathAppearance = { revision: number; color?: string; emoji?: string };
export type AppearancePort = {
  read(id: string): Promise<SavedPathAppearance>;
  save(id: string, value: PathAppearance, revision: number, key: string): Promise<SavedPathAppearance>;
};
export function parsePathAppearance(value: SavedPathAppearance): SavedPathAppearance {
  if (!Number.isSafeInteger(value.revision) || value.revision < 0 ||
    (value.revision > 0 && (!Object.hasOwn(pathPalette, value.color ?? '') || !validPathEmoji(value.emoji ?? '')))) throw new Error('invalid_appearance');
  return value.revision === 0 ? { revision: 0 } : { ...value };
}
// One instance per signed-in account. Disposal invalidates in-flight results.
export function createPathAppearanceStore(port: AppearancePort, key: () => string, changed: () => void) {
  let disposed = false;
  const values = new Map<string, SavedPathAppearance>();
  const tickets = new Map<string, number>();
  const failures = new Set<string>();
  const pending = new Map<string, { signature: string; key: string; revision: number }>();
  const saving = new Set<string>();
  function ticket(id: string) { const next = (tickets.get(id) ?? 0) + 1; tickets.set(id, next); return next; }
  function publish() { if (!disposed) changed(); }
  async function load(id: string) {
    if (disposed || saving.has(id)) return;
    const request = ticket(id);
    try {
      const value = parsePathAppearance(await port.read(id));
      if (disposed || tickets.get(id) !== request) return;
      values.set(id, value); failures.delete(id); publish();
    } catch {
      if (disposed || tickets.get(id) !== request) return;
      failures.add(id); publish();
    }
  }
  return {
    appearance(id: string): PathAppearance {
      const value = values.get(id);
      return value?.revision ? { color: value.color as PathColor, emoji: value.emoji! } : defaultPathAppearance(id);
    },
    hasFailures: () => failures.size > 0,
    revision: (id: string) => values.get(id)?.revision ?? 0,
    isLoaded: (id: string) => values.has(id),
    async refresh(ids: readonly string[]) {
      // Bounded fan-out avoids flooding the principal limiter on large grids.
      for (let offset = 0; offset < ids.length && !disposed; offset += 4) {
        await Promise.all(ids.slice(offset, offset + 4).map(load));
      }
    },
    async save(id: string, appearance: PathAppearance, expectedRevision?: number) {
      if (disposed || saving.has(id)) return false;
      if (!values.has(id)) await load(id);
      if (disposed || !values.has(id)) throw new Error('appearance_unavailable');
      saving.add(id); ticket(id);
      const revision = expectedRevision ?? values.get(id)!.revision;
      const signature = JSON.stringify(appearance);
      const attempt = pending.get(id)?.signature === signature ? pending.get(id)! : { signature, key: key(), revision };
      pending.set(id, attempt);
      try {
        const result = parsePathAppearance(await port.save(id, appearance, attempt.revision, attempt.key));
        if (disposed) return false;
        values.set(id, result); failures.delete(id); pending.delete(id); publish();
        return true;
      } catch (error) {
        // A conflict refreshes the base revision, but preserves the draft until
        // the person explicitly retries. Unknown outcomes keep the same key.
        if ((error as { status?: number })?.status === 409) {
          pending.delete(id); saving.delete(id); await load(id);
        }
        throw error;
      } finally { saving.delete(id); }
    },
    dispose() { disposed = true; values.clear(); failures.clear(); pending.clear(); },
  };
}
