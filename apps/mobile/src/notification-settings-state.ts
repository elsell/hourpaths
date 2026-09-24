export function ownsNotificationSettingsState(
  stateOwnerKey: string | null,
  presentationOwnerKey: string,
) {
  return stateOwnerKey === presentationOwnerKey;
}
