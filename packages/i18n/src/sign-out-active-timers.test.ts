import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('active timer sign-out choices are concise and correctly pluralized', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.match(english.t('settings.account.activeTimers.message', { count: 1 }), /1 running timer\./);
  assert.match(english.t('settings.account.activeTimers.message', { count: 2 }), /2 running timers\./);
  assert.match(spanish.t('settings.account.activeTimers.message', { count: 1 }), /1 temporizador en curso\./);
  assert.match(spanish.t('settings.account.activeTimers.message', { count: 2 }), /2 temporizadores en curso\./);
  assert.equal(
    english.t('settings.account.activeTimers.stopAndSignOut'),
    'Stop and Save, Then Sign Out',
  );
  assert.equal(
    english.t('settings.account.activeTimers.keepRunningAndSignOut'),
    'Keep Running and Sign Out',
  );
});
