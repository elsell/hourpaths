import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import { boundedAccessibilityProgress } from './ui/progress-indicator-values';
import { mobileTheme } from './ui/tokens';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const progressIndicator = readFileSync(fileURLToPath(new URL('./ui/progress-indicator.tsx', import.meta.url)), 'utf8');
const pathDetailView = readFileSync(fileURLToPath(new URL('./ui/path-detail-view.tsx', import.meta.url)), 'utf8');

test('native progress values stay within their declared accessibility range', () => {
  assert.deepEqual(boundedAccessibilityProgress(60, 45), { max: 60, now: 45 });
  assert.deepEqual(boundedAccessibilityProgress(60, 90), { max: 60, now: 60 });
  assert.deepEqual(boundedAccessibilityProgress(60, -15), { max: 60, now: 0 });
});

function relativeLuminance(hex: string): number {
  const channels = [1, 3, 5].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255)
    .map((channel) => channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4);

  return (0.2126 * channels[0]!) + (0.7152 * channels[1]!) + (0.0722 * channels[2]!);
}

function contrastRatio(foreground: string, background: string): number {
  const foregroundLuminance = relativeLuminance(foreground);
  const backgroundLuminance = relativeLuminance(background);

  return (Math.max(foregroundLuminance, backgroundLuminance) + 0.05)
    / (Math.min(foregroundLuminance, backgroundLuminance) + 0.05);
}

test('mobile Home and Path details show authoritative accumulated and overall progress', () => {
  assert.match(page, /overallProgress/);
  assert.match(pathDetailView, /pathDetails\.totalTime/);
  assert.match(page, /path\.progress\.overallDetail(?:Complete)?/);
  assert.match(page, /formatCompactDuration\(progress\.accumulatedSeconds, i18n\)/);
  assert.match(page, /formatCompactDuration\(progress\.targetSeconds, i18n\)/);
  assert.match(page, /i18n\.t\('path\.progress\.overallLabel'\)/);
  assert.match(page, /targetValue=\{progress\.targetSeconds\}/);
  assert.match(page, /visualValue=\{progress\.visualSeconds\}/);
  assert.match(progressIndicator, /accessibilityRole="progressbar"/);
  assert.match(progressIndicator, /boundedAccessibilityProgress\(targetValue, visualValue\)/);
  assert.match(progressIndicator, /accessibilityValue=\{\{\s*min:\s*0,\s*\.\.\.accessibilityProgress,\s*text\s*,?\s*\}\}/);
  assert.match(page, /state && path\.overallTarget\s*\? overallProgress\(state\.accumulatedSeconds, path\.overallTarget\)/);
  assert.match(page, /selectedPath\?\.overallTarget && selectedTimerState[\s\S]*overallProgress\(selectedTimerState\.accumulatedSeconds, selectedPath\.overallTarget\)/);
});

test('mobile goal progress remains conditional so absent goals do not render false indicators', () => {
  assert.match(page, /\{progress \? <OverallProgressIndicator compact progress=\{progress\} pathName=\{path\.name\} \/> : null\}/);
  assert.match(page, /overallProgress=\{selectedOverallProgress \? <OverallProgressIndicator progress=\{selectedOverallProgress\} \/> : undefined\}/);
  assert.match(page, /\{currentIntervalProgress \? <IntervalProgressIndicator compact/);
  assert.match(page, /const elapsedText = state\?\.running[\s\S]*formatCompactDuration\(activeTimerSeconds\(state\.timer\?\.startedAt, now\), i18n\)/);
  assert.match(page, /elapsedAccessibilityLabel=\{elapsedText \? i18n\.t\('timer\.elapsedValue', \{ duration: elapsedText \}\) : undefined\}/);
  assert.match(page, /activeTimerSeconds\(state\.timer\?\.startedAt, now\)/);
});

test('shared mobile progress presentation keeps visible and semantic values together', () => {
  assert.match(page, /import \{ ProgressIndicator \} from '\.\.\/src\/ui\/progress-indicator';/);
  assert.match(page, /pathName\s*\?\s*i18n\.t\('path\.progress\.overallLabelForPath',\s*\{\s*path:\s*pathName\s*\}\)\s*:\s*i18n\.t\('path\.progress\.overallLabel'\)/);
  assert.match(page, /function OverallProgressIndicator[\s\S]*?<ProgressIndicator[\s\S]*?targetValue=\{progress\.targetSeconds\}[\s\S]*?visualValue=\{progress\.visualSeconds\}/);
  assert.match(progressIndicator, /accessibilityRole="progressbar"/);
  assert.match(progressIndicator, /accessibilityLabel=\{accessibilityLabel\}/);
  assert.match(progressIndicator, /accessibilityValue=\{\{\s*min:\s*0,\s*\.\.\.accessibilityProgress,\s*text/);
  assert.match(progressIndicator, /<Text[^>]*>\{text\}<\/Text>/);
  assert.doesNotMatch(progressIndicator, /allowFontScaling=\{false\}/);
  assert.match(progressIndicator, /container:\s*\{[\s\S]*?backgroundColor:\s*mobileTheme\.colors\.surfaceRaised/);
  assert.match(progressIndicator, /fill:\s*\{[\s\S]*?backgroundColor:\s*mobileTheme\.colors\.accent/);
  assert.match(progressIndicator, /label:\s*\{[\s\S]*?color:\s*mobileTheme\.colors\.text/);
  assert.match(progressIndicator, /track:\s*\{[\s\S]*?backgroundColor:\s*mobileTheme\.colors\.progressTrack/);
});

test('charcoal and yellow mobile tokens meet applicable contrast and target baselines', () => {
  assert.ok(contrastRatio(mobileTheme.colors.text, mobileTheme.colors.background) >= 4.5);
  assert.ok(contrastRatio(mobileTheme.colors.textMuted, mobileTheme.colors.background) >= 4.5);
  assert.ok(contrastRatio(mobileTheme.colors.accent, mobileTheme.colors.background) >= 3);
  assert.ok(contrastRatio(mobileTheme.colors.accent, mobileTheme.colors.progressTrack) >= 3);
  assert.ok(contrastRatio(mobileTheme.colors.progressTrack, mobileTheme.colors.background) >= 3);
  assert.ok(contrastRatio(mobileTheme.colors.progressTrack, mobileTheme.colors.surface) >= 3);
  assert.ok(contrastRatio(mobileTheme.colors.progressTrack, mobileTheme.colors.surfaceRaised) >= 3);
  assert.ok(mobileTheme.sizes.minimumTouchTarget >= 48);
  assert.equal(mobileTheme.typography.body.fontSize, 17);
});
