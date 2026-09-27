import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import test from 'node:test';
import { homeIntervalProgress } from './ui/home-path-presentation';

const pathCard = readFileSync(fileURLToPath(new URL('./ui/path-card.tsx', import.meta.url)), 'utf8');
const timerControl = readFileSync(fileURLToPath(new URL('./ui/timer-control.tsx', import.meta.url)), 'utf8');
const nativeTrackingButton = readFileSync(fileURLToPath(new URL('./ui/native-tracking-button.ios.tsx', import.meta.url)), 'utf8');

test('Home interval progress remains authoritative while a timer is still unrecorded', () => {
  assert.deepEqual(homeIntervalProgress(
    { accumulatedSeconds: 40, targetSeconds: 120 },
  ), {
    accumulatedSeconds: 40,
    completed: false,
    targetSeconds: 120,
    visualSeconds: 40,
  });
});

test('Home interval progress remains absent when the API supplies no interval projection', () => {
  assert.equal(homeIntervalProgress(
    undefined,
  ), undefined);
});
