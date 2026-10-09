import type { ProviderSettingsService } from './provider-identities';

/** Keep provider requests and results within their owning presentation lifetime. */
export function providerSettingsLifetime(
  current: () => ProviderSettingsService,
  isActive: () => boolean,
): ProviderSettingsService {
  const assertActive = () => {
    if (!isActive()) throw new Error('settings_presentation_superseded');
  };
  return {
    owner: () => isActive() ? current().owner() : null,
    list: async () => { assertActive(); const rows = await current().list(); assertActive(); return rows; },
    link: async provider => { assertActive(); await current().link(provider); assertActive(); },
    unlink: async (provider, owner) => { assertActive(); await current().unlink(provider, owner); assertActive(); },
  };
}
