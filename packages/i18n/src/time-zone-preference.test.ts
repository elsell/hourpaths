import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

const keys = [
  'settings.timeZone.heading',
  'settings.timeZone.openLabel',
  'settings.timeZone.current',
  'settings.timeZone.searchPlaceholder',
  'settings.timeZone.warningTitle',
  'settings.timeZone.warning',
  'settings.timeZone.confirm',
  'settings.timeZone.cancel',
  'settings.timeZone.loading',
  'settings.timeZone.unavailableHeading',
  'settings.timeZone.unavailableDescription',
  'settings.timeZone.retry',
  'settings.timeZone.saving',
  'settings.timeZone.saveError',
  'settings.timeZone.empty',
] as const;

test('time-zone review and native settings states are complete in English and Spanish', () => {
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of keys) {
      const message = translator.t(key, {
        current: 'America/New_York',
        proposed: 'Europe/Paris',
        timeZone: 'America/New_York',
      });
      assert.notEqual(message, key);
      assert.ok(message.trim().length > 0);
    }
  }
});

test('confirmation explains reprojection and retained timer/activity zones', () => {
  const warning = createTranslator(['en']).t('settings.timeZone.warning', {
    current: 'America/New_York',
    proposed: 'Europe/Paris',
  });
  assert.match(warning, /current interval and remaining time/i);
  assert.match(warning, /running timer and previously recorded activity keep/i);
});
