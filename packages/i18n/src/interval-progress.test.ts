import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('current interval progress has accessible uncapped English and Spanish text', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  const zero = { accumulated: 0, target: 120 };
  const completed = { accumulated: 150, target: 120 };

  assert.equal(english.t('path.progress.interval', zero), '0 of 120 seconds this interval');
  assert.equal(spanish.t('path.progress.interval', zero), '0 de 120 segundos en este intervalo');
  assert.equal(english.t('path.progress.intervalComplete', completed), '150 of 120 seconds — interval goal completed');
  assert.equal(spanish.t('path.progress.intervalComplete', completed), '150 de 120 segundos — objetivo del intervalo completado');
  assert.notEqual(english.t('path.progress.interval', completed), english.t('path.progress.intervalComplete', completed));
  assert.equal(english.t('path.progress.intervalLabel'), 'Current interval progress');
  assert.equal(spanish.t('path.progress.intervalLabel'), 'Progreso del intervalo actual');
  assert.equal(english.t('path.progress.intervalLabelForPath', { path: 'Read' }), 'Current interval progress for Read');
  assert.equal(spanish.t('path.progress.intervalLabelForPath', { path: 'Leer' }), 'Progreso del intervalo actual de Leer');
});
