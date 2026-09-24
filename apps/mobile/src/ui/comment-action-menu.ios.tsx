import { Button, Host, Image as SwiftUIImage, Menu } from '@expo/ui/swift-ui';
import { accessibilityLabel as nativeAccessibilityLabel, disabled as nativeDisabled } from '@expo/ui/swift-ui/modifiers';
import { StyleSheet, View } from 'react-native';
import type { CommentAction } from './comment-action';

export function CommentActionMenu({ accessibilityLabel, actions }: { accessibilityLabel: string; actions: readonly CommentAction[] }) {
  return <View style={styles.frame}><Host style={styles.frame}><Menu
    label={<SwiftUIImage size={18} systemName="ellipsis" />}
    modifiers={[nativeAccessibilityLabel(accessibilityLabel)]}
  >
    {actions.map((action) => <Button
      key={action.label}
      label={action.label}
      modifiers={[nativeDisabled(action.disabled ?? false)]}
      onPress={action.onPress}
      role={action.destructive ? 'destructive' : undefined}
      systemImage={action.systemImage as never}
    />)}
  </Menu></Host></View>;
}

const styles = StyleSheet.create({ frame: { height: 44, width: 44 } });
