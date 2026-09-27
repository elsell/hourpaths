import assert from 'node:assert/strict';
import test from 'node:test';
import { validPathEmoji } from './path-appearance-validation';

test('appearance accepts compound keyboard emoji while rejecting empty, prose, or multiple emoji', () => {
  for (const value of ['📚', '🎹', '🧑🏽‍💻', '👨‍👩‍👧‍👦', '🇪🇸', '1️⃣', '❤️']) assert.equal(validPathEmoji(value), true, value);
  for (const value of ['', 'Reading', '📚🎹', 'hello 📚', '\u200D', '🇪', '📚\n']) assert.equal(validPathEmoji(value), false, value);
});
