export interface ProviderLogoutPorts {
  pending(): Promise<boolean>;
  markPending(): Promise<void>;
  clearPending(): Promise<void>;
  endSession(): Promise<boolean>;
}

/** A canceled browser operation must never silently admit another authorization. */
export function createProviderLogout(ports: ProviderLogoutPorts) {
  let running: Promise<boolean> | null = null;
  const complete = () => {
    if (running) return running;
    running = Promise.resolve().then(async () => {
      try {
        await ports.markPending();
        if (!await ports.endSession()) return false;
        await ports.clearPending();
        return true;
      } catch { return false; }
    }).finally(() => { running = null; });
    return running;
  };
  return {
    signOut: complete,
    async beforeSignIn() {
      try { return await ports.pending() ? complete() : true; }
      catch { return false; }
    },
  };
}

export function validProviderLogoutReturn(url: string, redirectUri: string, state: string): boolean {
  try {
    const result = new URL(url);
    const expected = new URL(redirectUri);
    return result.origin === expected.origin && result.protocol === expected.protocol &&
      result.host === expected.host && result.pathname === expected.pathname &&
      !result.hash && !result.username && !result.password &&
      result.searchParams.getAll('state').length === 1 && result.searchParams.get('state') === state &&
      !result.searchParams.has('error') && !result.searchParams.has('code');
  } catch { return false; }
}
