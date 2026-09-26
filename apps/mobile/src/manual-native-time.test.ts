import assert from 'node:assert/strict';
import test from 'node:test';
import { androidClockPickerValue, localTimeFromAndroidClock } from './ui/manual-occurrence-values';

test('Android clock opens the participant wall time in the device calendar without an offset conversion', () => {
  const value = androidClockPickerValue('01:30:45');
  assert.ok(value);
  assert.equal(value.getHours(), 1);
  assert.equal(value.getMinutes(), 30);
  assert.equal(value.getSeconds(), 45);
  assert.equal(localTimeFromAndroidClock(new Date(2000, 0, 15, 23, 59, 7)), '23:59:00');
});

test('malformed wall time remains invalid before opening the Android clock', () => {
  for (const time of ['', '25:30', '09:61', '00:30:99', 'not-a-time']) {
    assert.equal(androidClockPickerValue(time), null);
  }
});
