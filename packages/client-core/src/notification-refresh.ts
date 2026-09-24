export type NotificationRefreshLatch = Readonly<{
  request(ownerId: string): Promise<void>;
  dispose(): void;
}>;

export function createNotificationRefreshLatch(
  refresh: (ownerId: string) => Promise<void>,
): NotificationRefreshLatch {
  let disposed = false;
  let inFlight: Promise<void> | null = null;
  let pendingOwnerId = '';

  const drain = async (initialOwnerId: string) => {
    let ownerId = initialOwnerId;
    while (!disposed && ownerId) {
      pendingOwnerId = '';
      await refresh(ownerId);
      ownerId = pendingOwnerId;
    }
  };

  return Object.freeze({
    request(ownerId: string) {
      if (disposed || !ownerId) return Promise.resolve();
      if (inFlight) {
        pendingOwnerId = ownerId;
        return inFlight;
      }
      inFlight = drain(ownerId).finally(() => { inFlight = null; });
      return inFlight;
    },
    dispose() {
      disposed = true;
      pendingOwnerId = '';
    },
  });
}
