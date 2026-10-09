import { HStack, Spacer, Text, VStack } from '@expo/ui/swift-ui';
import { font, lineLimit, padding } from '@expo/ui/swift-ui/modifiers';
import type { TimerSurfacePresentation } from './timer-surface-types';

export const timerLayout = (props: TimerSurfacePresentation) => {
  'widget';
  const rows = <VStack alignment="leading" spacing={6}>
    {/* Swift widget children must be immediate view nodes, not nested arrays. */}
    <VStack alignment="leading" spacing={6}>{props.timers.map(timer => <HStack key={timer.id} spacing={12}>
      <Text modifiers={[lineLimit(1)]}>{timer.name}</Text>
      <Spacer />
      <Text date={new Date(timer.startedAt)} dateStyle="timer" />
    </HStack>)}</VStack>
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

