import assert from 'node:assert/strict';
import test from 'node:test';
import { withNativeChildRouteDismissalAllowed } from './ui/native-child-route-presentation';

test('unlocking a pending native child route publishes a new snapshot without losing ownership or dismissal', () => {
  const owner = {};
  const dismiss = () => undefined;
  const pending = { dismiss, dismissible: false, owner };

  const unlocked = withNativeChildRouteDismissalAllowed(pending);

  assert.notStrictEqual(unlocked, pending);
  assert.equal(unlocked.dismissible, true);
  assert.strictEqual(unlocked.owner, owner);
  assert.strictEqual(unlocked.dismiss, dismiss);
  assert.equal(pending.dismissible, false);
});
