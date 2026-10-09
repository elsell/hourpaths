/** Screen teardown owns dismissal; a replacement may mount before cleanup runs. */
export function createNativeRouteLifetime(
  prepareRelease: (key: string) => () => void,
  defer: (work: () => void) => void = queueMicrotask,
) {
  const routes = new Map<string, { mounts: number; generation: number }>();
  return {
    mount(key: string): () => void {
      const route = routes.get(key) ?? { mounts: 0, generation: 0 };
      routes.set(key, route);
      route.mounts++;
      route.generation++;
      let mounted = true;
      return () => {
        if (!mounted) return;
        mounted = false;
        route.mounts--;
        const generation = ++route.generation;
        if (route.mounts) return;
        const release = prepareRelease(key);
        defer(() => {
          if (routes.get(key) !== route || route.mounts || route.generation !== generation) return;
          routes.delete(key);
          release();
        });
      };
    },
  };
}
