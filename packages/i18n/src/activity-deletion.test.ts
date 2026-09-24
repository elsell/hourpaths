import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('activity deletion confirmation, failure, and success copy is complete in every locale', () => {
  for (const locale of ['en', 'es'] as const) {
    const i18n = createTranslator([locale]);
    for (const key of [
      'pathDetails.deleteConfirmationHeading', 'pathDetails.deleteConfirmation',
      'pathDetails.deleteRetryableError', 'pathDetails.deleteFailed', 'pathDetails.deleted',
    ] as const) assert.notEqual(i18n.t(key), key);
    assert.match(i18n.t('pathDetails.deleteConfirmation'), locale === 'en'
      ? /feed post.*comments.*reactions.*notifications.*can.t be undone/i
      : /publicación.*comentarios.*reacciones.*notificaciones.*no se puede deshacer/i);
    assert.match(i18n.t('pathDetails.deleteRetryableError'), locale === 'en' ? /wasn.t deleted/i : /no se eliminó/i);
    assert.match(i18n.t('pathDetails.deleteFailed'), locale === 'en' ? /nothing changed/i : /no cambió nada/i);
  }
});
