export function ownsSettingsRouteState(stateOwnerKey: string | null, currentOwnerKey: string): boolean {
  return stateOwnerKey === currentOwnerKey;
}

export type SettingsRouteOwnerDecision = 'claim' | 'retain' | 'replacement' | 'unowned';

export function settingsRouteOwnerDecision(
  ownerKey: string | undefined,
  incomingKey: string | undefined,
): SettingsRouteOwnerDecision {
  if (!incomingKey) return ownerKey ? 'retain' : 'unowned';
  if (!ownerKey) return 'claim';
  return ownerKey === incomingKey ? 'retain' : 'replacement';
}
