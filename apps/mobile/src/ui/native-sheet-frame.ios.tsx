import type { ReactNode } from 'react';
import { NavigationContainer, NavigationIndependentTree } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { NativeSheetAction } from './native-sheet-action';
import { nativeStackOptions, navigationTheme } from './navigation-theme';

const SheetStack = createNativeStackNavigator();
type Action = { disabled?: boolean; label: string; onPress: () => void };
export function NativeSheetFrame({ children, title, leadingAction, trailingAction }: { children: ReactNode; title?: string; leadingAction?: Action; trailingAction?: Action }) {
  return <NavigationIndependentTree>
    <NavigationContainer theme={navigationTheme}>
      <SheetStack.Navigator screenOptions={{ ...nativeStackOptions, headerLargeTitle: false }}>
        <SheetStack.Screen name="task" options={{
          title,
          headerShown: Boolean(title),
          headerLeft: leadingAction ? () => <NativeSheetAction {...leadingAction} /> : undefined,
          headerRight: trailingAction ? () => <NativeSheetAction {...trailingAction} /> : undefined,
        }}>{() => children}</SheetStack.Screen>
      </SheetStack.Navigator>
    </NavigationContainer>
  </NavigationIndependentTree>;
}
