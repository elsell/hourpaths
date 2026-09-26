import type { PracticeComment } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  FlatList,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  StyleSheet,
  useWindowDimensions,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { needsCompactVerticalLayout } from './adaptive-layout';
import type { CommentAction } from './comment-action';
import { CommentActionMenu } from './comment-action-menu';
import {
  beginPracticeCommentEdit,
  clearPracticeCommentEditDraft,
  practiceCommentDraftGeneration,
  restorePracticeCommentComposerDraftIfEmpty,
  setPracticeCommentComposerDraft,
  setPracticeCommentEditDraft,
  usePracticeCommentComposerDraft,
  usePracticeCommentEditDraft,
} from './comment-draft-presentation';
import { CommentEditSheet } from './comment-edit-sheet';
import { CommentHeartIcon } from './comment-heart-icon';
import { normalizedComment } from './comment-presentation';
import { createCommentSubmissionOwner } from './comment-submission-admission';
import { NativeCommentSendButton } from './native-comment-send-button';
import { NativeButton } from './native-button';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ActionButton, ThemedText as Text, ThemedTextInput } from './primitives';
import type { PracticeCommentsRoutePresentation } from './practice-comments-route-presentation';
import { SocialProfileAvatar } from './social-profile-avatar';
import { mobileTheme } from './tokens';

function CommentRow({ comment, i18n, presentation }: {
  comment: PracticeComment;
  i18n: Translator;
  presentation: PracticeCommentsRoutePresentation;
}) {
  const editDraft = usePracticeCommentEditDraft(presentation.eventID, comment.id, comment.text);
  const canEdit = comment.authorUserId === presentation.viewerID && !comment.pending;
  const canDelete = (canEdit || presentation.eventOwnerID === presentation.viewerID) && !comment.pending;
  const history = presentation.history?.commentID === comment.id ? presentation.history : undefined;
  const actions: CommentAction[] = [];
  if (canEdit) actions.push({
    label: i18n.t('social.commentsEdit'),
    onPress: () => beginPracticeCommentEdit(presentation.eventID, comment.id, comment.text),
    systemImage: 'pencil',
  });
  if (comment.edited && !comment.pending) actions.push({ label: i18n.t('social.commentsHistory'), onPress: () => presentation.loadHistory(comment), systemImage: 'clock.arrow.circlepath' });
  if (canDelete) actions.push({
    destructive: true,
    label: i18n.t('social.commentsDelete'),
    onPress: () => Alert.alert(
      i18n.t('social.commentsDeleteConfirmTitle'),
      i18n.t('social.commentsDeleteConfirmDescription'),
      [
        { text: i18n.t('common.cancel'), style: 'cancel' },
        { text: i18n.t('social.commentsDelete'), style: 'destructive', onPress: () => { void presentation.remove(comment); } },
      ],
    ),
    systemImage: 'trash',
  });

  return <View style={[styles.row, comment.id === presentation.focusedCommentID ? styles.focused : null]}>
    <SocialProfileAvatar
      accessibilityLabel={comment.author.profilePictureURL
        ? i18n.t('social.commentsAuthorAvatar', { displayName: comment.author.displayName })
        : i18n.t('social.neutralAvatarLabel')}
      profilePictureURL={comment.author.profilePictureURL}
      size={36}
    />
    <View style={styles.rowCopy}>
      <View style={styles.byline}>
        <Text style={styles.name}>{comment.author.displayName}</Text>
        {comment.edited ? <Text style={styles.metadata}>{i18n.t('social.commentsEdited')}</Text> : null}
        {comment.pending ? <ActivityIndicator color={mobileTheme.colors.textMuted} size="small" /> : null}
      </View>
      <Text allowFontScaling selectable style={styles.comment}>{comment.text}</Text>
      <View style={styles.heartActions}>
        <Pressable
          accessibilityLabel={i18n.t(comment.heartedByViewer ? 'social.commentHeartRemove' : 'social.commentHeartAdd')}
          accessibilityRole="button"
          accessibilityState={{ busy: comment.heartPending, selected: comment.heartedByViewer }}
          disabled={comment.pending}
          hitSlop={8}
          onPress={() => { void presentation.mutateHeart(comment); }}
          style={({ pressed }) => [
            styles.heartButton,
            comment.heartedByViewer ? styles.heartSelected : null,
            pressed ? styles.heartPressed : null,
          ]}
        >
          <CommentHeartIcon selected={comment.heartedByViewer} />
        </Pressable>
        {comment.heartCount > 0 ? <Pressable
          accessibilityLabel={i18n.t('social.commentHeartCount', { count: comment.heartCount })}
          accessibilityRole="button"
          hitSlop={8}
          onPress={() => presentation.openHeartRoster(comment)}
          style={({ pressed }) => [styles.heartCountButton, pressed ? styles.heartPressed : null]}
        >
          <Text style={styles.heartCount}>{i18n.number(comment.heartCount)}</Text>
        </Pressable> : null}
      </View>
      {history ? <View accessibilityLiveRegion="polite" style={styles.history}>
        <Text accessibilityRole="header" style={styles.historyHeading}>{i18n.t('social.commentsHistoryHeading')}</Text>
        {history.loading && history.versions.length === 0 ? <ActivityIndicator color={mobileTheme.colors.accent} /> : null}
        {history.versions.map((version) => <View key={version.version} style={styles.version}>
          <Text style={styles.metadata}>v{version.version} · {i18n.date(new Date(version.createdAt), { dateStyle: 'medium' })}</Text>
          <Text allowFontScaling selectable>{version.text}</Text>
        </View>)}
        {history.errorKey ? <Text accessibilityRole="alert" style={styles.error}>{i18n.t(history.errorKey)}</Text> : null}
        {history.nextCursor ? <ActionButton disabled={history.loading} label={i18n.t('social.commentsLoadMore')} onPress={() => presentation.loadHistory(comment, history.nextCursor)} variant="quiet" /> : null}
      </View> : null}
    </View>
    {actions.length > 0 ? <CommentActionMenu accessibilityLabel={i18n.t('social.commentsActions')} actions={actions} /> : null}
    <CommentEditSheet
      admittedBusy={presentation.busy}
      baseline={comment.text}
      draft={editDraft.draft}
      i18n={i18n}
      onClose={() => clearPracticeCommentEditDraft(presentation.eventID, comment.id)}
      onDraftChange={(draft) => setPracticeCommentEditDraft(presentation.eventID, comment.id, draft)}
      onSave={(text) => presentation.edit(comment, text)}
      visible={editDraft.visible}
    />
  </View>;
}

