import type { Translator } from '@hourpaths/i18n';
import * as Crypto from 'expo-crypto';
import { ActivityIndicator, Alert, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativePrimaryButton } from './native-primary-button';
import { ThemedText as Text } from './primitives';
import { SocialProfileAvatar } from './social-profile-avatar';
import type { SocialFollowRequestState } from './social-profile-route-presentation';
import { mobileTheme } from './tokens';

export function FollowRequestListView({
  i18n,
  onLoadMore,
  onRefresh,
  onReview,
  state,
}: {
  i18n: Translator;
  onLoadMore: () => void;
  onRefresh: () => void;
  onReview: (decision: 'accept' | 'reject', requestID: string, idempotencyKey?: string) => void;
  state: SocialFollowRequestState;
}) {
  if (state.status === 'loading') return <View accessibilityRole="progressbar" style={styles.centered}>
    <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
  </View>;
  if (state.status === 'error' && state.items.length === 0) return <View style={styles.centered}>
    <NativeContentUnavailable
      description={i18n.t('social.followRequestsUnavailable')}
      systemImage="wifi.exclamationmark"
      title={i18n.t('social.followRequests')}
    />
    <NativePrimaryButton label={i18n.t('common.retry')} onPress={onRefresh} variant="plain" />
  </View>;
  if (state.items.length === 0) return <View style={styles.centered}>
    <NativeContentUnavailable
      description={i18n.t('social.followRequestsEmptyDescription')}
      systemImage="person.crop.circle.badge.checkmark"
      title={i18n.t('social.followRequestsEmpty')}
    />
  </View>;

  return <ScrollView
    alwaysBounceVertical
    automaticallyAdjustContentInsets
    contentContainerStyle={styles.content}
    contentInsetAdjustmentBehavior="automatic"
    refreshControl={<RefreshControl onRefresh={onRefresh} refreshing={state.refreshing} tintColor={mobileTheme.colors.accent} />}
    style={styles.list}
  >
    <View style={styles.group}>
      {state.items.map((request, index) => <View key={request.id}>
        {index > 0 ? <View style={styles.separator} /> : null}
        <View style={styles.row}>
          <SocialProfileAvatar accessibilityLabel={i18n.t('social.neutralAvatarLabel')} />
          <View style={styles.identity}>
            <Text style={styles.displayName}>{request.requester.displayName}</Text>
            <Text style={styles.username}>@{request.requester.username}</Text>
          </View>
          <View style={styles.actions}>
            <NativePrimaryButton
              disabled={state.busyRequestID !== undefined}
              label={i18n.t('social.approveRequest')}
              onPress={() => onReview('accept', request.id, Crypto.randomUUID())}
            />
            <NativePrimaryButton
              disabled={state.busyRequestID !== undefined}
              label={i18n.t('social.declineRequest')}
              onPress={() => {
                const idempotencyKey = Crypto.randomUUID();
                Alert.alert(
                  i18n.t('social.declineRequest'),
                  `@${request.requester.username}`,
                  [
                    { style: 'cancel', text: i18n.t('common.cancel') },
                    { style: 'destructive', text: i18n.t('social.declineRequest'), onPress: () => onReview('reject', request.id, idempotencyKey) },
                  ],
                );
              }}
              variant="plain"
            />
          </View>
        </View>
      </View>)}
    </View>
    {state.errorKey ? <Text accessibilityRole="alert" style={styles.error}>{i18n.t(state.errorKey)}</Text> : null}
    {state.nextCursor ? <NativePrimaryButton
      disabled={state.busyRequestID !== undefined}
      label={i18n.t('social.loadMore')}
      onPress={onLoadMore}
      variant="plain"
    /> : null}
  </ScrollView>;
}

const styles = StyleSheet.create({
  actions: { alignItems: 'center', flexDirection: 'row', flexWrap: 'wrap', gap: mobileTheme.spacing.xs },
  centered: { flex: 1, justifyContent: 'center', padding: mobileTheme.spacing.lg },
  content: { paddingBottom: mobileTheme.spacing.md },
  displayName: { fontSize: 17, fontWeight: '600', lineHeight: 22 },
  error: { color: mobileTheme.colors.error, fontSize: 14, lineHeight: 19, textAlign: 'center' },
  group: { borderTopColor: mobileTheme.colors.border, borderTopWidth: StyleSheet.hairlineWidth },
  identity: { flexGrow: 1, flexShrink: 1, minWidth: 160 },
  list: { backgroundColor: mobileTheme.colors.background, flex: 1 },
  row: { alignItems: 'center', flexDirection: 'row', flexWrap: 'wrap', gap: mobileTheme.spacing.sm, minHeight: mobileTheme.sizes.minimumTouchTarget, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  separator: { backgroundColor: mobileTheme.colors.border, height: StyleSheet.hairlineWidth, marginLeft: 76 },
  username: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20 },
});
