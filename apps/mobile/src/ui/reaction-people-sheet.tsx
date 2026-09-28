import type { PublicProfile } from '@hourpaths/api-client';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, StyleSheet, View } from 'react-native';
import { NativeButton } from './native-button';
import { NativeSheet, StatusBanner, ThemedText as Text } from './primitives';
import { SettingsIcon } from './settings-icon';
import { SocialProfileAvatar } from './social-profile-avatar';
import { profileAccessibilityLabel } from './social-profile-presentation';
import { mobileTheme } from './tokens';

export type ReactionPeopleLoader = (eventId: string, emoji: string, cursor?: string) => Promise<{ items: PublicProfile[]; nextCursor: string }>;

export function ReactionPeopleSheet({ i18n, emoji, eventId, loadPeople, onClose, onOpenProfile, onToggle, reacted, busy }: {
  i18n: Translator;
  emoji: string;
  eventId: string;
  loadPeople: ReactionPeopleLoader;
  onClose: () => void;
  onOpenProfile: (username: string) => void;
  onToggle: () => void;
  reacted: boolean;
  busy: boolean;
}) {
  const [items, setItems] = useState<PublicProfile[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const lifetime = useRef(0);
  const pending = useRef(false);
  const loader = useRef(loadPeople);
  loader.current = loadPeople;
  const lastCursor = useRef<string | undefined>(undefined);

  async function load(cursor?: string) {
    if (pending.current) return;
    const ticket = lifetime.current;
    pending.current = true;
    lastCursor.current = cursor;
    setLoading(true);
    setFailed(false);
    try {
      const page = await loader.current(eventId, emoji, cursor);
      if (ticket !== lifetime.current) return;
      setItems((previous) => {
        const unique = new Map((cursor ? previous : []).map((profile) => [profile.id, profile]));
        for (const profile of page.items) unique.set(profile.id, profile);
        return [...unique.values()];
      });
      setNextCursor(page.nextCursor);
    } catch {
      if (ticket === lifetime.current) setFailed(true);
    } finally {
      if (ticket === lifetime.current) { pending.current = false; setLoading(false); }
    }
  }

  useEffect(() => {
    lifetime.current += 1;
    pending.current = false;
    setItems([]);
    setNextCursor('');
    void load();
    return () => { lifetime.current += 1; pending.current = false; };
  }, [eventId, emoji, reacted]);

  return <NativeSheet
    onRequestClose={onClose}
    title={i18n.t('reactions.peopleTitle', { emoji })}
    trailingAction={{ label: i18n.t('common.done'), onPress: onClose }}
    visible
    scrollable={false}
  >
    <View style={styles.body}>
      <NativeButton
        busy={busy}
        fullWidth
        label={i18n.t(reacted ? 'reactions.remove' : 'reactions.add', { emoji })}
        onPress={onToggle}
        variant="secondary"
      />
      {failed ? <View style={styles.error}>
        <StatusBanner text={i18n.t('errors.temporarilyUnavailable')} tone="error" />
        <NativeButton label={i18n.t('common.retry')} onPress={() => void load(lastCursor.current)} variant="quiet" />
      </View> : null}
      <FlatList
        data={items}
        keyExtractor={(profile) => profile.id}
        ItemSeparatorComponent={() => <View style={styles.separator} />}
        ListEmptyComponent={!loading && !failed ? <Text style={styles.empty}>{i18n.t('reactions.peopleEmpty')}</Text> : null}
        ListFooterComponent={loading ? <ActivityIndicator accessibilityLabel={i18n.t('common.loading')} color={mobileTheme.colors.accent} style={styles.loading} /> : nextCursor && !failed ? <NativeButton label={i18n.t('social.loadMore')} onPress={() => void load(nextCursor)} variant="quiet" /> : null}
        renderItem={({ item }) => <Pressable
          accessibilityLabel={profileAccessibilityLabel(item)}
          accessibilityRole="button"
          onPress={() => { onClose(); onOpenProfile(item.username); }}
          style={({ pressed }) => [styles.row, pressed && styles.pressed]}
        >
          <SocialProfileAvatar accessibilityLabel={i18n.t('social.neutralAvatarLabel')} profilePictureURL={item.profilePictureUrl} />
          <View style={styles.identity}>
            <Text style={styles.name}>{item.displayName}</Text>
            <Text style={styles.username}>@{item.username}</Text>
          </View>
          <SettingsIcon systemName="chevron.right" variant="disclosure" />
        </Pressable>}
        style={styles.list}
      />
    </View>
  </NativeSheet>;
}

const styles = StyleSheet.create({
  body: { flex: 1, gap: mobileTheme.spacing.md, padding: mobileTheme.spacing.md },
  list: { flex: 1 },
  row: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.sm, minHeight: mobileTheme.sizes.minimumTouchTarget, paddingVertical: mobileTheme.spacing.sm, paddingHorizontal: mobileTheme.spacing.xs, borderRadius: mobileTheme.radii.md },
  identity: { flex: 1, minWidth: 0 },
  name: { ...mobileTheme.typography.body, fontWeight: '600' },
  username: { ...mobileTheme.typography.caption, color: mobileTheme.colors.textMuted },
  pressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  separator: { height: StyleSheet.hairlineWidth, backgroundColor: mobileTheme.colors.separator },
  empty: { padding: mobileTheme.spacing.md, color: mobileTheme.colors.textMuted },
  loading: { padding: mobileTheme.spacing.md },
  error: { gap: mobileTheme.spacing.xs },
});