export function PracticeCommentsView({ i18n, presentation }: { i18n: Translator; presentation: PracticeCommentsRoutePresentation }) {
  const insets = useSafeAreaInsets();
  const keyboardFrame = useRef<View>(null);
  const [keyboardOffset, setKeyboardOffset] = useState(0);
  const draft = usePracticeCommentComposerDraft(presentation.eventID);
  const list = useRef<FlatList<PracticeComment>>(null);
  const composerSubmission = useRef(createCommentSubmissionOwner());
  const { fontScale, width } = useWindowDimensions();
  const stackComposer = needsCompactVerticalLayout(width, fontScale);
  const validDraft = normalizedComment(draft);
  useEffect(() => {
    if (presentation.focusedCommentID) {
      const index = presentation.items.findIndex(({ id }) => id === presentation.focusedCommentID);
      if (index >= 0) setTimeout(() => list.current?.scrollToIndex({ animated: true, index, viewPosition: 0.5 }), 0);
    }
  }, [presentation.focusedCommentID, presentation.items]);

  const empty = presentation.status === 'loading'
    ? <View accessibilityLabel={i18n.t('social.commentsLoading')} accessibilityRole="progressbar" style={styles.state}>
        <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      </View>
    : presentation.status === 'error'
      ? <View style={styles.state}><NativeContentUnavailable
          description={i18n.t(presentation.errorKey ?? 'social.commentsUnavailableDescription')}
          systemImage="wifi.exclamationmark"
          title={i18n.t('social.commentsUnavailableHeading')}
        /><ActionButton label={i18n.t('common.retry')} onPress={presentation.retry} variant="secondary" /></View>
      : <NativeContentUnavailable
          description={i18n.t('social.commentsEmptyDescription')}
          systemImage="bubble.left.and.bubble.right"
          title={i18n.t('social.commentsEmptyHeading')}
        />;

  return <View ref={keyboardFrame} style={styles.screen} onLayout={() => {
    // Android reports window coordinates; keyboard events use screen coordinates.
    keyboardFrame.current?.measureInWindow((_x, y) => setKeyboardOffset(y + (Platform.OS === 'android' ? insets.top : 0)));
  }}>
  <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : 'height'} keyboardVerticalOffset={keyboardOffset} style={styles.screen}>
    <FlatList
      automaticallyAdjustContentInsets
      contentContainerStyle={presentation.items.length ? styles.list : styles.emptyList}
      data={presentation.items}
      ItemSeparatorComponent={() => <View style={styles.separator} />}
      keyboardDismissMode={Platform.OS === 'ios' ? 'interactive' : 'on-drag'}
      keyboardShouldPersistTaps="handled"
      ListEmptyComponent={empty}
      ListFooterComponent={presentation.nextCursor ? <NativeButton busy={presentation.loadingMore} disabled={presentation.loadingMore} label={i18n.t(presentation.loadingMore ? 'social.commentsLoadingMore' : 'social.commentsLoadMore')} onPress={presentation.loadMore} variant="quiet" /> : null}
      onRefresh={presentation.refresh}
      ref={list}
      refreshing={presentation.refreshing}
      renderItem={({ item }) => <CommentRow comment={item} i18n={i18n} presentation={presentation} />}
    />
    {presentation.errorKey && presentation.items.length > 0 ? <Text accessibilityLiveRegion="assertive" accessibilityRole="alert" style={styles.errorBanner}>
      {i18n.t(presentation.errorKey)}
    </Text> : null}
    <View style={[styles.composer, { paddingBottom: Math.max(insets.bottom, mobileTheme.spacing.sm) }, stackComposer ? styles.composerStacked : null]}>
      <ThemedTextInput
        accessibilityLabel={i18n.t('social.commentsPlaceholder')}
        allowFontScaling
        multiline
        onChangeText={(value) => setPracticeCommentComposerDraft(presentation.eventID, value)}
        placeholder={i18n.t('social.commentsPlaceholder')}
        style={[styles.composerInput, stackComposer && styles.composerStackedInput]}
        value={draft}
      />
      <NativeCommentSendButton
        busy={presentation.busy}
        disabled={!validDraft}
        label={i18n.t('social.commentsSend')}
        onPress={() => {
          if (!validDraft) return;
          const admission = composerSubmission.current.admit();
          if (!admission) return;
          const admittedDraftGeneration = practiceCommentDraftGeneration();
          const value = validDraft;
          setPracticeCommentComposerDraft(presentation.eventID, '');
          void presentation.create(value)
            .catch(() => {
              if (composerSubmission.current.owns(admission)) {
                restorePracticeCommentComposerDraftIfEmpty(
                  presentation.eventID,
                  value,
                  admittedDraftGeneration,
                );
              }
            })
            .finally(() => {
              if (composerSubmission.current.owns(admission)) composerSubmission.current.release(admission);
            });
        }}
      />
    </View>
  </KeyboardAvoidingView></View>;
}

