import assert from 'node:assert/strict';
import test from 'node:test';
import { createDeviceTranslator } from './i18n';
import { formatCompactDuration } from './ui/compact-duration';

test('uses the Expo device locale list for translated rendering', () => {
  const translator = createDeviceTranslator(() => [{ languageTag: 'es-MX' }]);
  assert.equal(translator.locale, 'es');
  assert.equal(translator.t('auth.signIn'), 'Iniciar sesión');
  assert.equal(translator.t('examples.count', { count: 2 }), '2 ejemplos');
});

test('falls back when the device reports no supported locale', () => {
  const translator = createDeviceTranslator(() => [{ languageTag: 'zz-ZZ' }]);
  assert.equal(translator.locale, 'en');
  assert.equal(translator.t('auth.signIn'), 'Sign in');
});

test('compact durations choose seconds, minutes, and hours in every supported locale', () => {
  const english = createDeviceTranslator(() => [{ languageTag: 'en-US' }]);
  const spanish = createDeviceTranslator(() => [{ languageTag: 'es-MX' }]);

  assert.equal(formatCompactDuration(0, english), '0 sec');
  assert.equal(formatCompactDuration(59, english), '59 sec');
  assert.equal(formatCompactDuration(61, english), '1 min 1 sec');
  assert.equal(formatCompactDuration(3_661, english), '1 hr 1 min');
  assert.equal(formatCompactDuration(61, spanish), '1 min 1 s');
  assert.equal(formatCompactDuration(3_661, spanish), '1 h 1 min');
});
