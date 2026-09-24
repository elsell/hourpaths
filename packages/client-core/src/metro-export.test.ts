import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

test('workspace source exports remain resolvable by Metro', () => {
  const entry = readFileSync(new URL('./index.ts', import.meta.url), 'utf8');
  assert.doesNotMatch(entry, /from ['"]\.\/[^'"]+\.js['"]/);
});
