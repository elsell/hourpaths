/// <reference lib="webworker" />
import { base, build, version } from '$service-worker';
import { createShellCache } from './lib/offline-shell/cache';

// Checksum-reviewed worker capability bootstrap; only the public cache adapter
// classifies requests. API, OIDC and account storage are never intercepted.
const worker = self as unknown as ServiceWorkerGlobalScope;
const shell = createShellCache({
  origin: worker.location.origin, base, version, assets: build,
  storage: worker.caches,
  request: (url, init) => worker.fetch(url, init)
});
worker.addEventListener('install', event => event.waitUntil(shell.install()));
worker.addEventListener('activate', event => event.waitUntil(shell.activate()));
worker.addEventListener('fetch', event => {
  if (shell.handles(event.request)) event.respondWith(shell.respond(event.request));
});
