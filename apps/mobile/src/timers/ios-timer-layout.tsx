import { Button, HStack, ProgressView, Spacer, Text, VStack } from '@expo/ui/swift-ui';
import { font, foregroundStyle, lineLimit, monospacedDigit, padding, tint, frame } from '@expo/ui/swift-ui/modifiers';
import type { TimerSurfaceEnvelope, TimerSurfacePresentation } from './timer-surface-types';

export const timerLayout = (props: TimerSurfacePresentation | TimerSurfaceEnvelope, environment?: { colorScheme?: 'light' | 'dark'; timestamp?: number; isStale?: boolean }) => {
  'widget';
  if (!('timers' in props) || !props.timers.length) return { banner: <VStack>{null}</VStack> };
  const now = environment?.timestamp ?? Date.now();
  const timers = props.timers.map(timer => {
    if (environment?.isStale) return { ...timer, goal: undefined };
    if (!timer.goalPeriods) return timer;
    const period = timer.goalPeriods.find(value => value.startsAt <= now && now < value.endsAt);
    if (!period || !timer.goal) return { ...timer, goal: undefined };
    const stopped = timer.stoppedAt !== undefined;
    const lower = (stopped ? now : Math.max(period.startsAt, timer.startedAt)) - period.savedSeconds * 1000;
    const progressUpper = lower + period.targetSeconds * 1000;
    return { ...timer, goal: {
      ...timer.goal, lower, upper: period.endsAt,
      pauseAt: stopped ? now : undefined,
      fraction: Math.min(1, Math.max(0, (now - lower) / (period.targetSeconds * 1000))),
      progressUpper: progressUpper <= period.endsAt ? progressUpper : undefined,
    } };
  });
  const primary = timers.find(timer => timer.stoppedAt === undefined) ?? timers[0];
  const primaryClock = primary.stoppedAt !== undefined
    ? <Text timerInterval={{ lower: new Date(primary.startedAt), upper: new Date(Math.max(primary.startedAt + 1, primary.stoppedAt)) }} countsDown={false} pauseTime={new Date(primary.stoppedAt)} />
    : <Text date={new Date(primary.startedAt)} dateStyle="timer" />;
  const rows = <VStack alignment="leading" spacing={6}>
    {/* Swift widget children must be immediate view nodes, not nested arrays. */}
    <VStack alignment="leading" spacing={8}>{timers.map(timer => <VStack key={timer.pathId} alignment="leading" spacing={5}>
      <HStack spacing={10}>
        <Text modifiers={[font({ size: 22 })]}>{timer.emoji}</Text>
        <Text modifiers={[font({ weight: 'semibold' }), lineLimit(1)]}>{timer.name}</Text>
        <Spacer />
        {timer.stoppedAt !== undefined ? <Text timerInterval={{ lower: new Date(timer.startedAt), upper: new Date(Math.max(timer.startedAt + 1, timer.stoppedAt)) }} countsDown={false} pauseTime={new Date(timer.stoppedAt)} modifiers={[font({ size: 18, weight: 'semibold' }), monospacedDigit()]} /> : <Text date={new Date(timer.startedAt)} dateStyle="timer" modifiers={[font({ size: 18, weight: 'semibold' }), monospacedDigit()]} />}
      {timer.action ? <Button target={timer.action.target} label={timer.action.label} systemImage={timer.stoppedAt !== undefined ? "play.fill" : "stop.fill"} /> : null}
      </HStack>
      {timer.goal ? <VStack alignment="leading" spacing={4}>
        {timer.goal.lower !== undefined && timer.goal.upper !== undefined ? <HStack spacing={4}>
          <Text timerInterval={{ lower: new Date(timer.goal.lower), upper: new Date(timer.goal.upper) }} countsDown={false} pauseTime={timer.goal.pauseAt !== undefined ? new Date(timer.goal.pauseAt) : timer.stoppedAt !== undefined ? new Date(timer.stoppedAt) : undefined} modifiers={[font({ size: 12 }), monospacedDigit()]} />
          <Text modifiers={[font({ size: 12 }), foregroundStyle({ type: 'hierarchical', style: 'secondary' })]}>{timer.goal.targetText}</Text>
        {timer.stoppedAt === undefined && timer.goal.lower !== undefined && timer.goal.progressUpper !== undefined ? <ProgressView
          timerInterval={{ lower: new Date(timer.goal.lower), upper: new Date(timer.goal.progressUpper) }}
          countsDown={false} modifiers={[tint(timer.color ?? '#DFDEFF'), frame({ width: 40 })]} /> : null}
        </HStack> : <Text modifiers={[font({ size: 12 }), foregroundStyle({ type: 'hierarchical', style: 'secondary' })]}>{timer.goal.text}</Text>}
      </VStack> : null}
    </VStack>)}</VStack>
    {props.remaining ? <Text modifiers={[font({ size: 12 })]}>{props.remaining}</Text> : null}
  </VStack>;
  return {
    banner: <VStack alignment="leading" spacing={8} modifiers={[padding({ all: 12 })]}>
      <Text modifiers={[font({ weight: 'semibold' }), lineLimit(1)]}>{props.title}</Text>
      {rows}
    </VStack>,
    bannerSmall: <VStack>
      <Text>{props.title}</Text>
      {primaryClock}
    </VStack>,
    compactLeading: <Text>{primary.emoji}</Text>,
    compactTrailing: primaryClock,
    minimal: <Text>{primary.emoji}</Text>,
    expandedCenter: <Text modifiers={[font({ weight: 'semibold' })]}>{props.title}</Text>,
    expandedBottom: rows,
  };
};

