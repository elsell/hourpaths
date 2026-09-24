export type OwnershipTransferScreen = 'overview' | 'candidates' | 'review' | 'pending';

export function ownershipTransferActionState({
  busy,
  selectedRecipientID,
}: {
  busy: boolean;
  selectedRecipientID?: string;
}) {
  return {
    canConfirm: !busy && Boolean(selectedRecipientID),
    canSelect: !busy,
  };
}

export function reconcileOwnershipTransferScreen(
  screen: OwnershipTransferScreen,
  { hasPendingTransfer }: { hasPendingTransfer: boolean },
): OwnershipTransferScreen {
  if (hasPendingTransfer) return 'pending';
  return screen === 'pending' ? 'overview' : screen;
}
