import { emojiChoices, type Translator } from '@hourpaths/i18n';
import { useEffect, useMemo, useState } from 'react';
import { FlatList, Pressable, StyleSheet, View, useWindowDimensions } from 'react-native';
import { NativeSheet, ThemedText as Text, ThemedTextInput } from './primitives';
import { mobileTheme } from './tokens';

export function EmojiPicker({ i18n, onSelect, onClose, visible, selectedEmojis = [] }: {
  i18n: Translator;
  onSelect: (emoji: string) => void;
  onClose: () => void;
  visible: boolean;
  selectedEmojis?: readonly string[];
}) {
  const [query, setQuery] = useState('');
  const { width, fontScale } = useWindowDimensions();
  const [availableWidth, setAvailableWidth] = useState(width);
  useEffect(() => { if (!visible) setQuery(''); }, [visible]);
  const choices = useMemo(() => emojiChoices.map((choice) => ({ ...choice, label: i18n.t(choice.key) })), [i18n]);
  const filtered = useMemo(() => {
    const search = query.trim().toLocaleLowerCase();
    return search ? choices.filter((choice) => choice.emoji.includes(search) || choice.label.toLocaleLowerCase().includes(search)) : choices;
  }, [choices, query]);
  const cellSize = Math.max(mobileTheme.sizes.minimumTouchTarget, mobileTheme.spacing.xxl * Math.min(fontScale, 2));
  const columns = Math.max(1, Math.min(8, Math.floor(availableWidth / cellSize)));
  return <NativeSheet
    onRequestClose={onClose}
    title={i18n.t('emojiPicker.title')}
    trailingAction={{ label: i18n.t('common.done'), onPress: onClose }}
    visible={visible}
    scrollable={false}
  >
    <View style={styles.body} onLayout={(event) => setAvailableWidth(Math.max(mobileTheme.sizes.minimumTouchTarget, event.nativeEvent.layout.width - mobileTheme.spacing.md * 2))}>
      <ThemedTextInput
        accessibilityLabel={i18n.t('emojiPicker.search')}
        autoCapitalize="none"
        autoCorrect={false}
        onChangeText={setQuery}
        placeholder={i18n.t('emojiPicker.search')}
        returnKeyType="search"
        value={query}
      />
      <FlatList
        key={columns}
        data={filtered}
        extraData={selectedEmojis}
        initialNumToRender={columns * 8}
        keyboardDismissMode="on-drag"
        keyboardShouldPersistTaps="handled"
        keyExtractor={(item) => item.key}
        ListEmptyComponent={<Text style={styles.empty}>{i18n.t('emojiPicker.empty')}</Text>}
        numColumns={columns}
        renderItem={({ item }) => <Pressable
          accessibilityLabel={item.label}
          accessibilityRole="button"
          accessibilityState={{ selected: selectedEmojis.includes(item.emoji) }}
          onPress={() => onSelect(item.emoji)}
          style={({ pressed }) => [styles.cell, { width: availableWidth / columns, height: cellSize }, selectedEmojis.includes(item.emoji) && styles.selected, pressed && styles.pressed]}
        >
          <Text allowFontScaling={false} style={[styles.emoji, { fontSize: cellSize * 0.55 }]}>{item.emoji}</Text>
        </Pressable>}
        style={styles.list}
        windowSize={7}
      />
    </View>
  </NativeSheet>;
}

const styles = StyleSheet.create({
  body: { flex: 1, gap: mobileTheme.spacing.sm, padding: mobileTheme.spacing.md },
  list: { flex: 1 },
  cell: { alignItems: 'center', justifyContent: 'center', borderRadius: mobileTheme.radii.sm, borderWidth: mobileTheme.sizes.border, borderColor: 'transparent' },
  emoji: { color: mobileTheme.colors.text },
  selected: { backgroundColor: mobileTheme.colors.surfaceRaised, borderColor: mobileTheme.colors.accent },
  pressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  empty: { padding: mobileTheme.spacing.md, color: mobileTheme.colors.textMuted },
});
