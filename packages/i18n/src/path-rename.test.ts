import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('Path rename has concise accessible actions and status in both locales', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.equal(english.t('pathRename.heading'), 'Rename Path');
  assert.equal(english.t('pathRename.nameLabel'), 'Path name');
  assert.equal(english.t('pathRename.save'), 'Save name');
  assert.equal(english.t('pathRename.saved', { name: 'Reading' }), 'Path renamed to Reading.');
  assert.equal(spanish.t('pathRename.heading'), 'Cambiar nombre de la ruta');
  assert.equal(spanish.t('pathRename.nameLabel'), 'Nombre de la ruta');
  assert.equal(spanish.t('pathRename.save'), 'Guardar nombre');
  assert.equal(spanish.t('pathRename.saved', { name: 'Lectura' }), 'La ruta ahora se llama Lectura.');
  assert.notEqual(english.t('pathRename.save'), english.t('common.cancel'));
  assert.notEqual(spanish.t('pathRename.save'), spanish.t('common.cancel'));
});
