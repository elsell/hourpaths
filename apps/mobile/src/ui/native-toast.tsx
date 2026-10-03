import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { AccessibilityInfo, Platform, Pressable, StyleSheet, Text, View } from 'react-native';
import { FullWindowOverlay } from 'react-native-screens';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { getLocales } from 'expo-localization';
import { createDeviceTranslator } from '../i18n';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);
type Notice = { id: symbol; message: string; dismiss: () => void };
const ToastContext = createContext<((notice: Notice) => () => void) | null>(null);

export function NativeToastProvider({ children }: { children: ReactNode }) {
  const [notices, setNotices] = useState<Notice[]>([]);
  const register = useRef((notice: Notice) => {
    setNotices(current => [...current.filter(item => item.id !== notice.id), notice]);
    return () => setNotices(current => current.filter(item => item.id !== notice.id));
  }).current;
  const notice = notices[0];
  return <ToastContext.Provider value={register}>{children}{notice ? <ToastOverlay key={String(notice.id)} notice={notice} /> : null}</ToastContext.Provider>;
}

// Declarative ownership ensures sign-out and route disposal remove private feedback.
export function NativeToast({ message, onDismiss }: { message: string; onDismiss: () => void }) {
  const register = useContext(ToastContext);
  const dismiss = useRef(onDismiss);
  dismiss.current = onDismiss;
  useEffect(() => register?.({ id: Symbol(message), message, dismiss: () => dismiss.current() }), [message, register]);
  return null;
}

function ToastOverlay({ notice }: { notice: Notice }) {
  const insets = useSafeAreaInsets();
  useEffect(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const configure = (enabled: boolean) => {
      if (timer) clearTimeout(timer);
      if (!enabled && !disposed) timer = setTimeout(notice.dismiss, 8000);
    };
    const listener = AccessibilityInfo.addEventListener('screenReaderChanged', configure);
    void AccessibilityInfo.isScreenReaderEnabled().then(enabled => {
      if (disposed) return;
      configure(enabled);
      if (enabled) AccessibilityInfo.announceForAccessibility(notice.message);
    });
    return () => { disposed = true; if (timer) clearTimeout(timer); listener.remove(); };
  }, [notice]);
  const content = <View pointerEvents="box-none" style={[StyleSheet.absoluteFill, styles.overlay]}>
    <View style={[styles.banner, { marginTop: insets.top + mobileTheme.spacing.xs }]}>
      <Text style={styles.message}>{notice.message}</Text>
      <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('common.dismiss')} onPress={notice.dismiss} style={styles.dismiss}>
        <Text style={styles.action}>{i18n.t('common.dismiss')}</Text>
      </Pressable>
    </View>
  </View>;
  return Platform.OS === 'ios' ? <FullWindowOverlay unstable_accessibilityContainerViewIsModal={false}>{content}</FullWindowOverlay> : content;
}

const styles = StyleSheet.create({
  overlay: { zIndex: 1000, elevation: 20 },
  banner: { marginHorizontal: mobileTheme.spacing.md, padding: mobileTheme.spacing.md, backgroundColor: mobileTheme.colors.surfaceRaised, borderRadius: mobileTheme.radii.lg, borderWidth: 1, borderColor: mobileTheme.colors.separator, gap: mobileTheme.spacing.xs },
  message: { ...mobileTheme.typography.body, color: mobileTheme.colors.text },
  dismiss: { alignSelf: 'flex-end', minHeight: mobileTheme.sizes.minimumTouchTarget, justifyContent: 'center', paddingHorizontal: mobileTheme.spacing.sm },
  action: { ...mobileTheme.typography.button, color: mobileTheme.colors.accent },
});
