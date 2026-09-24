import assert from 'node:assert/strict';
import test from 'node:test';
import en from './locales/en.json';
import es from './locales/es.json';
import { createTranslator } from './index.js';

const invitationKeys = [
  'pathInvitation.share',
  'pathInvitation.heading',
  'pathInvitation.inviteAction',
  'pathInvitation.usernameLabel',
  'pathInvitation.usernameHint',
  'pathInvitation.review',
  'pathInvitation.reviewing',
  'pathInvitation.reviewedIdentity',
  'pathInvitation.roleLabel',
  'pathInvitation.role.participant',
  'pathInvitation.role.supporter',
  'pathInvitation.role.participantEffect',
  'pathInvitation.role.supporterEffect',
  'pathInvitation.confirmHeading',
  'pathInvitation.confirmSend',
  'pathInvitation.send',
  'pathInvitation.sending',
  'pathInvitation.sent',
  'pathInvitation.managed.heading',
  'pathInvitation.managed.loading',
  'pathInvitation.managed.loadingMore',
  'pathInvitation.managed.unavailableHeading',
  'pathInvitation.managed.emptyHeading',
  'pathInvitation.managed.emptyDescription',
  'pathInvitation.managed.count_one',
  'pathInvitation.managed.count_other',
  'pathInvitation.managed.recipient',
  'pathInvitation.managed.role',
  'pathInvitation.managed.inviter',
  'pathInvitation.managed.sentAt',
  'pathInvitation.managed.cancel',
  'pathInvitation.managed.canceling',
  'pathInvitation.managed.cancelFor',
  'pathInvitation.managed.cancelingFor',
  'pathInvitation.managed.cancelConfirmationHeading',
  'pathInvitation.managed.cancelConfirmationBody',
  'pathInvitation.pendingHeading',
  'pathInvitation.pendingEmpty',
  'pathInvitation.pendingCount_one',
  'pathInvitation.pendingCount_other',
  'pathInvitation.pendingContext',
  'pathInvitation.accept',
  'pathInvitation.accepting',
  'pathInvitation.accepted',
  'pathInvitation.participantTracking',
  'pathInvitation.supporterReadOnly',
  'pathInvitation.unavailable',
  'pathInvitation.warningRequired',
  'pathInvitation.visibilityWarning.heading',
  'pathInvitation.visibilityWarning.audience.followers',
  'pathInvitation.visibilityWarning.audience.public',
  'pathInvitation.visibilityWarning.exposure',
  'pathInvitation.visibilityWarning.privacyScope',
  'pathInvitation.visibilityWarning.retainedActivity',
  'pathInvitation.visibilityWarning.confirm',
  'pathInvitation.visibilityWarning.cancel',
  'pathInvitation.retry',
  'pathInvitation.rateLimited',
  'pathInvitation.dependencyUnavailable',
  'pathInvitation.failure',
] as const;

function placeholders(message: string): string[] {
  return [...message.matchAll(/{{\s*([^},\s]+)[^}]*}}/g)]
    .map((match) => match[1]!)
    .sort();
}

test('PATH-03 invitation copy exists with matching interpolation and plural structure', () => {
  for (const key of invitationKeys) {
    assert.equal(typeof en[key], 'string', `English ${key}`);
    assert.equal(typeof es[key], 'string', `Spanish ${key}`);
    assert.notEqual(en[key], '', `English ${key}`);
    assert.notEqual(es[key], '', `Spanish ${key}`);
    assert.deepEqual(placeholders(es[key]), placeholders(en[key]), `${key} placeholders`);
  }
  for (const catalog of [en, es]) {
    assert.equal(
      Object.hasOwn(catalog, 'pathInvitation.pendingCount_one'),
      Object.hasOwn(catalog, 'pathInvitation.pendingCount_other'),
    );
  }
});

test('review and confirmation identify the canonical person without provider email', () => {
  const values = { displayName: 'Reader One', username: 'Reader.One', role: 'participant' };
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    const reviewed = translator.t('pathInvitation.reviewedIdentity', values);
    const confirmation = translator.t('pathInvitation.confirmSend', values);
    assert.match(reviewed, /Reader One/);
    assert.match(reviewed, /@Reader\.One/);
    assert.match(confirmation, /Reader One/);
    assert.match(confirmation, /@Reader\.One/);
    assert.doesNotMatch(`${reviewed} ${confirmation}`, /email|correo/i);
  }
});

test('participant and supporter effects explicitly distinguish tracking from read-only access', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  assert.match(english.t('pathInvitation.role.participantEffect'), /track their own time/i);
  assert.match(english.t('pathInvitation.role.supporterEffect'), /view.*cannot track/i);
  assert.match(english.t('pathInvitation.supporterReadOnly'), /read-only/i);
  assert.match(spanish.t('pathInvitation.role.participantEffect'), /registrar su propio tiempo/i);
  assert.match(spanish.t('pathInvitation.role.supporterEffect'), /ver.*no pueden registrar/i);
  assert.match(spanish.t('pathInvitation.supporterReadOnly'), /solo lectura/i);
});

