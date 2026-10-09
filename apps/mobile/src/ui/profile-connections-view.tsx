import { createFollowerRemovalOwner, ProfileConnectionsFailure, type ConnectionDirection, type ConnectionPerson, type ProfileConnectionsRepository } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import * as Crypto from 'expo-crypto';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ActivityIndicator, Alert, AppState, StyleSheet, View } from 'react-native';
import { NativeRouteScreen } from './native-route-presentation';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeButton } from './native-button';
import { ProfileRow } from './social-profile-search-view';
import { ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

export function ProfileConnectionsView({ i18n, repository, viewerId, username, direction, canRemove, onOpen, onChanged }: {
  i18n: Translator; repository: ProfileConnectionsRepository; viewerId: string; username: string; direction: ConnectionDirection;
  canRemove: boolean; onOpen(username: string): void; onChanged(): void;
}) {
  const repo = useRef(repository); repo.current = repository;
  const current = useRef(true), generation = useRef(0), controller = useRef<AbortController | null>(null), loading = useRef(false);
  const [owner] = useState(() => createFollowerRemovalOwner(viewerId, Crypto.randomUUID));
  const [people, setPeople] = useState<ConnectionPerson[]>([]), [next, setNext] = useState<string | null>(null);
  const [busy, setBusy] = useState(false), [removing, setRemoving] = useState<string | null>(null), [error, setError] = useState<'hidden' | 'unavailable' | null>(null), [removeError, setRemoveError] = useState(false);
  const admitted = useRef(false);
  const removeAllowed = useRef(canRemove); removeAllowed.current = canRemove;
  useEffect(() => () => { current.current = false; generation.current++; controller.current?.abort(); owner.cancel(); }, [owner]);
  const load = useCallback(async (cursor?: string) => {
    if (!current.current || (cursor && loading.current)) return;
    controller.current?.abort(); const request = new AbortController(); controller.current = request;
    const ticket = ++generation.current; loading.current = true; setBusy(true); setError(null);
    try {
      const page = await repo.current.list(username, direction, cursor, request.signal);
      if (!current.current || ticket !== generation.current) return;
      setPeople(previous => [...new Map([...(cursor ? previous : []), ...page.items].map(person => [person.id, person])).values()]); setNext(page.next);
    } catch (cause) {
      if (!current.current || ticket !== generation.current || request.signal.aborted) return;
      setPeople([]); setNext(null); setError(cause instanceof ProfileConnectionsFailure && (cause.kind === 'hidden' || cause.kind === 'rejected') ? 'hidden' : 'unavailable');
    } finally { if (current.current && ticket === generation.current) { loading.current = false; setBusy(false); } }
  }, [username, direction]);
  useFocusEffect(useCallback(() => {
    void load();
    const subscription = AppState.addEventListener('change', state => { if (state === 'active') void load(); });
    return () => subscription.remove();
  }, [load]));
  async function remove(person: ConnectionPerson) {
    if (!removeAllowed.current || !current.current || admitted.current) return;
    admitted.current = true;
    setRemoving(person.id); setRemoveError(false);
    const result = await owner.remove(viewerId, person.id, (id, key) => repo.current.remove(id, key));
    if (!current.current) return;
    if (result.kind === 'applied') { setPeople(items => items.filter(item => item.id !== person.id)); onChanged(); void load(); }
    else if (result.kind === 'failed') setRemoveError(true);
    admitted.current = false; setRemoving(null);
  }
  const title = i18n.t(direction === 'followers' ? 'social.profileFollowers' : 'social.profileFollowing');
  return <NativeRouteScreen onRefresh={() => void load()} refreshing={busy}>
    {busy && !people.length && <ActivityIndicator accessibilityLabel={i18n.t('studio.loading')} color={mobileTheme.colors.accent} />}
    {error && <><NativeContentUnavailable title={title} description={i18n.t(error === 'hidden' ? 'connections.hidden' : 'connections.loadFailed')} systemImage={error === 'hidden' ? 'person.2' : 'wifi.exclamationmark'} /><NativeButton label={i18n.t('common.retry')} onPress={() => void load()} variant="quiet" /></>}
    {!busy && !error && !people.length && <NativeContentUnavailable title={title} description={i18n.t(direction === 'followers' ? 'connections.noFollowers' : 'connections.noFollowing')} systemImage="person.2" />}
    {removeError && <Text accessibilityRole="alert">{i18n.t('connections.removeFailed')}</Text>}
    {people.map(person => <View key={person.id} style={styles.row}><View style={styles.identity}><ProfileRow i18n={i18n} profile={{ displayName: person.name, username: person.username, profilePictureUrl: person.picture ?? undefined }} onOpen={() => onOpen(person.username)} /></View>
      {canRemove && direction === 'followers' && <NativeButton accessibilityLabel={i18n.t('connections.removeLabel', { name: person.name })} label={i18n.t('connections.remove')} busy={removing === person.id} disabled={removing !== null} variant="secondary" onPress={() => Alert.alert(i18n.t('connections.confirmTitle', { name: person.name }), i18n.t('connections.confirmDescription'), [
        { text: i18n.t('common.cancel'), style: 'cancel', onPress: () => { if (!admitted.current) owner.cancel(); } },
        { text: i18n.t('connections.remove'), style: 'destructive', onPress: () => void remove(person) },
      ])} />}
    </View>)}
    {next && !error && <NativeButton label={i18n.t('studio.social.morePeople')} disabled={busy} onPress={() => void load(next)} variant="quiet" />}
  </NativeRouteScreen>;
}
const styles = StyleSheet.create({ row: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: mobileTheme.spacing.xs, paddingVertical: mobileTheme.spacing.xs }, identity: { flex: 1, minWidth: 180 } });
