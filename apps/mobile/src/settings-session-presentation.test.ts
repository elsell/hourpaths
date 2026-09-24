import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import test from 'node:test';

const presentation = readFileSync(
  fileURLToPath(new URL('./ui/settings-presentation.tsx', import.meta.url)),
  'utf8',
);
const shell = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('settings presentation is account-generation keyed and invalidates delayed reviewed actions', () => {
  assert.match(presentation, /sessionKey: string/);
  assert.match(presentation, /isCurrent: \(\) => boolean/);
  assert.match(presentation, /let active = true/);
  assert.match(presentation, /if \(!active \|\| !isCurrentRef\.current\(\)\) throw new Error\('settings_presentation_superseded'\)/);
  assert.match(presentation, /active = false/);
  assert.match(presentation, /\[displayName, email, runningTimerCount, sessionKey\]/);
  assert.match(shell, /<SettingsPresentationSource[\s\S]*sessionKey=\{socialPresentationKey\}/);
  assert.match(shell, /<SettingsPresentationSource[\s\S]*isCurrent=\{\(\) => socialPresentationKey ===/);
});
