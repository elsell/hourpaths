import type { BlockedAccount } from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { ActivityIndicator, RefreshControl, ScrollView, StyleSheet, View, useWindowDimensions } from 'react-native';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { NativeButton } from './native-button';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeSheetAction } from './native-sheet-action';
import { ThemedText as Text } from './primitives';
import { SocialProfileAvatar } from './social-profile-avatar';
import { profileAccessibilityLabel } from './social-profile-presentation';
import { mobileTheme } from './tokens';

export type BlockedAccountListState = Readonly<{
  busyUserId?: string;
  errorKey?: MessageKey;
  items: readonly BlockedAccount[];
  loadingMore: boolean;
  nextCursor?: string;
  refreshing: boolean;
  status: 'loading' | 'ready' | 'error';
}>;

function BlockedAccountRow({
  account,
  busy,
  i18n,
  onUnblock,
}: {
  account: BlockedAccount;
  busy: boolean;
  i18n: Translator;
  onUnblock: () => void;
}) {
  const { identity } = account;
  const { fontScale, width } = useWindowDimensions();
  const stacked = needsCompactVerticalLayout(width, fontScale);
  return <View style={[styles.row, stacked ? styles.rowStacked : null]}>
    <View
      accessibilityLabel={profileAccessibilityLabel(identity)}
      accessible
      style={[styles.identityGroup, stacked ? styles.identityGroupStacked : null]}
    >
      <SocialProfileAvatar
        accessibilityLabel={i18n.t('social.neutralAvatarLabel')}
        profilePictureURL={identity.profilePictureUrl}
        size={44}
      />
      <View style={styles.identity}>
        <Text style={styles.displayName}>{identity.displayName}</Text>
        <Text style={styles.username}>@{identity.username}</Text>
      </View>
    </View>
    <NativeSheetAction
      disabled={busy}
      label={i18n.t('blocking.unblock')}
      onPress={onUnblock}
    />
  </View>;
}

function FullState({ i18n, onRetry, state }: {
  i18n: Translator;
  onRetry: () => void;
  state: BlockedAccountListState;
}) {
  if (state.status === 'loading') return <View
    accessibilityLabel={i18n.t('blocking.loading')}
    accessibilityRole="progressbar"
    style={styles.centered}
  >
    <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
    <Text style={styles.secondary}>{i18n.t('blocking.loading')}</Text>
  </View>;
  if (state.status === 'error') return <View style={styles.centered}>
    <NativeContentUnavailable
      description={i18n.t('blocking.unavailableDescription')}
      systemImage="wifi.exclamationmark"
      title={i18n.t('blocking.unavailableHeading')}
    />
    <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
  </View>;
  return <View style={styles.centered}>
    <NativeContentUnavailable
      description={i18n.t('blocking.emptyDescription')}
      systemImage="hand.raised.fill"
      title={i18n.t('blocking.emptyHeading')}
    />
  </View>;
}

export function BlockedAccountListView({
  i18n,
  onLoadMore,
  onRefresh,
  onRetry,
  onUnblock,
  state,
}: {
  i18n: Translator;
  onLoadMore: () => void;
  onRefresh: () => void;
  onRetry: () => void;
  onUnblock: (account: BlockedAccount) => void;
  state: BlockedAccountListState;
}) {
  const hasItems = state.items.length > 0;
  return <ScrollView
    alwaysBounceVertical
    automaticallyAdjustContentInsets
    contentContainerStyle={[styles.content, !hasItems ? styles.emptyContent : null]}
    contentInsetAdjustmentBehavior="automatic"
    refreshControl={<RefreshControl
      colors={[mobileTheme.colors.accent]}
      onRefresh={onRefresh}
      refreshing={state.refreshing}
      tintColor={mobileTheme.colors.accent}
    />}
    style={styles.list}
  >
    {hasItems ? <View style={styles.group}>
      {state.items.map((account, index) => <View key={account.identity.userId}>
        {index > 0 ? <View style={styles.separator} /> : null}
        <BlockedAccountRow
          account={account}
          busy={state.busyUserId !== undefined}
          i18n={i18n}
          onUnblock={() => onUnblock(account)}
        />
      </View>)}
    </View> : <FullState i18n={i18n} onRetry={onRetry} state={state} />}
    {state.errorKey && hasItems ? <View accessibilityRole="alert" style={styles.partialError}>
      <Text style={styles.errorText}>{i18n.t(state.errorKey)}</Text>
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : null}
    {state.nextCursor ? <NativeButton busy={state.loadingMore} disabled={state.loadingMore || state.busyUserId !== undefined} label={i18n.t(state.loadingMore ? 'blocking.loadingMore' : 'blocking.loadMore')} onPress={onLoadMore} variant="quiet" /> : null}
  </ScrollView>;
}

const styles = StyleSheet.create({
  centered: { flex: 1, gap: mobileTheme.spacing.sm, justifyContent: 'center', minHeight: 300 },
  content: { gap: mobileTheme.spacing.sm, padding: mobileTheme.spacing.md },
  displayName: { fontSize: 17, fontWeight: '600', lineHeight: 22 },
  emptyContent: { flexGrow: 1 },
  errorText: { color: mobileTheme.colors.error, fontSize: 14, lineHeight: 19, textAlign: 'center' },
  group: { backgroundColor: mobileTheme.colors.surface, borderRadius: mobileTheme.radii.md, overflow: 'hidden' },
  identity: { flex: 1, minWidth: 0 },
  identityGroup: { alignItems: 'center', flex: 1, flexDirection: 'row', gap: mobileTheme.spacing.sm, minWidth: 0 },
  identityGroupStacked: { alignSelf: 'stretch' },
  list: { backgroundColor: mobileTheme.colors.background, flex: 1 },
  partialError: { gap: mobileTheme.spacing.xs, padding: mobileTheme.spacing.md },
  row: { alignItems: 'center', flexDirection: 'row', gap: mobileTheme.spacing.sm, minHeight: 64, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.xxs },
  rowStacked: { alignItems: 'stretch', flexDirection: 'column', paddingVertical: mobileTheme.spacing.sm },
  secondary: { color: mobileTheme.colors.textMuted, textAlign: 'center' },
  separator: { backgroundColor: mobileTheme.colors.border, height: StyleSheet.hairlineWidth, marginLeft: 68 },
  username: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20 },
});
