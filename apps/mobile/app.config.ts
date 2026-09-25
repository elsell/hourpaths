const releaseTagPattern = /^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
const buildNumberPattern = /^[1-9][0-9]{0,8}$/;

export function resolveMobileReleaseVersion(tag = process.env.HOURPATHS_MOBILE_RELEASE_TAG) {
  const value = tag?.trim() ?? '';
  if (!value) return undefined;
  if (!releaseTagPattern.test(value)) {
    throw new Error('HOURPATHS_MOBILE_RELEASE_TAG must be an exact vMAJOR.MINOR.PATCH tag.');
  }
  return value.slice(1);
}

export function resolveMobileBuildNumber(value = process.env.HOURPATHS_MOBILE_BUILD_NUMBER) {
  const normalized = value?.trim() ?? '';
  if (!normalized) return undefined;
  if (!buildNumberPattern.test(normalized)) {
    throw new Error('HOURPATHS_MOBILE_BUILD_NUMBER must be a positive numeric build number.');
  }
  const numeric = Number(normalized);
  if (!Number.isSafeInteger(numeric)) {
    throw new Error('HOURPATHS_MOBILE_BUILD_NUMBER must be a safe integer.');
  }
  return { numeric, text: normalized };
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

export default ({ config }: { config: Record<string, unknown> }) => {
  const releaseTag = process.env.HOURPATHS_MOBILE_RELEASE_TAG?.trim() ?? '';
  const configuredBuildNumber = process.env.HOURPATHS_MOBILE_BUILD_NUMBER?.trim() ?? '';
  if (Boolean(releaseTag) !== Boolean(configuredBuildNumber)) {
    throw new Error('HOURPATHS_MOBILE_RELEASE_TAG and HOURPATHS_MOBILE_BUILD_NUMBER must be provided together.');
  }

  const releaseVersion = resolveMobileReleaseVersion(releaseTag);
  const releaseBuildNumber = resolveMobileBuildNumber(configuredBuildNumber);
  const pushProjectId = process.env.HOURPATHS_EXPO_PROJECT_ID?.trim();
  const releaseConfig = releaseVersion && releaseBuildNumber
    ? {
        version: releaseVersion,
        ios: { ...asRecord(config.ios), buildNumber: releaseBuildNumber.text },
        android: { ...asRecord(config.android), versionCode: releaseBuildNumber.numeric },
      }
    : {};

  return {
    ...config,
    ...releaseConfig,
    extra: {
      ...((config.extra as Record<string, unknown> | undefined) ?? {}),
      ...(pushProjectId ? { eas: { projectId: pushProjectId } } : {}),
      hourpaths: {
        environment: process.env.HOURPATHS_APP_ENV ?? 'development',
        apiURL: process.env.HOURPATHS_API_URL ?? 'http://localhost:8080',
        oidcIssuer: process.env.HOURPATHS_OIDC_ISSUER ?? 'http://localhost:5556/dex',
        oidcClientId: process.env.HOURPATHS_MOBILE_OIDC_CLIENT_ID ?? 'hourpaths-mobile',
        ...(pushProjectId ? { pushProjectId } : {}),
      },
    },
  };
};
