export default ({ config }: { config: Record<string, unknown> }) => {
  const pushProjectId = process.env.HOURPATHS_EXPO_PROJECT_ID?.trim();
  return {
    ...config,
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
