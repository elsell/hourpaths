import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('the temporary active Home section has concise localized copy', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.equal(english.t('home.activeTimersHeading'), 'Active');
  assert.equal(spanish.t('home.activeTimersHeading'), 'En curso');
});
