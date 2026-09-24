import { useEffect, useState } from 'react';
import type { Translator } from '@hourpaths/i18n';
import { AccessibilityInfo } from 'react-native';
import { NativeSheet, StatusBanner, ThemedText as Text } from './primitives';

export type PathArchiveConfirmationSheetProps = {
  archived: boolean; busy: boolean; errorText?: string; i18n: Translator; onCancel: () => void; onConfirm: () => void; visible: boolean;
};
export function PathArchiveConfirmationSheet({ archived, busy, errorText, i18n, onCancel, onConfirm, visible }: PathArchiveConfirmationSheetProps) {
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  useEffect(() => { void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion); const listener = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion); return () => listener.remove(); }, []);
  const action = archived ? 'pathArchive.confirmUnarchive' : 'pathArchive.confirm';
  const actionLabel = errorText ? i18n.t('common.retry') : i18n.t(action);
  return <NativeSheet animationType={reduceMotion === false ? 'slide' : 'none'} compact dismissible={!busy}
    leadingAction={{ disabled: busy, label: i18n.t('common.cancel'), onPress: onCancel }} onRequestClose={onCancel}
    title={i18n.t('pathArchive.heading')} trailingAction={{ disabled: busy, label: actionLabel, onPress: onConfirm }} visible={visible}>
    <Text accessibilityRole="alert">{i18n.t(archived ? 'pathArchive.unarchiveWarning' : 'pathArchive.warning')}</Text>
    {busy ? <StatusBanner text={i18n.t('common.loading')} tone="loading" /> : null}
    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
  </NativeSheet>;
}
