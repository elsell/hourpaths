import { SafeAreaView } from 'react-native-safe-area-context';
import type { ReactNode } from 'react';
import { Text, View, useWindowDimensions } from 'react-native';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { NativeSheetAction } from './native-sheet-action';
import { mobileShellStyles } from './shell-styles';

type Action = { disabled?: boolean; label: string; onPress: () => void };
export function NativeSheetFrame({ children, title, leadingAction, trailingAction }: { children: ReactNode; title?: string; leadingAction?: Action; trailingAction?: Action }) {
  const { fontScale, width } = useWindowDimensions();
  const stackSheetHeader = needsCompactVerticalLayout(width, fontScale);
  const leadingHeaderAction = <View style={mobileShellStyles.sheetHeaderAction}>
    {leadingAction ? <NativeSheetAction
      disabled={leadingAction.disabled}
      label={leadingAction.label}
      onPress={leadingAction.onPress}
    /> : null}
  </View>;
  const trailingHeaderAction = <View style={[mobileShellStyles.sheetHeaderAction, mobileShellStyles.sheetHeaderTrailing]}>
    {trailingAction ? <NativeSheetAction
      disabled={trailingAction.disabled}
      label={trailingAction.label}
      onPress={trailingAction.onPress}
    /> : null}
  </View>;

  return <SafeAreaView edges={['top']} style={mobileShellStyles.sheet}>
      {title ? <View style={[mobileShellStyles.sheetHeader, stackSheetHeader ? mobileShellStyles.sheetHeaderStacked : null]}>
        {stackSheetHeader ? <>
          <Text accessibilityRole="header" style={[mobileShellStyles.sheetTitle, mobileShellStyles.sheetTitleStacked]}>{title}</Text>
          <View style={mobileShellStyles.sheetHeaderStackedActions}>
            {leadingHeaderAction}
            {trailingHeaderAction}
          </View>
        </> : <>
          {leadingHeaderAction}
          <Text accessibilityRole="header" style={mobileShellStyles.sheetTitle}>{title}</Text>
          {trailingHeaderAction}
        </>}
      </View> : null}
    {children}
  </SafeAreaView>;
}
