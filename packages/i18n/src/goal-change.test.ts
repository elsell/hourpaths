import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('goal-change confirmation distinguishes reprojected progress from unchanged activity in both locales', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.equal(
    english.t('pathManage.goalWarning'),
    'This change recalculates current and historical progress for every participant. Recorded activity will not change.',
  );
  assert.equal(
    spanish.t('pathManage.goalWarning'),
    'Este cambio recalcula el progreso actual e histórico de cada participante. La actividad registrada no cambiará.',
  );
  assert.equal(english.t('pathManage.current'), 'Current goals');
  assert.equal(english.t('pathManage.proposed'), 'Proposed goals');
  assert.equal(spanish.t('pathManage.current'), 'Objetivos actuales');
  assert.equal(spanish.t('pathManage.proposed'), 'Objetivos propuestos');
  assert.notEqual(english.t('pathManage.confirm'), english.t('common.cancel'));
  assert.notEqual(spanish.t('pathManage.confirm'), spanish.t('common.cancel'));
});
