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
