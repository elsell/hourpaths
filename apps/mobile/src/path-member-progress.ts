import type { PathMemberGoalProgress } from './ui/path-member-management-view';

export function pathMemberProgressFromAPI(value: unknown): PathMemberGoalProgress | undefined {
  if (value === undefined) return undefined;
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid Path member progress');
  const progress = value as Record<string, unknown>;
  if (Object.keys(progress).sort().join(',') !== 'accumulatedSeconds,targetSeconds'
    || typeof progress.accumulatedSeconds !== 'number' || !Number.isSafeInteger(progress.accumulatedSeconds) || progress.accumulatedSeconds < 0
    || typeof progress.targetSeconds !== 'number' || !Number.isSafeInteger(progress.targetSeconds) || progress.targetSeconds <= 0) {
    throw new Error('invalid Path member progress');
  }
  return Object.freeze({ accumulatedSeconds: progress.accumulatedSeconds, targetSeconds: progress.targetSeconds });
}