test('warning-required copy explains visibility and guarantees the invitation remains pending', () => {
  const english = createTranslator(['en']).t('pathInvitation.warningRequired');
  const spanish = createTranslator(['es']).t('pathInvitation.warningRequired');
  assert.match(english, /more broadly.*private profile/i);
  assert.match(english, /remain pending/i);
  assert.match(spanish, /más amplia.*perfil privado/i);
  assert.match(spanish, /seguirá pendiente/i);
});

test('PATH-04 warning copy plainly describes Path-governed visibility before confirmation', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.match(english.t('pathInvitation.visibilityWarning.audience.followers'), /followers/i);
  assert.match(english.t('pathInvitation.visibilityWarning.audience.public'), /public/i);
  assert.match(
    english.t('pathInvitation.visibilityWarning.exposure'),
    /cannot open your private profile.*identity, progress, and activity.*this Path/i,
  );
  assert.match(
    english.t('pathInvitation.visibilityWarning.privacyScope'),
    /profile and unrelated Paths remain private/i,
  );
  assert.match(
    english.t('pathInvitation.visibilityWarning.retainedActivity'),
    /retained activity.*visible again/i,
  );

  assert.match(spanish.t('pathInvitation.visibilityWarning.audience.followers'), /seguidores/i);
  assert.match(spanish.t('pathInvitation.visibilityWarning.audience.public'), /públic/i);
  assert.match(
    spanish.t('pathInvitation.visibilityWarning.exposure'),
    /no pueden abrir tu perfil privado.*identidad, progreso y actividad.*esta ruta/i,
  );
  assert.match(
    spanish.t('pathInvitation.visibilityWarning.privacyScope'),
    /perfil y tus rutas no relacionadas permanecen privados/i,
  );
  assert.match(
    spanish.t('pathInvitation.visibilityWarning.retainedActivity'),
    /actividad conservada.*visible de nuevo/i,
  );
});

test('pending invitation counts use locale plural forms', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  assert.equal(english.t('pathInvitation.pendingCount', { count: 1 }), '1 pending invitation');
  assert.equal(english.t('pathInvitation.pendingCount', { count: 2 }), '2 pending invitations');
  assert.equal(spanish.t('pathInvitation.pendingCount', { count: 1 }), '1 invitación pendiente');
  assert.equal(spanish.t('pathInvitation.pendingCount', { count: 2 }), '2 invitaciones pendientes');
});

test('pending invitation context identifies the inviter and Path without private identity', () => {
  const values = {
    displayName: 'Book Owner',
    username: 'Book.Owner',
    pathName: 'Morning Reading',
  };
  for (const locale of ['en', 'es']) {
    const context = createTranslator([locale]).t('pathInvitation.pendingContext', values);
    assert.match(context, /Book Owner/);
    assert.match(context, /@Book\.Owner/);
    assert.match(context, /Morning Reading/);
    assert.doesNotMatch(context, /email|correo|profile|perfil/i);
  }
});

test('managed invitation copy identifies the public people, role, and sent time in both locales', () => {
  const recipient = {
    displayName: 'Reader One',
    username: 'reader.one',
  };
  const inviter = {
    displayName: 'Path Owner',
    username: 'owner',
  };
  const metadata = {
    date: 'Aug 8, 2026',
    role: 'Participant',
    time: '3:15 PM',
  };
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    const copy = [
      translator.t('pathInvitation.managed.recipient', recipient),
      translator.t('pathInvitation.managed.role', metadata),
      translator.t('pathInvitation.managed.inviter', inviter),
      translator.t('pathInvitation.managed.sentAt', metadata),
      translator.t('pathInvitation.managed.cancelConfirmationBody', { ...recipient, role: metadata.role }),
      translator.t('pathInvitation.managed.cancelFor', { ...recipient, role: metadata.role }),
    ].join(' ');
    assert.match(copy, /Reader One/);
    assert.match(copy, /@reader\.one/);
    assert.match(copy, /Path Owner/);
    assert.match(copy, /@owner/);
    assert.match(copy, /Participant/);
    assert.match(copy, /Aug 8, 2026/);
    assert.match(copy, /3:15 PM/);
    assert.doesNotMatch(copy, /email|correo/i);
  }
});

test('managed invitation counts use locale plural forms', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  assert.equal(english.t('pathInvitation.managed.count', { count: 1 }), '1 pending invitation');
  assert.equal(english.t('pathInvitation.managed.count', { count: 2 }), '2 pending invitations');
  assert.equal(spanish.t('pathInvitation.managed.count', { count: 1 }), '1 invitación pendiente');
  assert.equal(spanish.t('pathInvitation.managed.count', { count: 2 }), '2 invitaciones pendientes');
});
