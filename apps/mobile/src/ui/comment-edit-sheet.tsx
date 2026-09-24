import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Alert, InputAccessoryView, Keyboard, StyleSheet, View } from 'react-native';
import { commentDraftIsDirty, normalizedComment } from './comment-presentation';
import { createCommentSubmissionOwner } from './comment-submission-admission';
import { NativePrimaryButton } from './native-primary-button';
import { NativeSheet, StatusBanner, ThemedTextInput } from './primitives';
import { mobileTheme } from './tokens';

export function CommentEditSheet({
  baseline,
  admittedBusy,
  commentID,
  draft,
  i18n,
  onDraftChange,
  onClose,
  onSave,
  visible,
}: {
  baseline: string;
  admittedBusy: boolean;
  commentID: string;
  draft: string;
  i18n: Translator;
  onDraftChange: (draft: string) => void;
  onClose: () => void;
  onSave: (text: string) => Promise<void>;
  visible: boolean;
}) {
  const [localBusy, setLocalBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  const submission = useRef(createCommentSubmissionOwner());
  const accessoryID = `comment-edit-keyboard-${commentID}`;
  const validDraft = normalizedComment(draft);
  const dirty = commentDraftIsDirty(baseline, draft);
  const busy = admittedBusy || localBusy;

  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => subscription.remove();
  }, []);
  useEffect(() => {
    if (!visible) return;
    setLocalBusy(false);
    setFailed(false);
  }, [visible]);

  const close = () => {
    if (busy || submission.current.active()) return;
    if (!dirty) { onClose(); return; }
    Alert.alert(
      i18n.t('social.commentsDiscardTitle'),
      i18n.t('social.commentsDiscardDescription'),
      [
        { style: 'cancel', text: i18n.t('common.cancel') },
        { onPress: onClose, style: 'destructive', text: i18n.t('social.commentsDiscardAction') },
      ],
    );
  };

  const save = async () => {
    if (busy || !dirty || !validDraft) return;
    const admission = submission.current.admit();
    if (!admission) return;
    setLocalBusy(true);
    setFailed(false);
    try {
      await onSave(validDraft);
      if (submission.current.owns(admission)) onClose();
    } catch {
      if (submission.current.owns(admission)) setFailed(true);
    } finally {
      if (submission.current.release(admission)) setLocalBusy(false);
    }
  };

  return <NativeSheet
    animationType={reduceMotion === false ? 'slide' : 'none'}
    compact
    dismissible={!busy}
    leadingAction={{ disabled: busy, label: i18n.t('common.cancel'), onPress: close }}
    onRequestClose={close}
    title={i18n.t('social.commentsEditTitle')}
    trailingAction={{
      disabled: busy || !dirty || !validDraft,
      label: i18n.t('social.commentsSave'),
      onPress: () => { void save(); },
    }}
    visible={visible}
  >
    <ThemedTextInput
      accessibilityLabel={i18n.t('social.commentsEditTitle')}
      autoFocus
      editable={!busy}
      inputAccessoryViewID={accessoryID}
      multiline
      onChangeText={onDraftChange}
      style={styles.input}
      value={draft}
    />
    {failed ? <StatusBanner text={i18n.t('social.commentsMutationUnavailable')} tone="error" /> : null}
    {busy ? <View accessibilityLiveRegion="polite">
      <StatusBanner text={i18n.t('social.commentsSaving')} tone="loading" />
    </View> : null}
    <InputAccessoryView nativeID={accessoryID}>
      <View style={styles.keyboardBar}>
        <NativePrimaryButton
          label={i18n.t('common.done')}
          onPress={Keyboard.dismiss}
          variant="plain"
        />
      </View>
    </InputAccessoryView>
  </NativeSheet>;
}

const styles = StyleSheet.create({
  input: { minHeight: 132, textAlignVertical: 'top' },
  keyboardBar: {
    alignItems: 'flex-end',
    backgroundColor: mobileTheme.colors.surfaceRaised,
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: mobileTheme.spacing.sm,
  },
});
