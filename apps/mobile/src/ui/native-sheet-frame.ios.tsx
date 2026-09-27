import type { ReactNode } from 'react';
import { NavigationContainer, NavigationIndependentTree } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { NativeSheetAction } from './native-sheet-action';
import { nativeStackOptions, navigationTheme } from './navigation-theme';

const SheetStack = createNativeStackNavigator();
type Action = { disabled?: boolean; label: string; onPress: () => void; systemImage?: string };
export function NativeSheetFrame({ children, title, leadingAction, trailingAction }: { children: ReactNode; title?: string; leadingAction?: Action; trailingAction?: Action }) {
  return <NavigationIndependentTree>
    <NavigationContainer theme={navigationTheme}>
      <SheetStack.Navigator screenOptions={{ ...nativeStackOptions, headerLargeTitle: false }}>
        <SheetStack.Screen name="task" options={{
          title,
          headerShown: Boolean(title),
          headerLeft: leadingAction ? () => <NativeSheetAction disabled={leadingAction.disabled} label={leadingAction.label} onPress={leadingAction.onPress} systemImage={leadingAction.systemImage} /> : undefined,
          headerRight: trailingAction ? () => <NativeSheetAction disabled={trailingAction.disabled} label={trailingAction.label} onPress={trailingAction.onPress} systemImage={trailingAction.systemImage} /> : undefined,
        }}>{() => children}</SheetStack.Screen>
      </SheetStack.Navigator>
    </NavigationContainer>
  </NavigationIndependentTree>;
}
