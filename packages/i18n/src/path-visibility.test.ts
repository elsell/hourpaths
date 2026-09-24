import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

const keys = [
  'pathVisibility.heading',
  'pathVisibility.current',
  'pathVisibility.choiceLabel',
  'pathVisibility.option.private',
  'pathVisibility.option.followers',
  'pathVisibility.option.public',
  'pathVisibility.save',
  'pathVisibility.saving',
  'pathVisibility.saved',
  'pathVisibility.unavailable',
  'pathVisibility.confirmation.heading',
  'pathVisibility.confirmation.transition',
  'pathVisibility.confirmation.historyExposure',
  'pathVisibility.confirmation.unchangedScope',
  'pathVisibility.confirmation.confirm',
  'notification.pathVisibilityChanged',
] as const;

test('PATH-05C visibility controls and status copy exist in every locale', () => {
  for (const locale of ['en', 'es']) {
    const i18n = createTranslator([locale]);
    for (const key of keys) assert.notEqual(i18n.t(key), key, `${locale}: ${key}`);
  }
});

test('visibility notification names newly visible Path history and explicitly unchanged external scope', () => {
  for (const locale of ['en', 'es']) {
    const i18n = createTranslator([locale]);
    const message = i18n.t('notification.pathVisibilityChanged', {
      displayName: 'Alex',
      pathName: 'Reading',
      pathVisibility: locale === 'en' ? 'Followers' : 'Seguidores',
    });
    assert.match(message, /Alex/);
    assert.match(message, /Reading/);
    assert.match(message, locale === 'en'
      ? /Followers.*participant identity.*progress.*activity.*profile.*unrelated Paths.*unchanged/i
      : /Seguidores.*identidad.*participantes.*progreso.*actividad.*perfil.*rutas no relacionadas.*sin cambios/i);
  }
});

test('broader confirmation names the Path and both audiences without implying unrelated changes', () => {
  const values = { pathName: 'Reading', current: 'Private', proposed: 'Followers' };
  for (const locale of ['en', 'es']) {
    const i18n = createTranslator([locale]);
    const transition = i18n.t('pathVisibility.confirmation.transition', values);
    assert.match(transition, /Reading/);
    assert.match(transition, /Private/);
    assert.match(transition, /Followers/);
    const warning = [
      i18n.t('pathVisibility.confirmation.historyExposure'),
      i18n.t('pathVisibility.confirmation.unchangedScope'),
    ].join(' ');
    assert.match(warning, locale === 'en' ? /historical.*identity.*progress.*activity/i : /históric.*identidad.*progreso.*actividad/i);
    assert.match(warning, locale === 'en' ? /membership.*recorded activity.*progress.*profile.*unrelated Paths/i : /membresía.*actividad registrada.*progreso.*perfil.*rutas no relacionadas/i);
  }
});
