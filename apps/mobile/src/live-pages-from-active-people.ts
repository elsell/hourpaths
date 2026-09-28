import type { Translator } from '@hourpaths/i18n';
import type { ActiveFollowingItem } from './ui/social-active-following-presentation';
import type { LiveActivityPage } from './live-activity-pages';
import { defaultPathAppearance } from './ui/path-appearance';

export function livePagesFromActivePeople(items: readonly ActiveFollowingItem[], i18n: Translator): LiveActivityPage[] {
  return items.flatMap(({ participant, timers }) => timers.map((timer) => {
    const progress = timer.progress;
    const interval = progress?.interval;
    const overall = progress?.overallTargetSeconds ? { recordedSeconds: progress.accumulatedSeconds, targetSeconds: progress.overallTargetSeconds, label: i18n.t('path.progress.overallLabel') } : undefined;
    return {
      timerId: timer.id, personId: participant.userId, displayName: participant.displayName, username: participant.username,
      profilePictureURL: participant.profilePictureURL, pathId: timer.path.id, pathName: timer.path.name, startedAt: timer.startedAt,
      appearance: defaultPathAppearance(timer.path.id),
      goal: interval ? { recordedSeconds: interval.recordedSeconds, targetSeconds: interval.targetSeconds, intervalStart: interval.startedAt, intervalEnd: interval.endedAt, label: i18n.t(interval.recurrence === 'hourly' ? 'pathDetails.period.hourly' : interval.recurrence === 'daily' ? 'pathDetails.period.daily' : interval.recurrence === 'weekly' ? 'pathDetails.period.weekly' : interval.recurrence === 'monthly' ? 'pathDetails.period.monthly' : interval.recurrence === 'yearly' ? 'pathDetails.period.yearly' : 'path.progress.intervalLabel') } : overall,
      overallGoal: interval ? overall : undefined,
    };
  }));
}
