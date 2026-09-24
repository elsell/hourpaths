import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from '@hourpaths/i18n';
import { ownershipTransferExpirationPresentation } from './ownership-transfer-expiration';

test('ownership-transfer lifetime combines every nonzero unit without rounding', () => {
  const translator = createTranslator(['en']);
  const created = '2026-07-27T16:00:00.000Z';
  const expires = '2027-07-28T17:30:00.000Z';
  const result = ownershipTransferExpirationPresentation(created, expires, 'America/New_York', translator);
  assert.equal(result?.relative, '1 year 1 day 1 hour 30 minutes');
  assert.match(result?.summary ?? '', /1 year 1 day 1 hour 30 minutes/);
  assert.match(result?.exact ?? '', /Jul 28, 2027/);
});

test('ownership-transfer exact expiration follows the viewing time zone', () => {
  const translator = createTranslator(['en']);
  const created = '2026-07-27T16:00:00.000Z';
  const expires = '2026-07-27T17:30:00.000Z';
  const newYork = ownershipTransferExpirationPresentation(created, expires, 'America/New_York', translator);
  const losAngeles = ownershipTransferExpirationPresentation(created, expires, 'America/Los_Angeles', translator);
  assert.equal(newYork?.relative, '1 hour 30 minutes');
  assert.notEqual(newYork?.exact, losAngeles?.exact);
});

test('ownership-transfer expiration rejects malformed or sub-minute lifetimes', () => {
  const translator = createTranslator(['en']);
  assert.equal(ownershipTransferExpirationPresentation('bad', '2026-07-27T17:30:00Z', 'UTC', translator), undefined);
  assert.equal(ownershipTransferExpirationPresentation('2026-07-27T16:00:00Z', '2026-07-27T16:00:30Z', 'UTC', translator), undefined);
  assert.equal(ownershipTransferExpirationPresentation('2026-07-27T16:00:00Z', '2026-07-27T17:00:00Z', 'Not/AZone', translator), undefined);
});
