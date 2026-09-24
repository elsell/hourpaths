import { useEffect, useRef, useSyncExternalStore, type ReactNode } from 'react';
import { RefreshControl, ScrollView } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { mobileShellStyles } from './primitives';
import { mobileTheme } from './tokens';

export type NativeRoutePresentation = {
  actions: readonly NativeRouteAction[];
  content: ReactNode;
  pathID: string;
  title: string;
};

export type NativeRouteAction = {
  destructive?: boolean;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  systemImage: 'archivebox' | 'archivebox.fill' | 'clock.arrow.circlepath' | 'hand.raised' | 'hand.wave' | 'person.2' | 'person.badge.plus' | 'pin' | 'pin.slash' | 'rectangle.portrait.and.arrow.right' | 'slider.horizontal.3' | 'trash';
};

type PublishedNativeRoute = NativeRoutePresentation & {
  dismiss: () => void;
  owner: object;
};

let currentPresentation: PublishedNativeRoute | null = null;
const listeners = new Set<() => void>();

function emitChange() {
  for (const listener of listeners) listener();
}

export function NativeRouteSource({
  actions,
  children,
  onDismiss,
  pathID,
  title,
}: {
  actions: readonly NativeRouteAction[];
  children: ReactNode;
  onDismiss: () => void;
  pathID: string;
  title: string;
}) {
  const owner = useRef({});
  const dismissRef = useRef(onDismiss);
  dismissRef.current = onDismiss;

  useEffect(() => {
    const presentation: PublishedNativeRoute = {
      actions,
      content: children,
      dismiss: () => dismissRef.current(),
      owner: owner.current,
      pathID,
      title,
    };
    currentPresentation = presentation;
    emitChange();
    return () => {
      if (currentPresentation?.owner === owner.current) {
        currentPresentation = null;
        emitChange();
      }
    };
  }, [pathID]);

  useEffect(() => {
    if (currentPresentation?.owner !== owner.current) return;
    currentPresentation = {
      actions,
      content: children,
      dismiss: () => dismissRef.current(),
      owner: owner.current,
      pathID,
      title,
    };
    emitChange();
  }, [actions, children, pathID, title]);

  return null;
}

export function useNativeRoutePresentation(pathID: string): NativeRoutePresentation | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => currentPresentation?.pathID === pathID ? currentPresentation : null,
    () => null,
  );
}

export function dismissNativeRoute(pathID: string) {
  if (currentPresentation?.pathID !== pathID) return;
  const presentation = currentPresentation;
  currentPresentation = null;
  emitChange();
  presentation.dismiss();
}

export function NativeRouteScreen({
  children,
  grouped = false,
  onRefresh,
  refreshing = false,
}: {
  children: ReactNode;
  grouped?: boolean;
  onRefresh?: () => void;
  refreshing?: boolean;
}) {
  return <SafeAreaView edges={['left', 'right', 'bottom']} style={mobileShellStyles.screen}>
    <ScrollView
      automaticallyAdjustContentInsets
      alwaysBounceVertical={Boolean(onRefresh)}
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={[
        mobileShellStyles.scrollContent,
        grouped ? mobileShellStyles.groupedScrollContent : null,
      ]}
      refreshControl={onRefresh ? <RefreshControl
        colors={[mobileTheme.colors.accent]}
        onRefresh={onRefresh}
        refreshing={refreshing}
        tintColor={mobileTheme.colors.accent}
      /> : undefined}
    >
      {children}
    </ScrollView>
  </SafeAreaView>;
}
