import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const action = readFileSync(
  fileURLToPath(new URL('./ui/native-sheet-action.ios.tsx', import.meta.url)),
  'utf8',
);

test('iOS sheet actions retain their proposed header width while measuring native height', () => {
  assert.match(action, /matchContents=\{\{ vertical: true \}\}/);
  assert.match(action, /host:\s*\{[\s\S]*width:\s*'100%'/);
  assert.match(action, /<Text[\s\S]*>\{label\}<\/Text>/);
  assert.doesNotMatch(action, /<NativePrimaryButton/);
});
