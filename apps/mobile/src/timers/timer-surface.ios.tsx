import { HStack, Spacer, Text, VStack } from '@expo/ui/swift-ui';
import { font, lineLimit, padding } from '@expo/ui/swift-ui/modifiers';
import { createLiveActivity } from 'expo-widgets';
import { Platform } from 'react-native';
import type { CreateTimerSurface, TimerSurfacePresentation } from './timer-surface-types';

const timerLayout = (props: TimerSurfacePresentation) => {
  'widget';
  const rows = <VStack alignment="leading" spacing={6}>
    {props.timers.map(timer => <HStack key={timer.id} spacing={12}>
      <Text modifiers={[lineLimit(1)]}>{timer.name}</Text>
      <Spacer />
      <Text date={new Date(timer.startedAt)} dateStyle="timer" />
    </HStack>)}
    {props.remaining ? <Text modifiers={[font({ size: 12 })]}>{props.remaining}</Text> : null}
  </VStack>;
  return {
    banner: <VStack alignment="leading" spacing={8} modifiers={[padding({ all: 16 })]}>
      <Text modifiers={[font({ weight: 'semibold' }), lineLimit(1)]}>{props.title}</Text>
      {rows}
    </VStack>,
    bannerSmall: <VStack>
      <Text>{props.title}</Text>
      <Text date={new Date(props.timers[0].startedAt)} dateStyle="timer" />
    </VStack>,
    compactLeading: <Text>{props.count}</Text>,
    compactTrailing: <Text date={new Date(props.timers[0].startedAt)} dateStyle="timer" />,
    minimal: <Text>{props.count}</Text>,
    expandedCenter: <Text modifiers={[font({ weight: 'semibold' })]}>{props.title}</Text>,
    expandedBottom: rows,
  };
};

// Lazy construction keeps unsupported/denied native presentation inside the
// coordinator's failure boundary, separate from durable timer commands.
let activity: ReturnType<typeof createLiveActivity<TimerSurfacePresentation>> | undefined;
function factory() {
  return activity ??= createLiveActivity<TimerSurfacePresentation>('HourPathsTimers', timerLayout);
}

export const createTimerSurface: CreateTimerSurface = copy => ({
  async clear() {
    if (Number.parseFloat(String(Platform.Version)) < 16.1) return;
    for (const instance of factory().getInstances()) await instance.end('immediate');
  },
  async replace(timers) {
    if (Number.parseFloat(String(Platform.Version)) < 16.2) return;
    if (!timers.length) {
      for (const instance of factory().getInstances()) await instance.end('immediate');
      return;
    }
    const ordered = [...timers].sort((a, b) => a.startedAt - b.startedAt || a.id.localeCompare(b.id));
    const props: TimerSurfacePresentation = {
      title: copy.title(ordered.length), count: copy.count(ordered.length),
      remaining: ordered.length > 3 ? copy.remaining(ordered.length - 3) : '',
      timers: ordered.slice(0, 3),
    };
    const [current, ...duplicates] = factory().getInstances();
    for (const duplicate of duplicates) await duplicate.end('immediate');
    if (current) await current.update(props);
    else factory().start(props, 'hourpaths:///');
  },
});
