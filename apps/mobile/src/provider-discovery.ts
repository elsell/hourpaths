export async function loadProviderDiscovery<Discovery>(
  load: () => Promise<Discovery>,
): Promise<Discovery | null> {
  try {
    return await load();
  } catch {
    return null;
  }
}
