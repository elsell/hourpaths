import { NativeButton } from './native-button';
import type { PracticeCommentHearter } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, FlatList, Pressable, StyleSheet, View } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import type { CommentHeartRosterRoutePresentation } from './comment-heart-roster-route-presentation';
import { ThemedText as Text } from './primitives';
import { SettingsIcon } from './settings-icon';
import { SocialProfileAvatar } from './social-profile-avatar';
import { mobileTheme } from './tokens';

function HeartRosterRow({ i18n, onPress, person }: {
  i18n: Translator;
  onPress: () => void;
  person: PracticeCommentHearter;
}) {
  return <Pressable
    accessibilityLabel={i18n.t('social.commentHeartRosterPersonLabel', {
      displayName: person.displayName,
      username: person.username,
    })}
    accessibilityRole="button"
    onPress={onPress}
    style={({ pressed }) => [styles.row, pressed ? styles.rowPressed : null]}
  >
    <SocialProfileAvatar
      accessibilityLabel={person.profilePictureURL
        ? i18n.t('social.commentsAuthorAvatar', { displayName: person.displayName })
        : i18n.t('social.neutralAvatarLabel')}
      profilePictureURL={person.profilePictureURL}
      size={40}
    />
    <View style={styles.identity}>
      <Text style={styles.displayName}>{person.displayName}</Text>
      <Text style={styles.username}>@{person.username}</Text>
    </View>
    <SettingsIcon systemName="chevron.right" variant="disclosure" />
  </Pressable>;
}

export function CommentHeartRosterView({ i18n, presentation }: {
  i18n: Translator;
  presentation: CommentHeartRosterRoutePresentation;
}) {
  const empty = presentation.status === 'loading'
    ? <View
        accessibilityLabel={i18n.t('social.commentHeartRosterLoading')}
        accessibilityRole="progressbar"
        style={styles.state}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      </View>
    : presentation.status === 'error'
      ? <View style={styles.state}>
          <NativeContentUnavailable
            description={i18n.t(presentation.errorKey ?? 'social.commentHeartRosterUnavailableDescription')}
            systemImage="wifi.exclamationmark"
            title={i18n.t('social.commentHeartRosterUnavailableHeading')}
          />
          <NativeButton label={i18n.t('common.retry')} onPress={presentation.retry} variant="quiet" />
        </View>
      : <NativeContentUnavailable
          description={i18n.t('social.commentHeartRosterEmptyDescription')}
          systemImage="heart"
          title={i18n.t('social.commentHeartRosterEmptyHeading')}
        />;

  const footer = presentation.errorKey && presentation.items.length > 0
    ? <View accessibilityRole="alert" style={styles.partialError}>
        <Text style={styles.errorText}>{i18n.t(presentation.errorKey)}</Text>
        <NativeButton label={i18n.t('common.retry')} onPress={presentation.retry} variant="quiet" />
      </View>
    : presentation.nextCursor ? <NativeButton busy={presentation.loadingMore} disabled={presentation.loadingMore} label={i18n.t(presentation.loadingMore ? 'social.commentHeartRosterLoadingMore' : 'social.commentHeartRosterLoadMore')} onPress={presentation.loadMore} variant="quiet" /> : null;

  return <FlatList
    automaticallyAdjustContentInsets
    contentContainerStyle={presentation.items.length ? styles.list : styles.emptyList}
    data={presentation.items}
    ItemSeparatorComponent={() => <View style={styles.separator} />}
    ListEmptyComponent={empty}
    ListFooterComponent={footer}
    onRefresh={presentation.refresh}
    refreshing={presentation.refreshing}
    renderItem={({ item }) => <HeartRosterRow
      i18n={i18n}
      onPress={() => presentation.openProfile(item)}
      person={item}
    />}
    style={styles.screen}
  />;
}

const styles = StyleSheet.create({
  displayName: { fontSize: 16, fontWeight: '600', lineHeight: 21 },
  emptyList: { flexGrow: 1, justifyContent: 'center' },
  identity: { flex: 1, minWidth: 0 },
  errorText: { color: mobileTheme.colors.error, textAlign: 'center' },
  list: { paddingBottom: mobileTheme.spacing.sm },
  partialError: { gap: mobileTheme.spacing.xs, padding: mobileTheme.spacing.md },
  row: { alignItems: 'center', flexDirection: 'row', gap: mobileTheme.spacing.sm, minHeight: mobileTheme.sizes.minimumTouchTarget, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  rowPressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  screen: { backgroundColor: mobileTheme.colors.background, flex: 1 },
  separator: { backgroundColor: mobileTheme.colors.border, height: StyleSheet.hairlineWidth, marginLeft: 68 },
  state: { alignItems: 'stretch', gap: mobileTheme.spacing.sm, justifyContent: 'center', minHeight: 260, padding: mobileTheme.spacing.lg },
  username: { color: mobileTheme.colors.textMuted, fontSize: 14, lineHeight: 19 },
});
