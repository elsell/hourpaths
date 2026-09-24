import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

const keys = [
  'pathDetails.genericTitle',
  'pathDetails.backToPath',
  'pathDetails.backToHistory',
  'pathDetails.unavailableTitle',
  'pathDetails.unavailableDescription',
  'activity.discardTitle',
  'activity.discardDescription',
  'activity.discard',
  'activity.keepEditing',
] as const;

test('Path route title, back chain, and opaque recovery copy exist in every locale', () => {
  for (const locale of ['en', 'es']) {
    const i18n = createTranslator([locale]);
    for (const key of keys) assert.notEqual(i18n.t(key), key, `${locale}: ${key}`);
    assert.doesNotMatch(i18n.t('pathDetails.unavailableDescription'), /404|forbidden|private|deleted/i);
  }
});
