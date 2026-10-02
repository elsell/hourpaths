/** Public asset storage only. Personal data remains in the account-scoped ledger. */
interface ShellBucket {
  match(url: string): Promise<Response | undefined>;
  put(url: string, response: Response): Promise<void>;
}
interface ShellStorage {
  open(name: string): Promise<ShellBucket>;
  keys(): Promise<string[]>;
  delete(name: string): Promise<boolean>;
}
interface ShellRequest { url: string; method: string; mode: string }
interface ShellDependencies {
  origin: string;
  base: string;
  version: string;
  assets: readonly string[];
  storage: ShellStorage;
  request(url: string, init?: RequestInit): Promise<Response>;
}
export function createShellCache(deps: ShellDependencies) {
  const prefix = `hourpaths-public-shell:${deps.base}:`;
  const name = prefix + deps.version;
  const shell = new URL(`${deps.base}/studio`, deps.origin).href;
  const assets = new Set(deps.assets.map(asset => new URL(asset, deps.origin).href));
  for (const asset of assets) {
    if (new URL(asset).origin !== deps.origin) throw new Error('Shell assets must be same-origin');
  }
  function navigation(request: ShellRequest) {
    const url = new URL(request.url);
    return request.method === 'GET' && request.mode === 'navigate' && url.origin === deps.origin &&
      (url.pathname === `${deps.base}/studio` || url.pathname.startsWith(`${deps.base}/studio/`));
  }
  return {
    handles(request: ShellRequest) {
      return request.method === 'GET' && (navigation(request) || assets.has(request.url));
    },
    async install() {
      // A new version owns a new cache; failure cannot overwrite the active shell.
      try {
        const cache = await deps.storage.open(name);
        for (const url of [shell, ...assets]) {
          const response = await deps.request(url, { credentials: 'omit', redirect: 'error', cache: 'reload' });
          if (!response.ok || response.redirected || (url === shell &&
            (response.headers.get('x-hourpaths-public-shell') !== '1' ||
             !response.headers.get('content-type')?.startsWith('text/html')))) {
            throw new Error('Public shell installation rejected');
          }
          await cache.put(url, response);
        }
      } catch (error) {
        await deps.storage.delete(name);
        throw error;
      }
    },
    async activate() {
      // Activation waits for old clients to close. Never force a timer/draft reload.
      for (const key of await deps.storage.keys()) {
        if (key.startsWith(prefix) && key !== name) await deps.storage.delete(key);
      }
    },
    async respond(request: ShellRequest): Promise<Response> {
      if (navigation(request)) {
        try { return await deps.request(request.url); }
        catch (error) {
          const cached = await (await deps.storage.open(name)).match(shell);
          if (cached) return cached;
          throw error;
        }
      }
      if (!assets.has(request.url) || request.method !== 'GET') throw new Error('Request outside public shell');
      const cached = await (await deps.storage.open(name)).match(request.url);
      return cached ?? deps.request(request.url);
    }
  };
}
