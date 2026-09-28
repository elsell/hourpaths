export type AccountDestinationKind = 'home' | 'onboarding' | 'duplicate_email_recovery' | null;

export function accountShellDestination({
  destinationKind,
  sessionNextAction = null,
  pathname,
  ready,
}: {
  destinationKind: AccountDestinationKind;
  sessionNextAction?: AccountDestinationKind;
  pathname: string;
  ready: boolean;
}): 'stay' | 'account-entry' | 'home-tabs' {
  if (!ready) return 'stay';
  if (destinationKind === 'home' && pathname === '/') return 'home-tabs';
  if (destinationKind !== 'home' && sessionNextAction !== 'home' && pathname !== '/') return 'account-entry';
  return 'stay';
}
