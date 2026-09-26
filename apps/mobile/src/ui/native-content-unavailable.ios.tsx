import { NativeHost as Host } from './native-host';
import { type ComponentProps } from 'react';
import { ContentUnavailableView, } from '@expo/ui/swift-ui';
import { StyleSheet, View } from 'react-native';

type SystemImage = ComponentProps<typeof ContentUnavailableView>['systemImage'];

export function NativeContentUnavailable({
  description,
  systemImage,
  title,
}: {
  description: string;
  systemImage: SystemImage;
  title: string;
}) {
  return <View style={styles.container}>
    <Host matchContents colorScheme="dark" style={styles.host}>
      <ContentUnavailableView
        description={description}
        systemImage={systemImage}
        title={title}
      />
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'stretch',
    minHeight: 180,
    width: '100%',
  },
  host: {
    minHeight: 180,
    width: '100%',
  },
});
