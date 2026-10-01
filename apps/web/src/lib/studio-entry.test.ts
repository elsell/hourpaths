import assert from 'node:assert/strict';
import test from 'node:test';
import { EntryController } from './studio/entry/application/entry-controller';
import { EntryFailure, type EntryReview } from './studio/entry/domain/entry';
import type { EntryService } from './studio/entry/ports/entry-service';

const review: EntryReview = { email: 'person@example.test', displayName: 'person', username: 'person', token: 'review-1', policies: { terms: { url: 'https://example.test/terms', version: '1' }, privacy: { url: 'https://example.test/privacy', version: '1' }, guidelines: { url: 'https://example.test/guidelines', version: '1' }, support: 'https://example.test/help' } };
const fields = { displayName: ' person ', username: ' person ', visibility: 'private' as const, 
    timeZone: 'UTC',
    firstDayOfWeek: 1, age: true, terms: true, privacy: true, guidelines: true };
function fixture() {
  let mode = 'onboarding' as 'onboarding' | 'recovery' | 'home' | 'entry';
  let active = true, reviews = 0, saves = 0, policyChanged = false;
  const navigation: string[] = [];
  const service: EntryService = {
    async restore() { return { kind: mode, expiresAt: mode === 'entry' ? undefined : 100 }; },
    async callback() { return service.restore(); },
    async review() { return { ...review, token: `review-${++reviews}` }; },
    defaults() { return { 
    timeZone: 'UTC',
    firstDayOfWeek: 1 }; },
    async activate(input) { saves++; assert.equal(input.username, 'person'); if (policyChanged) { policyChanged = false; throw new EntryFailure('policy'); } mode = 'home'; return service.restore(); },
    async decline() { mode = 'onboarding'; return service.restore(); },
    async begin() {}, async signOut() { active = false; mode = 'entry'; },
    current() { return active; }, expire() { active = false; mode = 'entry'; },
    navigate(destination) { navigation.push(destination); }, openPolicy() {}, dispose() { active = false; },
  };
  return { service, navigation, setMode(value: typeof mode) { mode = value; }, changePolicy() { policyChanged = true; }, counts() { return { saves, reviews }; } };
}
test('entry preserves editable onboarding across policy review and requires fresh explicit consent', async () => {
  const f = fixture(); const controller = new EntryController(f.service, () => 0, () => {});
  await controller.initialize(); assert.equal(controller.state.phase, 'onboarding');
  const old = controller.state.review!; f.changePolicy();
  await controller.activate(fields, old.token); assert.equal(controller.state.error, 'policy');
  assert.notEqual(controller.state.review!.token, old.token); assert.deepEqual(f.navigation, []);
  await controller.activate(fields, old.token); assert.equal(f.counts().saves, 1);
  await controller.activate({ ...fields, terms: false }, controller.state.review!.token); assert.equal(f.counts().saves, 1);
  await controller.activate(fields, controller.state.review!.token); assert.deepEqual(f.navigation, ['home']);
});
test('entry rejects duplicate work, expires recovery, and suppresses a disposed completion', async () => {
  const f = fixture(); f.setMode('recovery'); let now = 0;
  const controller = new EntryController(f.service, () => now, () => {});
  await controller.initialize(); now = 100; controller.maintain();
  await controller.decline(); assert.equal(controller.state.phase, 'entry'); assert.deepEqual(f.navigation, []);
  const other = fixture(); let finish!: (value: { kind: 'onboarding'; expiresAt: number }) => void; let calls = 0;
  other.service.restore = () => { calls++; return new Promise(resolve => { finish = resolve; }); };
  const pending = new EntryController(other.service, () => 0, () => {});
  const first = pending.initialize(); await pending.initialize(); assert.equal(calls, 1);
  pending.dispose(); finish({ kind: 'onboarding', expiresAt: 100 }); await first;
  assert.equal(other.counts().reviews, 0); assert.deepEqual(other.navigation, []);
});

test('provider completion follows the server destination before rendering restricted forms', async () => {
  for (const mode of ['home', 'onboarding', 'recovery'] as const) {
    const f = fixture(); f.setMode(mode);
    const controller = new EntryController(f.service, () => 0, () => {});
    await controller.initialize(true); assert.deepEqual(f.navigation, [mode]); assert.equal(f.counts().reviews, 0);
  }
});
