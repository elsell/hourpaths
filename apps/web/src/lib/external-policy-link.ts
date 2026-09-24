type BrowserPolicyLinkEffects = {
  open(url: string): void;
};

const browserPolicyLinkEffects: BrowserPolicyLinkEffects = {
  open: (url) => {
    const destination = window.open('', '_blank');
    if (!destination) throw new Error('policy window unavailable');
    destination.opener = null;
    destination.location.replace(url);
  },
};

function externalHTTPSURL(url: string): string {
  const destination = new URL(url);
  if (destination.protocol !== 'https:' || !destination.hostname || destination.username || destination.password) {
    throw new Error('unsafe external policy URL');
  }
  return destination.href;
}

export function openWebPolicyLink(
  url: string,
  unavailable: () => void,
  effects: BrowserPolicyLinkEffects = browserPolicyLinkEffects,
): void {
  try { effects.open(externalHTTPSURL(url)); }
  catch { unavailable(); }
}
