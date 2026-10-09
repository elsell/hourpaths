import type { CreateTimerSurface } from './timer-surface-types';

// Unsupported platforms retain normal in-app tracking.
export const createTimerSurface: CreateTimerSurface = () => ({
  clear: async () => undefined,
  replace: async () => undefined,
});
