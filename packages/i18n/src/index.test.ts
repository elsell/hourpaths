import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator, problemMessageKey, selectLocale, selectLocaleFromAcceptLanguage } from './index';

test('selects supported base locales and falls back safely', () => {
  assert.equal(selectLocale(['es-MX']), 'es');
  assert.equal(selectLocale(['ES-mx']), 'es');
  assert.equal(selectLocale(['zz-ZZ']), 'en');
  assert.equal(selectLocale(['es--MX']), 'en');
  assert.equal(selectLocale(['not_a_locale', 'es-MX']), 'es');
  assert.equal(selectLocale([]), 'en');
});

test('selects canonical locales when Intl.Locale is unavailable', () => {
  const descriptor = Object.getOwnPropertyDescriptor(Intl, 'Locale');
  Object.defineProperty(Intl, 'Locale', { configurable: true, value: undefined });
  try {
    assert.equal(selectLocale(['es-MX']), 'es');
    assert.equal(selectLocale(['ES-mx']), 'es');
    assert.equal(selectLocale(['zz-ZZ']), 'en');
  } finally {
    if (descriptor) Object.defineProperty(Intl, 'Locale', descriptor);
  }
});

test('selects Accept-Language candidates by descending quality', () => {
  assert.equal(selectLocaleFromAcceptLanguage('en;q=0.4, es-MX;q=0.9'), 'es');
});

test('excludes zero-quality and wildcard language ranges', () => {
  assert.equal(selectLocaleFromAcceptLanguage('es;q=0, en;q=0.5'), 'en');
  assert.equal(selectLocaleFromAcceptLanguage('*;q=1, es;q=0.5'), 'es');
  assert.equal(selectLocaleFromAcceptLanguage('*;q=1'), 'en');
});

test('ignores malformed language tags and quality values', () => {
  assert.equal(selectLocaleFromAcceptLanguage('es--MX;q=1, es;q=0.8'), 'es');
  assert.equal(selectLocaleFromAcceptLanguage('es;q=wat, en;q=0.8'), 'en');
  assert.equal(selectLocaleFromAcceptLanguage('es;q=1.1, en;q=0.8'), 'en');
  assert.equal(selectLocaleFromAcceptLanguage('es;q;q=1, en;q=0.8'), 'en');
});

test('preserves header order for equal quality candidates', () => {
  assert.equal(selectLocaleFromAcceptLanguage('es;q=0.8, en;q=0.8'), 'es');
  assert.equal(selectLocaleFromAcceptLanguage('en;q=0.8, es;q=0.8'), 'en');
});

test('uses supported regional tags and falls back for unsupported headers', () => {
  assert.equal(selectLocaleFromAcceptLanguage('fr-FR, es-MX;q=0.5'), 'es');
  assert.equal(selectLocaleFromAcceptLanguage('fr-FR, de;q=0.8'), 'en');
  assert.equal(selectLocaleFromAcceptLanguage(undefined), 'en');
});

test('translates interpolation and plurals', () => {
  const translator = createTranslator(['es-MX']);
  assert.equal(translator.t('auth.signedInAs', { name: 'Ada' }), 'Sesión iniciada como Ada');
  assert.equal(translator.t('examples.count', { count: 1 }), '1 ejemplo');
  assert.equal(translator.t('examples.count', { count: 2 }), '2 ejemplos');
});

test('duplicate-email recovery decline is localized in every supported catalog', () => {
  assert.equal(createTranslator(['en']).t('duplicateEmailRecovery.decline'), 'Create a separate account');
  assert.equal(createTranslator(['es']).t('duplicateEmailRecovery.decline'), 'Crear una cuenta separada');
});

test('overall progress communicates accumulated, target, and completion without color', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  const values = { accumulated: 150, target: 120 };
  assert.equal(english.t('path.progress.overallComplete', values), '150 of 120 seconds — overall target completed');
  assert.equal(spanish.t('path.progress.overallComplete', values), '150 de 120 segundos — objetivo total completado');
  assert.notEqual(english.t('path.progress.overall', values), english.t('path.progress.overallComplete', values));
  assert.equal(english.t('path.progress.overallLabel'), 'Overall target progress');
  assert.equal(spanish.t('path.progress.overallLabel'), 'Progreso del objetivo total');
  assert.equal(english.t('path.progress.overallLabelForPath', { path: 'Read' }), 'Overall target progress for Read');
  assert.equal(spanish.t('path.progress.overallLabelForPath', { path: 'Leer' }), 'Progreso del objetivo total de Leer');
});

test('active timer accessibility includes the current formatted duration', () => {
  assert.equal(createTranslator(['en']).t('timer.elapsedValue', { duration: '1:02' }), 'Active timer: 1:02');
  assert.equal(createTranslator(['es']).t('timer.elapsedValue', { duration: '1:02' }), 'Temporizador activo: 1:02');
});

test('formats numbers, dates, and times with the selected locale', () => {
  const translator = createTranslator(['es']);
  const date = new Date('2026-01-02T12:00:00Z');
  const dateOptions = { dateStyle: 'medium', timeZone: 'UTC' } as const;
  const timeOptions = { timeStyle: 'short', timeZone: 'UTC' } as const;
  assert.equal(translator.number(1234.5), new Intl.NumberFormat('es').format(1234.5));
  assert.equal(translator.date(date, dateOptions), new Intl.DateTimeFormat('es', dateOptions).format(date));
  assert.equal(translator.time(date, timeOptions), new Intl.DateTimeFormat('es', timeOptions).format(date));
});

test('stable problem codes select safe localized catalog keys', () => {
  assert.equal(problemMessageKey('idempotency_conflict'), 'errors.idempotencyConflict');
  assert.equal(problemMessageKey('authorization_pending'), 'errors.authorizationPending');
  assert.equal(problemMessageKey('authorization_dead_lettered'), 'errors.authorizationDeadLettered');
  assert.equal(problemMessageKey('authorization_policy_not_configured'), 'errors.authorizationUnavailable');
  assert.equal(problemMessageKey('invalid_identity_token'), 'errors.identityTokenRejected');
  assert.equal(problemMessageKey('rate_limited'), 'errors.rateLimited');
  assert.equal(problemMessageKey('unknown_server_code'), 'errors.apiRejected');
  assert.equal(problemMessageKey('<script>hostile</script>'), 'errors.apiRejected');
  assert.equal(problemMessageKey(undefined), 'errors.apiRejected');

  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  for (const code of ['idempotency_conflict', 'authorization_pending', 'authorization_dead_lettered'] as const) {
    const key = problemMessageKey(code);
    assert.notEqual(english.t(key), key);
    assert.notEqual(spanish.t(key), key);
    assert.notEqual(english.t(key), spanish.t(key));
  }
});
