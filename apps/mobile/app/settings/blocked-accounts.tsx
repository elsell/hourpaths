import * as Crypto from 'expo-crypto';
import { getLocales } from 'expo-localization';
import { router, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Alert } from 'react-native';
import {
  mergeBlockedAccountPage,
  type BlockedAccount,
  type BlockedAccountPage,
} from '@hourpaths/client-core';
import { ownsBlockedAccountLoad } from '../../src/blocked-account-operation';
import { createDeviceTranslator } from '../../src/i18n';
import { queueSettingsJourneyBootstrap, scheduleSettingsJourneyBootstrap } from '../../src/settings-journey-route-recovery';
import { ownsSettingsRouteState, settingsRouteOwnerDecision } from '../../src/settings-route-state';
import { useSettingsJourneyRouteAncestry } from '../../src/use-settings-journey-route-ancestry';
import {
  BlockedAccountListView,
  type BlockedAccountListState,
} from '../../src/ui/blocked-account-list-view';
import { SettingsJourneyRecoveryView } from '../../src/ui/settings-journey-recovery-view';
import { useSettingsJourneyRecovery } from '../../src/ui/settings-journey-route-presentation';
import { useUserBlockingRoutePresentation } from '../../src/ui/user-blocking-route-presentation';

const i18n = createDeviceTranslator(getLocales);
const emptyPage: BlockedAccountPage = { items: [], nextCursor: '' };
const intent = { kind: 'blocked-accounts', routeKey: 'settings:blocked-accounts' } as const;
type UnblockIntent = Readonly<{ idempotencyKey: string; sessionKey: string; userID: string }>;

