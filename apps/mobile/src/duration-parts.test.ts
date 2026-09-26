import assert from 'node:assert/strict';
import test from 'node:test';
import { durationParts, secondsFromDurationParts } from './ui/duration-parts';

test('editing hours and minutes preserves the recorded seconds', () => {
  const parts = durationParts('14021');
  assert.deepEqual(parts, { hours: '3',
    minutes: '53',
    seconds: '41' });
  assert.equal(secondsFromDurationParts(parts), '14021');
  assert.equal(secondsFromDurationParts({ ...parts,
    minutes: '54' }), '14081');
  assert.deepEqual(durationParts('5400'), { hours: '1',
    minutes: '30',
    seconds: '' });
});

test('blank, invalid, zero, and overflowing durations are not silently saved as a different value', () => {
  assert.equal(secondsFromDurationParts(durationParts('')), '');
  assert.equal(secondsFromDurationParts({ hours: '',
    minutes: '90',
    seconds: '' }), '5400');
  assert.equal(secondsFromDurationParts({ hours: '0',
    minutes: '',
    seconds: '' }), '0');
  for (const value of ['-1', '1.5', 'abc', '9007199254740992']) {
    assert.equal(secondsFromDurationParts({ hours: value,
    minutes: '5',
    seconds: '' }), 'invalid');
  }
});
