const providerCallbacks = ['hourpaths://callback', 'hourpaths://logout'];

export function redirectProviderSystemPath(path: string): string | null {
  const providerCallback = providerCallbacks.find((callback) => path.startsWith(callback));
  if (!providerCallback) return path;

  const suffix = path.slice(providerCallback.length);
  if (suffix === '' || suffix.startsWith('?') || suffix.startsWith('#')) return null;

  return path;
}
