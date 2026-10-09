import assert from 'node:assert/strict';
import test from 'node:test';
import { timeZoneChoices } from './time-zone-choices';

test('picker offers geographic zones without an Intl enumeration API', () => {
  const choices = timeZoneChoices('America/New_York');
  for (const zone of ['Europe/London', 'America/Chicago', 'Asia/Kolkata', 'Pacific/Chatham']) assert.ok(choices.includes(zone), zone);
  assert.ok(choices.length > 300);
  assert.equal(new Set(choices).size, choices.length);
});
test('picker preserves supported saved aliases and excludes runtime-unsupported zones', () => {
  const supported = new Set(['US/Eastern', 'Europe/London']);
  assert.deepEqual(timeZoneChoices('US/Eastern', zone => supported.has(zone)), ['Europe/London', 'US/Eastern']);
  assert.deepEqual(timeZoneChoices('invalid-zone', zone => supported.has(zone)), ['Europe/London']);
});
