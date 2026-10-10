import { useEffect, useRef, useSyncExternalStore } from 'react';
import type { PathAppearance, StatsSelection, StatsState } from '@hourpaths/client-core';

type StatsPresentation = {
  state: StatsState;
  calendarState: StatsState;
  onCalendarSelect: (selection: StatsSelection) => void;
  onCalendarRefresh: () => void;
  onSelect: (selection: StatsSelection) => void;
  onRefresh: () => void;
  appearance: (id: string) => PathAppearance;
};
type Published = StatsPresentation & { owner: object; sessionKey: string };
let current: Published | null = null;
const listeners = new Set<() => void>();
const emit = () => { for (const listener of listeners) listener(); };
export function StatsRouteSource(props: StatsPresentation & { sessionKey: string; isCurrent: () => boolean }) {
  const owner = useRef({});
  useEffect(() => {
    current = { ...props, owner: owner.current,
      onCalendarSelect: selection => { if (props.isCurrent() && current?.owner === owner.current && current.sessionKey === props.sessionKey) props.onCalendarSelect(selection); },
      onCalendarRefresh: () => { if (props.isCurrent() && current?.owner === owner.current && current.sessionKey === props.sessionKey) props.onCalendarRefresh(); },
      onSelect: selection => { if (props.isCurrent() && current?.owner === owner.current && current.sessionKey === props.sessionKey) props.onSelect(selection); },
      onRefresh: () => { if (props.isCurrent() && current?.owner === owner.current && current.sessionKey === props.sessionKey) props.onRefresh(); },
    };
    emit();
    return () => { if (current?.owner === owner.current) { current = null; emit(); } };
  }, [props.state, props.calendarState, props.onCalendarSelect, props.onCalendarRefresh, props.sessionKey, props.appearance, props.onSelect, props.onRefresh, props.isCurrent]);
  return null;
}
export function useStatsPresentation(): StatsPresentation | null {
  return useSyncExternalStore(listener => { listeners.add(listener); return () => { listeners.delete(listener); }; }, () => current, () => null);
}
