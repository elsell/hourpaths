import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('participant removal review describes every permanent consequence in both locales', () => {
  for (const locale of ['en', 'es'] as const) {
    const translator = createTranslator([locale]);
    const warning = [
      translator.t('pathMembers.participantActivityWarning'),
      translator.t('pathMembers.participantSocialWarning'),
      translator.t('pathMembers.offlineWarning'),
      translator.t('pathMembers.runningTimerWarning'),
      translator.t('pathMembers.reinviteWarning'),
    ].join(' ');
    assert.doesNotMatch(warning, /pathMembers\./);
    assert.match(warning, locale === 'en'
      ? /activity.*progress.*statistics.*achievements.*feed.*offline.*timer.*re-invited.*restored/i
      : /actividad.*progreso.*estadísticas.*logros.*feed.*sin conexión.*temporizador.*nueva invitación.*restaurarán/i);
    assert.equal(
      translator.t('pathMembers.removeParticipantAndData'),
      locale === 'en' ? 'Remove participant and delete data' : 'Eliminar participante y borrar datos',
    );
  }
});

test('supporter removal copy is explicitly access-only', () => {
  for (const locale of ['en', 'es'] as const) {
    const translator = createTranslator([locale]);
    assert.doesNotMatch(translator.t('pathMembers.supporterWarning'), /pathMembers\./);
    assert.notEqual(translator.t('pathMembers.removeSupporter'), 'pathMembers.removeSupporter');
  }
});
