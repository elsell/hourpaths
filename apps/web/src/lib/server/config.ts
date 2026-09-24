import { env } from '$env/dynamic/private';
import { parseWebConfig } from '$lib/config';

export function loadWebConfig() {
  return parseWebConfig({
    environment: env.HOURPATHS_APP_ENV ?? 'development',
    apiURL: env.HOURPATHS_API_URL ?? 'http://localhost:8080',
    oidcIssuer: env.HOURPATHS_OIDC_ISSUER ?? 'http://localhost:5556/dex',
    oidcClientId: env.HOURPATHS_WEB_OIDC_CLIENT_ID ?? 'hourpaths-web',
  });
}

export const webConfig = loadWebConfig();
