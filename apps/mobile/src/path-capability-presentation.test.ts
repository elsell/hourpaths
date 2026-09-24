import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('mobile controls and handlers use effective server-authoritative capabilities', () => {
  assert.match(page, /const capabilities = effectivePathCapabilities\(path\);[\s\S]*capabilities\.manageGoals \|\| capabilities\.renamePath \|\| capabilities\.manageLifecycle/);
  assert.match(page, /effectivePathCapabilities\(path\)\.trackTime/);
  assert.match(page, /selectedCapabilities\.manageGoals/);
  assert.match(page, /selectedCapabilities\.manageLifecycle/);
  assert.match(page, /selectedCapabilities\.renamePath/);
  assert.match(page, /selectedCapabilities\.trackTime/);
  assert.match(page, /pathCapabilities\.trackTime && state/);
});