const styles = StyleSheet.create({
  byline: { alignItems: 'center', flexDirection: 'row', flexWrap: 'wrap', gap: mobileTheme.spacing.xs },
  comment: { color: mobileTheme.colors.text, fontSize: 16, lineHeight: 22 },
  composer: { alignItems: 'flex-end', backgroundColor: mobileTheme.colors.surface, borderTopColor: mobileTheme.colors.border, borderTopWidth: StyleSheet.hairlineWidth, flexDirection: 'row', gap: mobileTheme.spacing.sm, padding: mobileTheme.spacing.sm },
  composerStacked: { alignItems: 'stretch', flexDirection: 'column' },
  composerInput: { flex: 1, maxHeight: 120, minHeight: 44, paddingVertical: mobileTheme.spacing.sm },
  composerStackedInput: { flex: 0 },
  emptyList: { flexGrow: 1, justifyContent: 'center' },
  error: { color: mobileTheme.colors.error },
  errorBanner: { backgroundColor: mobileTheme.colors.errorSurface, color: mobileTheme.colors.error, padding: mobileTheme.spacing.sm, textAlign: 'center' },
  focused: { backgroundColor: mobileTheme.colors.surfacePressed },
  heartActions: { alignItems: 'center', flexDirection: 'row', gap: mobileTheme.spacing.xxs, minHeight: mobileTheme.sizes.minimumTouchTarget },
  heartButton: { alignItems: 'center', borderRadius: mobileTheme.radii.pill, justifyContent: 'center', minHeight: mobileTheme.sizes.minimumTouchTarget, minWidth: mobileTheme.sizes.minimumTouchTarget },
  heartCount: { color: mobileTheme.colors.textMuted, fontSize: 13, fontWeight: '600' },
  heartCountButton: { alignItems: 'center', borderRadius: mobileTheme.radii.pill, justifyContent: 'center', minHeight: mobileTheme.sizes.minimumTouchTarget, minWidth: mobileTheme.sizes.minimumTouchTarget, paddingHorizontal: mobileTheme.spacing.xs },
  heartPressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  heartSelected: { backgroundColor: mobileTheme.colors.surfaceRaised },
  history: { borderLeftColor: mobileTheme.colors.border, borderLeftWidth: StyleSheet.hairlineWidth, gap: mobileTheme.spacing.sm, marginTop: mobileTheme.spacing.sm, paddingLeft: mobileTheme.spacing.sm },
  historyHeading: { fontSize: 15, fontWeight: '600' },
  list: { paddingBottom: mobileTheme.spacing.md },
  metadata: { color: mobileTheme.colors.textMuted, fontSize: 12, lineHeight: 16 },
  name: { flexShrink: 1, fontSize: 15, fontWeight: '600' },
  row: { alignItems: 'flex-start', flexDirection: 'row', gap: mobileTheme.spacing.sm, minHeight: 64, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  rowCopy: { flex: 1, gap: mobileTheme.spacing.xxs, minWidth: 0 },
  screen: { backgroundColor: mobileTheme.colors.background, flex: 1 },
  separator: { backgroundColor: mobileTheme.colors.border, height: StyleSheet.hairlineWidth, marginLeft: 64 },
  state: { alignItems: 'center', gap: mobileTheme.spacing.sm, justifyContent: 'center', minHeight: 240, padding: mobileTheme.spacing.lg },
  version: { gap: mobileTheme.spacing.xxs },
});
