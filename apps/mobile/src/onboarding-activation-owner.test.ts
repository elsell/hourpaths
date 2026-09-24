import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionOperationOwner } from '@hourpaths/client-core';
import { createOnboardingActivationOwner } from './onboarding-activation-owner';
import { activatePersistedMobileSession } from './session-exchange';

test('a cleared or replaced session cannot leave its activation blocking the next account', () => {
  const sessions = createSessionOperationOwner();
  const activations = createOnboardingActivationOwner();
  const accountA = activations.begin('account-a-token', sessions.issue());
  assert.equal(activations.blocked(), true);

  sessions.invalidate();
  activations.invalidate();
  assert.equal(accountA.ticket.current(), false);
  assert.equal(activations.blocked(), false);

  const accountB = activations.begin('account-b-token', sessions.issue());
  assert.equal(activations.blocked(), true);
  assert.equal(activations.owns(accountA), false, 'A late completion is superseded');
  assert.equal(activations.owns(accountB), true);

  activations.release(accountA);
  assert.equal(activations.owns(accountB), true, 'A late cleanup cannot release B');
  activations.release(accountB);
  assert.equal(activations.blocked(), false);
});

test('the activating credential keeps onboarding busy and immutable until Home publishes', async () => {
  const sessions = createSessionOperationOwner();
  const activations = createOnboardingActivationOwner();
  const ticket = sessions.issue();
  const attempt = activations.begin('onboarding-token', ticket);
  let publishHome!: (profile: { id: string }) => void;
  let homePublished = false;
  let appliedEdits = 0;
  const editDraft = () => {
    if (activations.blocked()) return;
    appliedEdits += 1;
  };
  const pendingHome = new Promise<{ id: string }>((resolve) => { publishHome = resolve; });

  const activation = activatePersistedMobileSession({
    credential: {
      token: 'active-token',
      expiresAt: '2030-01-01T00:00:00Z',
      nextAction: 'home' as const,
    },
    renewable: true,
    current: ticket.current,
    adopt: () => {
      if (!activations.ownedBy(ticket)) activations.invalidate();
    },
    loadHome: () => pendingHome,
    loadOnboarding: async () => ({ id: 'must-not-load' }),
    online: () => { homePublished = true; },
  });

  await Promise.resolve();
  assert.equal(homePublished, false);
  assert.equal(activations.blocked(), true, 'busy remains visible during Home loading');
  editDraft();
  assert.equal(appliedEdits, 0, 'draft handlers remain locked during Home loading');

  publishHome({ id: 'account-a' });
  await activation;
  assert.equal(homePublished, true);
  assert.equal(activations.blocked(), true, 'publication precedes release by the submit owner');
  activations.release(attempt);
  assert.equal(activations.blocked(), false);
  editDraft();
  assert.equal(appliedEdits, 1);
});
