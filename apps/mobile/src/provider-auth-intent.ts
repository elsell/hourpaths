const providerCallback = 'hourpaths://callback';

export function redirectProviderSystemPath(path: string): string | null {
  if (!path.startsWith(providerCallback)) return path;

  const suffix = path.slice(providerCallback.length);
  if (suffix === '' || suffix.startsWith('?') || suffix.startsWith('#')) return null;

  return path;
}
