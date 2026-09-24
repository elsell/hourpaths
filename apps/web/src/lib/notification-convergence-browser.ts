export type NotificationConvergenceBrowser = Readonly<{
  publish(signal: unknown): void;
  close(): void;
}>;

type Listener = () => void;

export type NotificationConvergenceBrowserEnvironment = Readonly<{
  addFocus(listener: Listener): void;
  removeFocus(listener: Listener): void;
  addVisibility(listener: Listener): void;
  removeVisibility(listener: Listener): void;
  visible(): boolean;
  openChannel(receive: (message: unknown) => void): Readonly<{
    publish(signal: unknown): void;
    close(): void;
  }>;
}>;

const browserEnvironment: NotificationConvergenceBrowserEnvironment = {
  addFocus(listener) { window.addEventListener('focus', listener); },
  removeFocus(listener) { window.removeEventListener('focus', listener); },
  addVisibility(listener) { document.addEventListener('visibilitychange', listener); },
  removeVisibility(listener) { document.removeEventListener('visibilitychange', listener); },
  visible() { return document.visibilityState === 'visible'; },
  openChannel(receive) {
    const channel = new BroadcastChannel('hourpaths:notification-history');
    const listener = (event: MessageEvent<unknown>) => receive(event.data);
    try { channel.addEventListener('message', listener); }
    catch (cause) {
      try { channel.close(); }
      finally { throw cause; }
    }
    return {
      publish(signal) { channel.postMessage(signal); },
      close() {
        try { channel.removeEventListener('message', listener); }
        finally { channel.close(); }
      },
    };
  },
};

export function openNotificationConvergenceBrowser(
  receive: (message: unknown) => void,
  resume: () => void,
  environment: NotificationConvergenceBrowserEnvironment = browserEnvironment,
): NotificationConvergenceBrowser {
  let active = true;
  let focusInstalled = false;
  let visibilityInstalled = false;
  let channel: ReturnType<NotificationConvergenceBrowserEnvironment['openChannel']> | null = null;
  const handleFocus = () => { if (active) resume(); };
  const handleVisibility = () => { if (active && environment.visible()) resume(); };

  try { environment.addFocus(handleFocus); focusInstalled = true; } catch {}
  try { environment.addVisibility(handleVisibility); visibilityInstalled = true; } catch {}
  try { channel = environment.openChannel((message) => { if (active) receive(message); }); } catch {}

  return Object.freeze({
    publish(signal) {
      try { channel?.publish(signal); } catch {}
    },
    close() {
      active = false;
      if (focusInstalled) try { environment.removeFocus(handleFocus); } catch {}
      if (visibilityInstalled) try { environment.removeVisibility(handleVisibility); } catch {}
      try { channel?.close(); } catch {}
      channel = null;
    },
  });
}
