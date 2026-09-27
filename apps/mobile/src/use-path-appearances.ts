import { useEffect, useRef, useState } from 'react';
import * as Crypto from 'expo-crypto';
import { createPathAppearancePort } from '@hourpaths/api-client';
import { createPathAppearanceStore, defaultPathAppearance, type PathAppearance, type PathColor } from '@hourpaths/client-core';
import { subscribeNativeAppActive } from './push-notifications-native';

export function usePathAppearances(apiURL: string, accountID: string | undefined, token: string | undefined, pathIDs: string) {
  const [, redraw] = useState(0);
  const credential = useRef({ accountID, token });
  credential.current = { accountID, token };
  const current = useRef<{ accountID: string; store: ReturnType<typeof createPathAppearanceStore> } | null>(null);
  const colorIntent = useRef<object | null>(null);
  const [editor, setEditor] = useState<{ accountID: string; pathID: string; revision: number; draft?: PathAppearance; inline?: boolean; busy: boolean; failed: boolean } | null>(null);
  useEffect(() => {
    if (!accountID) return;
    const store = createPathAppearanceStore(createPathAppearancePort(apiURL, () => credential.current.accountID === accountID ? credential.current.token ?? null : null), () => Crypto.randomUUID(), () => redraw((n) => n + 1));
    current.current = { accountID, store };
    redraw((n) => n + 1);
    return () => { store.dispose(); if (current.current?.store === store) current.current = null; };
  }, [accountID, apiURL]);
  useEffect(() => {
    const owned = current.current;
    if (!accountID || owned?.accountID !== accountID) return;
    const refresh = () => { void owned.store.refresh(pathIDs.split('\u0000').filter(Boolean)); };
    refresh();
    return subscribeNativeAppActive(refresh);
  }, [accountID, pathIDs]);
  const entry = current.current;
  const owned = entry && entry.accountID === accountID ? entry.store : undefined;
  return {
    appearance: (id: string) => owned?.appearance(id) ?? defaultPathAppearance(id),
    failed: owned?.hasFailures() ?? false,
    retry: () => { void owned?.refresh(pathIDs.split('\u0000').filter(Boolean)); },
    editor: editor && editor.accountID === accountID && pathIDs.split('\u0000').includes(editor.pathID) ? editor : null,
    async selectColor(pathID: string, color: PathColor) {
      if (!accountID || !owned || colorIntent.current || editor?.busy) return;
      const intent = {};
      colorIntent.current = intent;
      try {
        if (!owned.isLoaded(pathID)) await owned.refresh([pathID]);
        if (current.current?.store !== owned || !owned.isLoaded(pathID)) return;
        const draft = { ...owned.appearance(pathID), color };
        const revision = owned.revision(pathID);
        const pendingEditor = { accountID, pathID, revision, draft, inline: true, busy: true, failed: false };
        setEditor(pendingEditor);
        try {
          await owned.save(pathID, draft, revision);
          if (current.current?.store === owned) setEditor((value) => value === pendingEditor ? null : value);
        } catch {
          if (current.current?.store === owned) setEditor((value) => value === pendingEditor ? { accountID, pathID, revision: owned.revision(pathID), draft, inline: false, busy: false, failed: true } : value);
        }
      } finally {
        if (colorIntent.current === intent) colorIntent.current = null;
      }
    },
    async open(pathID: string) {
      if (!accountID || !owned) return;
      if (!owned.isLoaded(pathID)) await owned.refresh([pathID]);
      if (current.current?.store === owned && owned.isLoaded(pathID)) setEditor({ accountID, pathID, revision: owned.revision(pathID), busy: false, failed: false });
    },
    close: () => setEditor(null),
    async save(appearance: PathAppearance) {
      if (!owned || !editor || editor.busy || editor.accountID !== accountID) return;
      const intent = editor;
      setEditor({ ...intent, busy: true, failed: false });
      try {
        if (await owned.save(intent.pathID, appearance, intent.revision)) setEditor((value) => value?.accountID === intent.accountID && value.pathID === intent.pathID ? null : value);
      } catch {
        setEditor((value) => value?.accountID === intent.accountID && value.pathID === intent.pathID ? { ...value, revision: owned.revision(intent.pathID), busy: false, failed: true } : value);
      }
    },
  };
}
