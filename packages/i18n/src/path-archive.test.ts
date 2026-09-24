import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('Path archive confirmation and read-only state are localized in every catalog', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  for (const key of [
    'home.archivedPaths',
    'home.pathViewLabel',
    'pathArchive.warning',
    'pathArchive.unarchiveWarning',
    'pathArchive.readOnly',
    'pathArchive.archived',
    'pathArchive.unarchived',
  ] as const) {
    assert.notEqual(english.t(key), key);
    assert.notEqual(spanish.t(key), key);
    assert.notEqual(english.t(key), spanish.t(key));
  }
  assert.match(english.t('pathArchive.warning'), /running timers.*stop/i);
  assert.match(english.t('pathArchive.unarchiveWarning'), /will not restart/i);
  assert.match(spanish.t('pathArchive.warning'), /temporizadores.*detendr/i);
});
