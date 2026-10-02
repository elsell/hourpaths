/** Reviewed browser-only lifecycle adapter. It performs no network requests. */
export function browserConnected(): boolean { return navigator.onLine; }
export function subscribeTrackingConnectivity(listener: () => void): () => void {
  window.addEventListener('online', listener);
  window.addEventListener('offline', listener);
  window.addEventListener('focus', listener);
  return () => {
    window.removeEventListener('online', listener);
    window.removeEventListener('offline', listener);
    window.removeEventListener('focus', listener);
  };
}
