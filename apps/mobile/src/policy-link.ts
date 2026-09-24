type MobilePolicyLinkEffects = {
  open(url: string): Promise<unknown>;
  unavailable(): void;
};

function externalHTTPSURL(url: string): string {
  const destination = new URL(url);
  if (destination.protocol !== 'https:' || !destination.hostname || destination.username || destination.password) {
    throw new Error('unsafe external policy URL');
  }
  return destination.href;
}

export async function openMobilePolicyLink(
  url: string,
  effects: MobilePolicyLinkEffects,
): Promise<void> {
  try { await effects.open(externalHTTPSURL(url)); }
  catch { effects.unavailable(); }
}
