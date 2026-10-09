// Transport-level fan-out only. Client application lifetimes own subscriptions
// and decide how to present the server's policy requirement.
type Observer = { baseURL: string; token: () => Promise<string | null> | string | null; notify: () => void };
const observers = new Set<Observer>();
const normalizedBase = (value: string) => value.replace(/\/+$/, '');
export function observePolicyRequirement(baseURL: string, token: Observer['token'], notify: Observer['notify']): () => void {
  const observer = { baseURL: normalizedBase(baseURL), token, notify };
  observers.add(observer);
  return () => { observers.delete(observer); };
}
export async function notifyPolicyRequirement(baseURL: string, token: string | null): Promise<void> {
  if (!token) return;
  await Promise.all([...observers].map(async observer => {
    try {
      if (observer.baseURL !== normalizedBase(baseURL) || await observer.token() !== token || !observers.has(observer)) return;
      await observer.notify();
    } catch { /* UI observers must not replace the authoritative HTTP result. */ }
  }));
}