export default function BlockedAccountsScreen() {
  const publishedPresentation = useUserBlockingRoutePresentation();
  const publishedRecovery = useSettingsJourneyRecovery(intent.routeKey);
  const lastRecoverySessionKey = useRef<string | undefined>(undefined);
  const incomingSessionKey = publishedPresentation?.sessionKey ?? publishedRecovery?.sessionKey;
  const ownerDecision = settingsRouteOwnerDecision(lastRecoverySessionKey.current, incomingSessionKey);
  if (ownerDecision === 'claim') lastRecoverySessionKey.current = incomingSessionKey;
  const ownerReplaced = ownerDecision === 'replacement';
  const presentation = ownerReplaced ? undefined : publishedPresentation;
  const recovery = ownerReplaced ? undefined : publishedRecovery;
  useSettingsJourneyRouteAncestry(intent);
  useEffect(() => {
    if (ownerReplaced) router.replace('/(tabs)/home');
  }, [ownerReplaced]);
  const page = useRef<BlockedAccountPage>(emptyPage);
  const operation = useRef(0);
  const unblockAdmission = useRef<UnblockIntent | null>(null);
  const unblockRetry = useRef<UnblockIntent | null>(null);
  const hasFocused = useRef(false);
  const [stateOwnerKey, setStateOwnerKey] = useState<string | null>(null);
  const activeOwnerKey = useRef<string | null>(presentation?.sessionKey ?? null);
  activeOwnerKey.current = presentation?.sessionKey ?? null;
  const [state, setState] = useState<BlockedAccountListState>({
    items: [], loadingMore: false, refreshing: false, status: 'loading',
  });

  const load = useCallback(async (cursor = '', refreshing = false) => {
    if (!presentation || unblockAdmission.current) return;
    const ownedPresentation = presentation;
    const ownedSessionKey = ownedPresentation.sessionKey;
    const ownedOperation = ++operation.current;
    setState((current) => ({
      ...current,
      errorKey: undefined,
      loadingMore: Boolean(cursor),
      refreshing,
      status: cursor || (refreshing && current.items.length > 0) ? 'ready' : 'loading',
    }));
    try {
      const incoming = await ownedPresentation.listBlockedAccounts(cursor || undefined);
      const next = cursor ? mergeBlockedAccountPage(page.current, incoming, cursor) : incoming;
      if (!ownsBlockedAccountLoad(ownedOperation, operation.current, Boolean(unblockAdmission.current)) ||
        activeOwnerKey.current !== ownedSessionKey) return;
      page.current = next;
      setState({
        items: next.items,
        loadingMore: false,
        nextCursor: next.nextCursor || undefined,
        refreshing: false,
        status: 'ready',
      });
    } catch {
      if (!ownsBlockedAccountLoad(ownedOperation, operation.current, Boolean(unblockAdmission.current)) ||
        activeOwnerKey.current !== ownedSessionKey) return;
      setState((current) => ({
        ...current,
        errorKey: 'blocking.unavailableDescription',
        loadingMore: false,
        refreshing: false,
        status: current.items.length > 0 ? 'ready' : 'error',
      }));
    }
  }, [presentation]);

  useEffect(() => {
    if (!presentation) return;
    setStateOwnerKey(presentation.sessionKey);
    page.current = emptyPage;
    ++operation.current;
    unblockAdmission.current = null;
    unblockRetry.current = null;
    setState({ items: [], loadingMore: false, refreshing: false, status: 'loading' });
    void load();
  }, [presentation?.sessionKey]);
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleSettingsJourneyBootstrap(intent, () => router.replace('/(tabs)/home'), lastRecoverySessionKey.current);
  }, [presentation, recovery]);
  useFocusEffect(useCallback(() => {
    if (hasFocused.current) void load('', true);
    else hasFocused.current = true;
  }, [load]));

  async function unblock(reviewedIntent: UnblockIntent) {
    if (!presentation || unblockAdmission.current || presentation.sessionKey !== reviewedIntent.sessionKey ||
      activeOwnerKey.current !== reviewedIntent.sessionKey) return;
    const account = page.current.items.find(({ identity }) => identity.userId === reviewedIntent.userID);
    if (!account) return;
    const ownedPresentation = presentation;
    const { identity } = account;
    unblockAdmission.current = reviewedIntent;
    unblockRetry.current = reviewedIntent;
    ++operation.current;
    setState((current) => ({ ...current, busyUserId: identity.userId, errorKey: undefined }));
    try {
      const result = await ownedPresentation.unblockUser(identity.userId, reviewedIntent.idempotencyKey);
      if (activeOwnerKey.current !== reviewedIntent.sessionKey || unblockAdmission.current !== reviewedIntent) return;
      if (result.blocked || result.target.userId !== identity.userId) throw new Error('mismatched_unblock_target');
      page.current = {
        ...page.current,
        items: page.current.items.filter(({ identity: candidate }) => candidate.userId !== identity.userId),
      };
      if (activeOwnerKey.current !== reviewedIntent.sessionKey || unblockAdmission.current !== reviewedIntent) return;
      setState((current) => ({
        ...current,
        busyUserId: undefined,
        items: current.items.filter(({ identity: candidate }) => candidate.userId !== identity.userId),
      }));
      await AccessibilityInfo.announceForAccessibility(
        i18n.t('blocking.unblockedSuccess', { username: identity.username }),
      );
      if (activeOwnerKey.current === reviewedIntent.sessionKey && unblockAdmission.current === reviewedIntent) {
        unblockRetry.current = null;
      }
    } catch {
      if (activeOwnerKey.current !== reviewedIntent.sessionKey || unblockAdmission.current !== reviewedIntent) return;
      setState((current) => ({
        ...current,
        busyUserId: undefined,
        errorKey: 'blocking.unblockUnavailable',
      }));
    } finally {
      if (activeOwnerKey.current === reviewedIntent.sessionKey && unblockAdmission.current === reviewedIntent) {
        unblockAdmission.current = null;
      }
    }
  }

  function confirmUnblock(account: BlockedAccount) {
    if (!presentation || !page.current.items.some(({ identity }) => identity.userId === account.identity.userId)) return;
    const retry = unblockRetry.current;
    const reviewedIntent: UnblockIntent = retry?.sessionKey === presentation.sessionKey &&
      retry.userID === account.identity.userId
      ? retry
      : {
          idempotencyKey: Crypto.randomUUID(),
          sessionKey: presentation.sessionKey,
          userID: account.identity.userId,
        };
    Alert.alert(
      i18n.t('blocking.unblockConfirmTitle', { username: account.identity.username }),
      i18n.t('blocking.unblockConfirmDescription'),
      [
        { style: 'cancel', text: i18n.t('common.cancel') },
        {
          onPress: () => void unblock(reviewedIntent),
          text: i18n.t('blocking.unblock'),
        },
      ],
    );
  }

  if (!presentation) return <SettingsJourneyRecoveryView
    i18n={i18n}
    intent={intent}
    onHome={() => router.replace('/(tabs)/home')}
    onRetry={recovery?.retry ?? (() => {
      queueSettingsJourneyBootstrap(intent, lastRecoverySessionKey.current);
      router.replace('/(tabs)/home');
    })}
    state={recovery?.state ?? 'loading'}
  />;
  const ownsState = ownsSettingsRouteState(stateOwnerKey, presentation.sessionKey);
  const ownedState = ownsState ? state : { items: [], loadingMore: false, refreshing: false, status: 'loading' as const };

  return <BlockedAccountListView
    i18n={i18n}
    onLoadMore={() => { if (page.current.nextCursor) void load(page.current.nextCursor); }}
    onRefresh={() => void load('', true)}
    onRetry={() => void load(ownedState.items.length > 0 ? page.current.nextCursor : '')}
    onUnblock={confirmUnblock}
    state={ownedState}
  />;
}
