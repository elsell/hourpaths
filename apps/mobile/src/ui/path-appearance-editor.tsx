import { useState } from 'react';
import { StyleSheet, View } from 'react-native';
import type { Translator } from '@hourpaths/i18n';
import { NativeChoicePicker } from './native-choice-picker';
import { PathCard } from './path-card';
import { pathColorKeys, type PathAppearance } from './path-appearance';
import { NativeSheet, StatusBanner, ThemedText as Text, ThemedTextInput as TextInput } from './primitives';
import { mobileTheme } from './tokens';
import { validPathEmoji } from '../path-appearance-validation';

// Mount for one editing intent; persistence and its account ownership belong to
// the caller. The same preview and picker are used on iOS and Android.
export function PathAppearanceEditor({ initial, name, i18n, busy, errorText, onCancel, onSave }: {
  initial: PathAppearance;
  name: string;
  i18n: Translator;
  busy: boolean;
  errorText?: string;
  onCancel: () => void;
  onSave: (appearance: PathAppearance) => void;
}) {
  const [draft, setDraft] = useState(initial);
  const validEmoji = validPathEmoji(draft.emoji.trim());
  return <NativeSheet
    animationType="none"
    compact
    dismissible={!busy}
    leadingAction={{ disabled: busy, label: i18n.t('common.cancel'), onPress: onCancel }}
    onRequestClose={onCancel}
    title={i18n.t('home.appearance.title')}
    trailingAction={{ disabled: busy || !validEmoji, label: i18n.t(errorText ? 'common.retry' : 'common.save'), onPress: () => onSave({ ...draft, emoji: draft.emoji.trim() }) }}
    visible
  >
    <Text accessibilityRole="header">{i18n.t('home.appearance.preview')}</Text>
    <PathCard appearance={{ ...draft, emoji: validEmoji ? draft.emoji.trim() : initial.emoji }} name={name} />
    <NativeChoicePicker
      accessibilityLabel={i18n.t('home.appearance.color')}
      choices={pathColorKeys.map((value) => ({ value, label: i18n.t(`home.appearance.color.${value}`) }))}
      disabled={busy}
      label={i18n.t('home.appearance.color')}
      onChange={(color) => setDraft((current) => ({ ...current, color }))}
      value={draft.color}
    />
    <View style={styles.field}>
      <Text>{i18n.t('home.appearance.emoji')}</Text>
      <TextInput
        accessibilityLabel={i18n.t('home.appearance.emoji')}
        autoCapitalize="none"
        autoCorrect={false}
        editable={!busy}
        maxLength={64}
        onChangeText={(emoji) => setDraft((current) => ({ ...current, emoji }))}
        placeholder={i18n.t('home.appearance.emojiHint')}
        style={styles.emoji}
        value={draft.emoji}
      />
      {!validEmoji ? <Text accessibilityRole="alert">{i18n.t('home.appearance.invalidEmoji')}</Text> : null}
    </View>
    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
  </NativeSheet>;
}
const styles = StyleSheet.create({
  field: { gap: mobileTheme.spacing.xs },
  emoji: { fontSize: 30, minHeight: mobileTheme.sizes.minimumTouchTarget },
});
