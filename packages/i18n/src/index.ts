import { createInstance, type TOptions } from 'i18next';
import en from './locales/en.json';
import es from './locales/es.json';

export const defaultLocale = 'en' as const;
export const supportedLocales = ['en', 'es'] as const;
export type SupportedLocale = (typeof supportedLocales)[number];

type CatalogKey = keyof typeof en;
type PluralKey = CatalogKey extends infer Key
  ? Key extends `${infer Stem}_one`
    ? Stem
    : never
  : never;
export type MessageKey = Exclude<CatalogKey, `${string}_one` | `${string}_other`> | PluralKey;

const problemMessages = {
  bad_request: 'errors.validationFailed',
  validation_failed: 'errors.validationFailed',
  unauthenticated: 'errors.authenticationRejected',
  invalid_credential: 'errors.authenticationRejected',
  forbidden: 'errors.forbidden',
  identity_forbidden: 'errors.forbidden',
  not_found: 'errors.notFound',
  conflict: 'errors.conflict',
  idempotency_conflict: 'errors.idempotencyConflict',
  rate_limited: 'errors.rateLimited',
  authorization_pending: 'errors.authorizationPending',
  authorization_dead_lettered: 'errors.authorizationDeadLettered',
  authorization_policy_not_configured: 'errors.authorizationUnavailable',
  unavailable: 'errors.temporarilyUnavailable',
  internal_error: 'errors.temporarilyUnavailable',
  request_failed: 'errors.apiRejected',
  invalid_identity_token: 'errors.identityTokenRejected',
  identity_exchange_unavailable: 'errors.temporarilyUnavailable',
} as const satisfies Record<string, MessageKey>;

export function problemMessageKey(code: string | undefined): MessageKey {
  return code && Object.hasOwn(problemMessages, code)
    ? problemMessages[code as keyof typeof problemMessages]
    : 'errors.apiRejected';
}

const resources = {
  en: { translation: en },
  es: { translation: es }
} as const;

export interface Translator {
  readonly locale: SupportedLocale;
  t(key: MessageKey, options?: TOptions): string;
  number(value: number, options?: Intl.NumberFormatOptions): string;
  date(value: Date | number, options?: Intl.DateTimeFormatOptions): string;
  time(value: Date | number, options?: Intl.DateTimeFormatOptions): string;
}

export function selectLocale(candidates: readonly (string | null | undefined)[]): SupportedLocale {
  for (const candidate of candidates) {
    const value = candidate?.trim();
    if (!value) continue;
    try {
      const canonical = Intl.getCanonicalLocales(value)[0];
      const language = canonical?.split('-', 1)[0]?.toLowerCase();
      if (supportedLocales.includes(language as SupportedLocale)) return language as SupportedLocale;
    } catch (error) {
      if (!(error instanceof RangeError)) throw error;
    }
  }
  return defaultLocale;
}

export function selectLocaleFromAcceptLanguage(header: string | null | undefined): SupportedLocale {
  const candidates = (header ?? '')
    .split(',')
    .map((entry, index) => {
      const [tag, ...parameters] = entry.split(';').map((part) => part.trim());
      const qualityParameters = parameters.filter((parameter) => /^q\s*=/i.test(parameter));
      const malformedQuality = parameters.some((parameter) => /^q(?:\s|$)/i.test(parameter)
        && !/^q\s*=/i.test(parameter));
      const qualityText = qualityParameters[0]?.replace(/^q\s*=\s*/i, '');
      const quality = qualityParameters.length === 0 && !malformedQuality
        ? 1
        : qualityParameters.length === 1 && !malformedQuality
          && /^(?:0(?:\.\d{0,3})?|1(?:\.0{0,3})?)$/.test(qualityText ?? '')
          ? Number(qualityText)
          : 0;
      return { tag, quality, index };
    })
    .filter(({ tag, quality }) => tag !== '' && tag !== '*' && quality > 0)
    .sort((left, right) => right.quality - left.quality || left.index - right.index)
    .map(({ tag }) => tag);

  return selectLocale(candidates);
}

export function createTranslator(candidates: readonly (string | null | undefined)[]): Translator {
  const locale = selectLocale(candidates);
  const instance = createInstance();
  void instance.init({
    lng: locale,
    fallbackLng: defaultLocale,
    resources,
    keySeparator: false,
    initAsync: false,
    interpolation: { escapeValue: false }
  });

  return {
    locale,
    t: (key, options) => instance.t(key, options),
    number: (value, options) => new Intl.NumberFormat(locale, options).format(value),
    date: (value, options) => new Intl.DateTimeFormat(locale, options).format(value),
    time: (value, options) => new Intl.DateTimeFormat(locale, options).format(value)
  };
}
