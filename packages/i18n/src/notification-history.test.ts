import assert from 'node:assert/strict';
import test from 'node:test';
import en from './locales/en.json';
import es from './locales/es.json';
import { createTranslator } from './index.js';

const notificationKeys = [
  'notification.bellLabel',
  'notification.heading',
  'notification.actionableHeading',
  'notification.informationalHeading',
  'notification.empty',
  'notification.loading',
  'notification.loadMore',
  'notification.loadingMore',
  'notification.error',
  'notification.unreadCount',
  'notification.unread',
  'notification.markAllRead',
  'notification.delete',
  'notification.mutationError',
  'notification.itemUnavailable',
  'notification.pathInvitationReceived.participant',
  'notification.pathInvitationReceived.supporter',
  'notification.pathInvitationAccepted.participant',
  'notification.pathInvitationAccepted.supporter',
  'notification.pathOwnershipTransferReceived',
  'notification.pathOwnershipTransferAccepted',
  'notification.pathOwnershipTransferDeclined',
  'notification.pathOwnershipTransferCanceled',
  'notification.practiceReaction.heart',
  'notification.practiceReaction.applause',
  'notification.practiceReaction.fire',
  'notification.practiceReaction.strong',
  'notification.practiceReaction.celebrate',
  'notification.practiceComment',
  'notification.commentHeart',
] as const;

function placeholders(message: string): string[] {
  return [...message.matchAll(/{{\s*([^},\s]+)[^}]*}}/g)]
    .map((match) => match[1]!)
    .sort();
}

test('notification experience copy exists in every locale with matching interpolation', () => {
  for (const key of notificationKeys) {
    assert.equal(typeof en[key], 'string', `English ${key}`);
    assert.equal(typeof es[key], 'string', `Spanish ${key}`);
    assert.notEqual(en[key], '', `English ${key}`);
    assert.notEqual(es[key], '', `Spanish ${key}`);
    assert.deepEqual(placeholders(es[key]), placeholders(en[key]), `${key} placeholders`);
  }
});

test('unavailable notification feedback is generic and contains no stale identity or Path interpolation', () => {
  for (const locale of ['en', 'es']) {
    const message = createTranslator([locale]).t('notification.itemUnavailable');
    assert.notEqual(message, '');
    assert.doesNotMatch(message, /{{|path|ruta|actor|username|usuario|name|nombre/i);
  }
});

test('ownership transfer notices identify the actor and Path without leaking private fields', () => {
  const values = {
    displayName: 'Reader One',
    username: 'Reader.One',
    pathName: 'Morning Reading',
  };
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of [
      'notification.pathOwnershipTransferReceived',
      'notification.pathOwnershipTransferAccepted',
      'notification.pathOwnershipTransferDeclined',
      'notification.pathOwnershipTransferCanceled',
    ] as const) {
      const message = translator.t(key, values);
      assert.match(message, /Reader One/);
      assert.match(message, /@Reader\.One/);
      assert.match(message, /Morning Reading/);
      assert.doesNotMatch(message, /email|correo|profile|perfil/i);
    }
  }
});

test('Path invitation notices identify actor, Path, and localized offered role', () => {
  const values = {
    displayName: 'Reader One',
    username: 'Reader.One',
    pathName: 'Morning Reading',
  };
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of [
      'notification.pathInvitationReceived.participant',
      'notification.pathInvitationReceived.supporter',
      'notification.pathInvitationAccepted.participant',
      'notification.pathInvitationAccepted.supporter',
    ] as const) {
      const message = translator.t(key, values);
      assert.match(message, /Reader One/);
      assert.match(message, /@Reader\.One/);
      assert.match(message, /Morning Reading/);
      assert.doesNotMatch(message, /email|correo|profile|perfil/i);
    }
    assert.notEqual(
      translator.t('notification.pathInvitationReceived.participant', values),
      translator.t('notification.pathInvitationReceived.supporter', values),
    );
  }
});

test('notification sections communicate actionability without warning language', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  assert.match(english.t('notification.actionableHeading'), /action/i);
  assert.match(english.t('notification.informationalHeading'), /update/i);
  assert.match(spanish.t('notification.actionableHeading'), /acción/i);
  assert.match(spanish.t('notification.informationalHeading'), /novedad/i);
  for (const locale of [english, spanish]) {
    assert.doesNotMatch(
      locale.t('notification.informationalHeading'),
      /warning|error|advertencia/i,
    );
  }
});

test('reaction notices identify actor, curated reaction, and Path without assuming the event is practice', () => {
  const values = { displayName: 'Alex Rivera', username: 'alex', pathName: 'Piano' };
  const expected = {
    en: ['Heart', 'Applause', 'Fire', 'Strong', 'Celebrate'],
    es: ['Corazón', 'Aplausos', 'Fuego', 'Fuerza', 'Celebración'],
  } as const;
  const emoji = ['❤️', '👏', '🔥', '💪', '🎉'];
  const reactions = ['heart', 'applause', 'fire', 'strong', 'celebrate'] as const;

  for (const locale of ['en', 'es'] as const) {
    const translator = createTranslator([locale]);
    reactions.forEach((reaction, index) => {
      const message = translator.t(`notification.practiceReaction.${reaction}`, values);
      assert.match(message, /Alex Rivera/);
      assert.match(message, /@alex/);
      assert.match(message, /Piano/);
      assert.match(message, new RegExp(emoji[index]!));
      assert.match(message, new RegExp(expected[locale][index]!));
      assert.doesNotMatch(message, /practice|práctica/i);
      assert.match(message, locale === 'en' ? /your update/ : /tu actualización/);
    });
  }
});

test('comment-heart notices identify the actor and Path without private identity', () => {
  const values = { displayName: 'Alex Rivera', username: 'alex', pathName: 'Piano' };
  for (const locale of ['en', 'es']) {
    const message = createTranslator([locale]).t('notification.commentHeart', values);
    assert.match(message, /Alex Rivera/);
    assert.match(message, /@alex/);
    assert.match(message, /Piano/);
    assert.doesNotMatch(message, /email|correo/i);
  }
});

test('bell, loading, pagination, empty, and failure states have concise copy', () => {
  for (const locale of [createTranslator(['en']), createTranslator(['es'])]) {
    for (const key of [
      'notification.bellLabel',
      'notification.empty',
      'notification.loading',
      'notification.loadMore',
      'notification.loadingMore',
      'notification.error',
    ] as const) {
      assert.ok(locale.t(key).trim().length > 0, `${locale.locale} ${key}`);
    }
  }
});
