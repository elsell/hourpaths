import assert from 'node:assert/strict';
import test from 'node:test';
import {
  ownershipTransferActionState,
  reconcileOwnershipTransferScreen,
} from './ownership-transfer-presentation';

test('ownership transfer cannot advance without a candidate and locks every action while busy', () => {
  assert.deepEqual(ownershipTransferActionState({ busy: false }), {
    canConfirm: false,
    canSelect: true,
  });
  assert.deepEqual(ownershipTransferActionState({ busy: false, selectedRecipientID: 'user-2' }), {
    canConfirm: true,
    canSelect: true,
  });
  assert.deepEqual(ownershipTransferActionState({ busy: true, selectedRecipientID: 'user-2' }), {
    canConfirm: false,
    canSelect: false,
  });
});

test('ownership transfer returns to its stable destination when data changes', () => {
  assert.equal(reconcileOwnershipTransferScreen('review', { hasPendingTransfer: true }), 'pending');
  assert.equal(reconcileOwnershipTransferScreen('candidates', { hasPendingTransfer: false }), 'candidates');
  assert.equal(reconcileOwnershipTransferScreen('pending', { hasPendingTransfer: false }), 'overview');
});
