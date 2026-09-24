import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('Path leave copy distinguishes retained activity from permanent deletion in both locales', () => {
  for (const locale of ['en', 'es'] as const) {
    const translator = createTranslator([locale]);
    const warning = translator.t('pathLeave.warning');
    assert.notEqual(warning, 'pathLeave.warning');
    assert.match(warning, locale === 'en' ? /immediately.*retained.*hidden.*rejoin/i : /inmediatamente.*conservará.*oculta.*vuelvas a unir/i);
    assert.notEqual(translator.t('pathLeave.action'), 'pathLeave.action');
    assert.match(translator.t('pathLeave.deleteWarning'), locale === 'en' ? /only your.*permanent.*rejoin/i : /solo tu.*permanente.*volverá.*vuelves a unirte/i);
    assert.notEqual(translator.t('pathLeave.keepActivity'), 'pathLeave.keepActivity');
    assert.notEqual(translator.t('pathLeave.deleteActivity'), 'pathLeave.deleteActivity');
    assert.doesNotMatch(translator.t('pathLeave.supporterWarning'), /activity|actividad/i);
    assert.notEqual(translator.t('pathLeave.completedRetained', { pathName: 'Piano' }), 'pathLeave.completedRetained');
    assert.notEqual(translator.t('pathLeave.completedDeleted', { pathName: 'Piano' }), 'pathLeave.completedDeleted');
  }
});
