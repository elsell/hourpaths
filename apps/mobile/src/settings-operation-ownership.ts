export function shouldHandleSettingsOperationFailure(
  requestToken: string,
  activeToken: string | undefined,
): boolean {
  return activeToken === requestToken;
}
