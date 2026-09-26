import { DarkTheme } from '@react-navigation/native';
import { mobileTheme } from './tokens';

// Navigation owns the system material; the theme supplies its semantic colors.
export const navigationTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    primary: mobileTheme.colors.accent,
    background: mobileTheme.colors.background,
    card: mobileTheme.colors.background,
    text: mobileTheme.colors.text,
    border: mobileTheme.colors.border,
    notification: mobileTheme.colors.accent,
  },
};

export const nativeStackOptions = {
  animation: 'default',
  contentStyle: { backgroundColor: mobileTheme.colors.background },
  gestureEnabled: true,
  headerBackButtonDisplayMode: 'minimal',
  headerLargeTitle: true,
  headerShadowVisible: false,
  headerTintColor: mobileTheme.colors.accent,
  headerTitleStyle: { color: mobileTheme.colors.text },
  statusBarStyle: 'light',
} as const;
