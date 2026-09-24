import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

for (const locale of ['en', 'es'] as const) {
  test(`${locale} has complete Path member role-change copy`, () => {
    const i18n = createTranslator([locale]);
    for (const key of [
      'pathMembers.changeRole',
      'pathMembers.roleParticipantEffect',
      'pathMembers.roleSupporterEffect',
      'pathMembers.roleAdministratorEffect',
      'pathMembers.roleChangeWarning',
      'pathMembers.roleChangeTimerWarning',
      'pathMembers.confirmSupporter',
      'pathMembers.confirmGrantAdministrator',
      'pathMembers.confirmRevokeAdministrator',
      'pathMembers.confirmStepDownAdministrator',
      'pathMembers.grantAdministratorWarning',
      'pathMembers.revokeAdministratorWarning',
      'pathMembers.stepDownAdministratorWarning',
      'pathMembers.changingRole',
      'pathMembers.roleChangeUnavailable',
      'pathMembers.roleChangedParticipant',
      'pathMembers.roleChangedSupporter',
      'pathMembers.roleChangedAdministrator',
      'notification.pathMemberRoleChanged.participant',
      'notification.pathMemberRoleChanged.supporter',
      'notification.pathMemberRoleChanged.administrator',
      'notification.pathMemberRemoved.participant',
      'notification.pathMemberRemoved.supporter',
    ] as const) assert.notEqual(i18n.t(key, { displayName: 'Avery', pathName: 'Piano' }), key);
  });
}

test('destructive role copy names the person and permanent data loss', () => {
  const i18n = createTranslator(['en']);
  assert.match(i18n.t('pathMembers.roleChangeWarning', { displayName: 'Avery' }), /Avery/);
  assert.match(i18n.t('pathMembers.roleChangeWarning', { displayName: 'Avery' }), /permanently deleted/i);
  assert.match(i18n.t('pathMembers.confirmSupporter'), /supporter/i);
});

test('administrator lifecycle copy states authority changes and requires explicit actions', () => {
  const i18n = createTranslator(['en']);
  assert.match(i18n.t('pathMembers.grantAdministratorWarning', { displayName: 'Avery' }), /Avery/);
  assert.match(i18n.t('pathMembers.grantAdministratorWarning', { displayName: 'Avery' }), /manage/i);
  assert.match(i18n.t('pathMembers.revokeAdministratorWarning', { displayName: 'Avery' }), /remain a participant/i);
  assert.match(i18n.t('pathMembers.stepDownAdministratorWarning'), /remain a participant/i);
  assert.match(i18n.t('notification.pathMemberRoleChanged.participant', {
    displayName: 'Avery', pathName: 'Piano',
  }), /cannot manage the Path or its members/i);
});
