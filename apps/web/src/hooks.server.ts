import type { Handle } from '@sveltejs/kit';
import { selectLocaleFromAcceptLanguage } from '@hourpaths/i18n';
import { webConfig } from '$lib/server/config';

export const handle: Handle = async ({ event, resolve }) => {
  const { apiURL, oidcIssuer } = webConfig;
  event.locals.locale = selectLocaleFromAcceptLanguage(event.request.headers.get('accept-language'));
  const response = await resolve(event, {
    transformPageChunk: ({ html }) => html.replace('lang="en"', `lang="${event.locals.locale}"`)
  });
  response.headers.append('Vary', 'Accept-Language');
  const connectOrigins = new Set<string>(["'self'"]);
  for (const configured of [apiURL, oidcIssuer]) connectOrigins.add(new URL(configured).origin);
  const generatedCSP = response.headers.get('Content-Security-Policy');
  response.headers.set(
    'Content-Security-Policy',
    generatedCSP?.replace(/connect-src [^;]+/, ['connect-src', ...connectOrigins].join(' ')) ?? "default-src 'none'"
  );
  response.headers.set('Referrer-Policy', 'no-referrer');
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('X-Frame-Options', 'DENY');
  response.headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');
  response.headers.set('Strict-Transport-Security', 'max-age=31536000; includeSubDomains');
  return response;
};
