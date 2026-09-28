import { cloneElement, isValidElement, useEffect, useRef, useState, type ReactNode } from 'react';
import { AccessibilityInfo, Animated, PanResponder, StyleSheet, View, type GestureResponderEvent } from 'react-native';
import { moveTile, nearestTileSlot, type TileFrame } from '../tile-reordering';
import type { PathCardProps } from './path-card';
import { mobileTheme } from './tokens';

type Props = {
  items: readonly ReactNode[];
  reorderHint: string;
  columns: number;
  disabled: boolean;
  scrollOffset: number;
  groupForID: (id: string) => string;
  onCommit: (ids: string[]) => void;
  onDragChange: (dragging: boolean) => void;
  onDragPosition: (pageY: number) => void;
  onTile: (id: string, node: View | null) => void;
  onFocusTarget: (id: string, node: View | null) => void;
};
type Drag = { id: string; original: string[]; slots: TileFrame[]; origin: TileFrame; pageX: number; pageY: number; x: number; y: number; scroll: number };

export function SortablePathGrid(props: Props) {
  const ids = props.items.map((item, index) => isValidElement(item) && item.key !== null ? String(item.key) : String(index));
  const signature = ids.join('\u0000');
  const [order, setOrder] = useState(ids);
  const [held, setHeld] = useState<string>();
  const frames = useRef(new Map<string, TileFrame>());
  const motions = useRef(new Map<string, Animated.ValueXY>());
  const drag = useRef<Drag | undefined>(undefined);
  const currentOrder = useRef(order);
  const latest = useRef(props);
  const reducedMotion = useRef(true);
  const suppressed = useRef(false);
  const gridWidth = useRef(0);
  latest.current = props;
  currentOrder.current = order;
  function motion(id: string) {
    let value = motions.current.get(id);
    if (!value) { value = new Animated.ValueXY(); motions.current.set(id, value); }
    return value;
  }
  function settle(id: string) {
    const value = motion(id);
    value.stopAnimation();
    if (reducedMotion.current) value.setValue({ x: 0, y: 0 });
    else Animated.spring(value, { toValue: { x: 0, y: 0 }, speed: 24, bounciness: 3, useNativeDriver: true }).start();
  }
  function finish(commit: boolean) {
    const owned = drag.current;
    if (!owned) return;
    drag.current = undefined;
    setHeld(undefined);
    latest.current.onDragChange(false);
    settle(owned.id);
    if (!commit) { currentOrder.current = owned.original; setOrder(owned.original); }
    else if (currentOrder.current.some((id, index) => id !== owned.original[index])) latest.current.onCommit(currentOrder.current);
    // Pressable may dispatch its release after the parent's responder release.
    setTimeout(() => { suppressed.current = false; }, 0);
  }
  function update() {
    const owned = drag.current;
    if (!owned) return;
    const dx = owned.x - owned.pageX;
    const dy = owned.y - owned.pageY + latest.current.scrollOffset - owned.scroll;
    const frame = frames.current.get(owned.id) ?? owned.origin;
    motion(owned.id).setValue({ x: owned.origin.x - frame.x + dx, y: owned.origin.y - frame.y + dy });
    const to = nearestTileSlot(owned.slots, owned.origin.x + owned.origin.width / 2 + dx, owned.origin.y + owned.origin.height / 2 + dy);
    const target = currentOrder.current[to];
    if (!target || latest.current.groupForID(target) !== latest.current.groupForID(owned.id)) return;
    const next = moveTile(currentOrder.current, owned.id, to);
    if (next.some((id, index) => id !== currentOrder.current[index])) { currentOrder.current = next; setOrder(next); }
  }
  const handlers = useRef({ update, finish });
  handlers.current = { update, finish };
  const responder = useRef(PanResponder.create({
    onMoveShouldSetPanResponderCapture: () => Boolean(drag.current),
    onPanResponderMove: (event, gesture) => {
      const owned = drag.current;
      if (!owned) return;
      if (gesture.numberActiveTouches > 1) { handlers.current.finish(false); return; }
      owned.x = event.nativeEvent.pageX; owned.y = event.nativeEvent.pageY;
      latest.current.onDragPosition(owned.y);
      handlers.current.update();
    },
    onPanResponderRelease: () => handlers.current.finish(true),
    onPanResponderTerminate: () => handlers.current.finish(false),
    onPanResponderTerminationRequest: () => !drag.current,
    onShouldBlockNativeResponder: () => Boolean(drag.current),
  })).current;
  useEffect(() => {
    let active = true;
    void AccessibilityInfo.isReduceMotionEnabled().then(value => { if (active) reducedMotion.current = value; });
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', value => { reducedMotion.current = value; });
    return () => { active = false; subscription.remove(); latest.current.onDragChange(false); motions.current.forEach(value => value.stopAnimation()); };
  }, []);
  useEffect(() => {
    finish(false);
    currentOrder.current = ids; setOrder(ids);
    for (const id of frames.current.keys()) if (!ids.includes(id)) { frames.current.delete(id); motions.current.delete(id); }
  }, [signature, props.columns]);
  useEffect(() => { update(); }, [props.scrollOffset]);
  useEffect(() => { if (props.disabled) finish(false); }, [props.disabled]);
  function begin(id: string, event: GestureResponderEvent) {
    if (latest.current.disabled || drag.current || ids.length < 2) return;
    const origin = frames.current.get(id);
    const slots = currentOrder.current.map(key => frames.current.get(key));
    if (!origin || slots.some(frame => !frame)) return;
    const { pageX, pageY } = event.nativeEvent;
    motions.current.forEach(value => value.stopAnimation());
    drag.current = { id, original: [...currentOrder.current], slots: slots as TileFrame[], origin, pageX, pageY, x: pageX, y: pageY, scroll: latest.current.scrollOffset };
    suppressed.current = true;
    setHeld(id);
    latest.current.onDragPosition(pageY);
    latest.current.onDragChange(true);
  }
  const elements = new Map(ids.map((id, index) => [id, props.items[index]]));
  // Always render only current elements when the collection changes mid-gesture.
  const rendered = order.filter(id => elements.has(id));
  for (const id of ids) if (!rendered.includes(id)) rendered.push(id);
  return <View style={styles.rows}
    onMoveShouldSetResponderCapture={responder.panHandlers.onMoveShouldSetResponderCapture}
    onResponderGrant={responder.panHandlers.onResponderGrant}
    onResponderMove={responder.panHandlers.onResponderMove}
    onResponderRelease={responder.panHandlers.onResponderRelease}
    onResponderTerminate={responder.panHandlers.onResponderTerminate}
    onResponderTerminationRequest={responder.panHandlers.onResponderTerminationRequest}
    onResponderStart={responder.panHandlers.onResponderStart}
    onResponderEnd={responder.panHandlers.onResponderEnd}
    onLayout={event => { const width = event.nativeEvent.layout.width; if (gridWidth.current && width !== gridWidth.current) finish(false); gridWidth.current = width; }}
    onTouchStart={event => { if (drag.current && event.nativeEvent.touches.length > 1) finish(false); }}
    onTouchEnd={() => finish(true)} onTouchCancel={() => finish(false)}>
    {rendered.map(id => {
      const item = elements.get(id);
      return <View key={id} collapsable={false}
        ref={node => props.onTile(id, node)}
        onLayout={event => {
          const previous = frames.current.get(id);
          const frame = event.nativeEvent.layout;
          frames.current.set(id, frame);
          if (drag.current?.id === id) update();
          else if (previous && (previous.x !== frame.x || previous.y !== frame.y)) {
            const value = motion(id);
            value.stopAnimation(offset => {
              value.setValue({ x: offset.x + previous.x - frame.x, y: offset.y + previous.y - frame.y });
              settle(id);
            });
          }
        }}
        style={{ width: props.columns === 2 ? '48.5%' : '100%', zIndex: held === id ? 10 : 0 }}>
        <Animated.View style={[styles.tile, { transform: [...motion(id).getTranslateTransform(), { scale: held === id && !reducedMotion.current ? 1.035 : 1 }] }, held === id ? styles.held : null]}>
          {isValidElement<PathCardProps>(item) ? cloneElement(item, {
            onFocusTarget: node => props.onFocusTarget(id, node),
            onLongPress: props.disabled ? undefined : event => begin(id, event),
            reorderHint: props.reorderHint,
            onOpen: () => { if (!suppressed.current) item.props.onOpen?.(); },
          }) : item}
        </Animated.View>
      </View>;
    })}
  </View>;
}
const styles = StyleSheet.create({
  rows: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', rowGap: mobileTheme.spacing.sm },
  tile: { flex: 1 },
  held: { shadowColor: '#000000', shadowOpacity: 0.18, shadowRadius: 14, shadowOffset: { width: 0, height: 6 }, elevation: 8 },
});
