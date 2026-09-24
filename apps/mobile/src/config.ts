import { clientRuntimeConfig, type ClientRuntimeConfig } from '@hourpaths/client-core';

export type MobileRuntimeConfig = ClientRuntimeConfig & {
  pushProjectId: string | null;
};

export function loadMobileConfig(extra: unknown): MobileRuntimeConfig {
  if (!extra || typeof extra !== 'object') throw new Error('HOURPATHS_CLIENT_CONFIG');
  const hourpaths = (extra as { hourpaths?: unknown }).hourpaths;
  if (!hourpaths || typeof hourpaths !== 'object' || Array.isArray(hourpaths)) {
    throw new Error('HOURPATHS_CLIENT_CONFIG');
  }
  const record = hourpaths as Record<string, unknown>;
  const client = clientRuntimeConfig({
    environment: record.environment,
    apiURL: record.apiURL,
    oidcIssuer: record.oidcIssuer,
    oidcClientId: record.oidcClientId,
  });
  const pushProjectId = record.pushProjectId ?? null;
  if (
    pushProjectId !== null &&
    (typeof pushProjectId !== 'string' ||
      !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/iu.test(pushProjectId))
  ) {
    throw new Error('HOURPATHS_PUSH_PROJECT_ID');
  }
  if (client.environment === 'production' && pushProjectId === null) {
    throw new Error('HOURPATHS_PUSH_PROJECT_ID');
  }
  return { ...client, pushProjectId };
}
