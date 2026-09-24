import assert from 'node:assert/strict';
import test from 'node:test';
import en from './locales/en.json';
import es from './locales/es.json';
import { createTranslator } from './index.js';

const nudgeKeys = [
  'nudge.section',
  'nudge.sendAction',
  'nudge.compose.title',
  'nudge.compose.pathContext',
  'nudge.compose.choosePreset',
  'nudge.compose.cancel',
  'nudge.compose.send',
  'nudge.compose.sending',
  'nudge.compose.sent',
  'nudge.compose.sendError',
  'nudge.eligibility.goalComplete',
  'nudge.eligibility.rateLimited',
  'nudge.preset.you_have_got_this',
  'nudge.preset.lets_go',
  'nudge.preset.little_progress_counts',
  'nudge.preset.keep_it_going',
  'nudge.preset.time_to_work',
  'nudge.audience.openLabel',
  'nudge.audience.heading',
  'nudge.audience.label',
  'nudge.audience.footer',
  'nudge.audience.nobody',
  'nudge.audience.path_members',
  'nudge.audience.followers',
  'nudge.audience.everyone',
  'nudge.audience.saving',
  'nudge.audience.saved',
  'nudge.audience.unavailableHeading',
  'nudge.audience.loadError',
  'nudge.audience.saveError',
  'nudge.channel.label',
  'nudge.channel.on',
  'nudge.channel.off',
  'nudge.channel.loading',
  'nudge.channel.loadError',
  'nudge.channel.saving',
  'nudge.channel.saveError',
  'nudge.channel.footer',
  'notification.nudge.you_have_got_this',
  'notification.nudge.lets_go',
  'notification.nudge.little_progress_counts',
  'notification.nudge.keep_it_going',
  'notification.nudge.time_to_work',
  'notification.settings.nudges',
] as const;

function placeholders(message: string): string[] {
  return [...message.matchAll(/{{\s*([^},\s]+)[^}]*}}/g)]
    .map((match) => match[1]!)
    .sort();
}

test('the complete nudge experience is localized with matching interpolation', () => {
  for (const key of nudgeKeys) {
    assert.equal(typeof en[key], 'string', `English ${key}`);
    assert.equal(typeof es[key], 'string', `Spanish ${key}`);
    assert.notEqual(en[key], '', `English ${key}`);
    assert.notEqual(es[key], '', `Spanish ${key}`);
    assert.deepEqual(placeholders(es[key]), placeholders(en[key]), `${key} placeholders`);
  }
});

test('curated encouragements are positive, distinct, and contain no custom-message affordance', () => {
  for (const locale of [createTranslator(['en']), createTranslator(['es'])]) {
    const messages = [
      locale.t('nudge.preset.you_have_got_this'),
      locale.t('nudge.preset.lets_go'),
      locale.t('nudge.preset.little_progress_counts'),
      locale.t('nudge.preset.keep_it_going'),
      locale.t('nudge.preset.time_to_work'),
    ];
    assert.equal(new Set(messages).size, 5);
    assert.equal(messages.every((message) => message.trim() === message && message.length > 0), true);
    assert.doesNotMatch(messages.join(' '), /lazy|behind|failed|failure|shame|perezos|atrasad|fracas|vergüenza/i);
  }
  assert.equal(nudgeKeys.some((key) => /(?:^|\.)(?:custom|message|text)(?:\.|$)/i.test(key)), false);
});

test('received-nudge notices identify sender, Path, and the selected localized preset', () => {
  const values = { displayName: 'Taylor Reader', pathName: 'Cello', username: 'taylor' };
  for (const locale of [createTranslator(['en']), createTranslator(['es'])]) {
    for (const preset of [
      'you_have_got_this',
      'lets_go',
      'little_progress_counts',
      'keep_it_going',
      'time_to_work',
    ] as const) {
      const notice = locale.t(`notification.nudge.${preset}`, values);
      assert.match(notice, /Taylor Reader/);
      assert.match(notice, /@taylor/);
      assert.match(notice, /Cello/);
      assert.match(notice, new RegExp(locale.t(`nudge.preset.${preset}`).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
    }
  }
});

test('audience copy keeps personal scope and access limits explicit', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  assert.match(english.t('nudge.audience.footer'), /Path|visibility|block/i);
  assert.match(spanish.t('nudge.audience.footer'), /Ruta|visibilidad|bloque/i);
  assert.match(english.t('nudge.audience.path_members'), /members/i);
  assert.match(spanish.t('nudge.audience.path_members'), /miembros/i);
});

test('notification channel copy describes one concise delivery toggle and recoverable states', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  assert.match(english.t('nudge.channel.footer'), /in-app|push/i);
  assert.match(spanish.t('nudge.channel.footer'), /aplicación|push/i);
  assert.match(english.t('nudge.channel.saveError'), /try again/i);
  assert.match(spanish.t('nudge.channel.saveError'), /inténtalo de nuevo/i);
});
