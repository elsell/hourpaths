import { clientRuntimeConfig, type ClientRuntimeConfig } from '@hourpaths/client-core';

export function parseWebConfig(value: unknown): ClientRuntimeConfig {
  return clientRuntimeConfig(value);
}
