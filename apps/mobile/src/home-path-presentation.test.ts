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

test('Home Path rows remove decorative markers and collapse around the circular timer', () => {
  assert.doesNotMatch(pathCard, /styles\.marker|markerFrame/);
  assert.doesNotMatch(pathCard, /pathTrackingRow/);
  assert.match(pathCard, /minHeight:\s*mobileTheme\.sizes\.minimumTouchTarget/);
  assert.match(pathCard, /tracking \? styles\.trackingRow/);
});

test('the running timer keeps elapsed duration readable outside the circular Stop control', () => {
  assert.match(timerControl, /running && elapsedText \? <Text[\s\S]*accessibilityLabel=\{elapsedAccessibilityLabel\}[\s\S]*\{elapsedText\}<\/Text>/);
  assert.match(timerControl, /elapsed:\s*\{[\s\S]*fontVariant:\s*\['tabular-nums'\][\s\S]*textAlign:\s*'center'/);
  assert.doesNotMatch(timerControl, /styles\.liveDot/);
  assert.doesNotMatch(nativeTrackingButton, /elapsedText|Animated\.Text|styles\.elapsed/);
  assert.match(nativeTrackingButton, /Animated\.loop/);
  assert.match(nativeTrackingButton, /rotate/);
  assert.match(nativeTrackingButton, /if \(!running \|\| reduceMotion !== false\)/);
  assert.doesNotMatch(nativeTrackingButton, /opacity:\s*pulse\.interpolate/);
});
