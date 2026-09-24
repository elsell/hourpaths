import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator, type MessageKey } from './index';

const keys = [
  'pathOwnership.heading',
  'pathOwnership.explanation',
  'pathOwnership.selectRecipient',
  'pathOwnership.administrator',
  'pathOwnership.reviewHeading',
  'pathOwnership.recipientBecomesCreator',
  'pathOwnership.creatorBecomesAdministrator',
  'pathOwnership.noChangeUntilAccepted',
  'pathOwnership.expiration',
  'pathOwnership.exactExpiration',
  'pathOwnership.confirm',
  'pathOwnership.pendingHeading',
  'pathOwnership.accept',
  'pathOwnership.decline',
  'pathOwnership.cancel',
  'pathOwnership.unavailable',
  'pathOwnership.reviewRecipient',
  'pathOwnership.recipientRole',
  'pathOwnership.yourRole',
  'pathOwnership.creator',
  'pathOwnership.loadingCandidates',
  'pathOwnership.loadingMore',
  'pathOwnership.candidatesUnavailable',
  'pathOwnership.candidatesEmpty',
  'pathOwnership.candidatesEmptyDescription',
  'pathOwnership.candidateFooter',
  'pathOwnership.candidateAccessibility',
  'pathOwnership.pendingWith',
  'pathOwnership.expires',
  'pathOwnership.creatorPendingExplanation',
  'pathOwnership.recipientPendingExplanation',
  'pathOwnership.sending',
  'pathOwnership.accepting',
  'pathOwnership.declining',
  'pathOwnership.canceling',
] as const satisfies readonly MessageKey[];

test('PATH-06 ownership-transfer copy is complete in English and Spanish', () => {
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of keys) assert.notEqual(translator.t(key), key);
    assert.match(translator.t('pathOwnership.expiration', {
      exact: 'July 30, 2026 at 4:30 PM',
      relative: '3 days',
    }), /3 days/);
  }
});

test('confirmation states both role changes and delayed effect', () => {
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    assert.notEqual(translator.t('pathOwnership.recipientBecomesCreator'), translator.t('pathOwnership.creatorBecomesAdministrator'));
    assert.match(translator.t('pathOwnership.noChangeUntilAccepted'), /accept|acept/i);
  }
});
