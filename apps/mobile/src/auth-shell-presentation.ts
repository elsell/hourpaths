export type AccountDestinationKind = 'home' | 'onboarding' | 'duplicate_email_recovery' | null;

export function accountShellDestination({
  destinationKind,
  pathname,
  ready,
}: {
  destinationKind: AccountDestinationKind;
  pathname: string;
  ready: boolean;
}): 'stay' | 'account-entry' | 'home-tabs' {
  if (!ready) return 'stay';
  if (destinationKind === 'home' && pathname === '/') return 'home-tabs';
  if (destinationKind !== 'home' && pathname === '/home') return 'account-entry';
  return 'stay';
}
