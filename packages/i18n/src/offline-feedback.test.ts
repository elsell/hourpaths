import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index';

test('archival feedback interpolates both saved and discarded durations in every locale', () => {
  for (const locale of ['en', 'es']) {
    const text = createTranslator([locale]).t('offline.archiveAmounts', { saved: '21s', discarded: '10s' });
    assert.ok(text.includes('21s') && text.includes('10s'), text);
    assert.ok(!text.includes('{'), text);
  }
});
