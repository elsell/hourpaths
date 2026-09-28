type SessionDeadlineClock = {
  now(): number;
  schedule(callback: () => void, delay: number): () => void;
};

const systemClock: SessionDeadlineClock = {
  now: Date.now,
  schedule(callback, delay) {
    const timer = setTimeout(callback, delay);
    return () => clearTimeout(timer);
  },
};

// Native and browser timers use signed 32-bit delays. Recheck the absolute
// deadline in bounded chunks, including after a suspended app resumes.
export function scheduleSessionDeadline(
  callback: () => void,
  deadline: number,
  clock: SessionDeadlineClock = systemClock,
): () => void {
  let cancelled = false;
  let cancelTimer: (() => void) | undefined;
  const schedule = () => {
    const remaining = Math.max(0, deadline - clock.now());
    cancelTimer = clock.schedule(() => {
      if (cancelled) return;
      if (clock.now() < deadline) schedule();
      else callback();
    }, Math.min(remaining, 24 * 60 * 60 * 1000));
  };
  schedule();
  return () => { cancelled = true; cancelTimer?.(); };
}
