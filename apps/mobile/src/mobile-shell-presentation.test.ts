import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  dismissManualActivityPresentation,
  ownsManualActivityPresentation,
  type ManualActivityPresentationOwner,
} from './manual-activity-presentation';
import { needsCompactVerticalLayout } from './ui/adaptive-layout';
import { mobileTheme } from './ui/tokens';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const workspaceConfig = readFileSync(fileURLToPath(new URL('../../../pnpm-workspace.yaml', import.meta.url)), 'utf8');
const layoutPath = fileURLToPath(new URL('../app/_layout.tsx', import.meta.url));
const layout = readFileSync(layoutPath, 'utf8') + readFileSync(fileURLToPath(new URL('./ui/navigation-theme.ts', import.meta.url)), 'utf8');
const tabLayoutPath = fileURLToPath(new URL('../app/(tabs)/_layout.tsx', import.meta.url));
const tabLayout = existsSync(tabLayoutPath) ? readFileSync(tabLayoutPath, 'utf8') : '';
const tabHomeLayoutPath = fileURLToPath(new URL('../app/(tabs)/home/_layout.tsx', import.meta.url));
const tabHomeLayout = existsSync(tabHomeLayoutPath) ? readFileSync(tabHomeLayoutPath, 'utf8') : '';
const tabHomePath = fileURLToPath(new URL('../app/(tabs)/home/index.tsx', import.meta.url));
const tabHome = existsSync(tabHomePath) ? readFileSync(tabHomePath, 'utf8') : '';
const pathRoutePath = fileURLToPath(new URL('../app/path/[pathID].tsx', import.meta.url));
const pathRoute = existsSync(pathRoutePath) ? readFileSync(pathRoutePath, 'utf8') : '';
const pathCreateFormPath = fileURLToPath(new URL('./ui/path-create-form.tsx', import.meta.url));
const pathCreateForm = existsSync(pathCreateFormPath) ? readFileSync(pathCreateFormPath, 'utf8') : '';
const pathDetailViewPath = fileURLToPath(new URL('./ui/path-detail-view.tsx', import.meta.url));
const pathDetailView = existsSync(pathDetailViewPath) ? readFileSync(pathDetailViewPath, 'utf8') : '';
const pathGoalManagementFormPath = fileURLToPath(new URL('./ui/path-goal-management-form.tsx', import.meta.url));
const pathGoalManagementForm = existsSync(pathGoalManagementFormPath)
  ? readFileSync(pathGoalManagementFormPath, 'utf8')
  : '';
const settingsRoutePath = fileURLToPath(new URL('../app/settings/index.tsx', import.meta.url));
const settingsRoute = existsSync(settingsRoutePath) ? readFileSync(settingsRoutePath, 'utf8') : '';
const accountRoutePath = fileURLToPath(new URL('../app/settings/account.tsx', import.meta.url));
const accountRoute = existsSync(accountRoutePath) ? readFileSync(accountRoutePath, 'utf8') : '';
const activityHistoryRoutePath = fileURLToPath(new URL('../app/path/[pathID]/history/index.tsx', import.meta.url));
const activityHistoryRoute = existsSync(activityHistoryRoutePath) ? readFileSync(activityHistoryRoutePath, 'utf8') : '';
const activityDetailRoutePath = fileURLToPath(new URL('../app/path/[pathID]/history/[activityID].tsx', import.meta.url));
const activityDetailRoute = existsSync(activityDetailRoutePath) ? readFileSync(activityDetailRoutePath, 'utf8') : '';
const settingsListPath = fileURLToPath(new URL('./ui/settings-list.tsx', import.meta.url));
const settingsList = existsSync(settingsListPath) ? readFileSync(settingsListPath, 'utf8') : '';
const settingsIconPath = fileURLToPath(new URL('./ui/settings-icon.ios.tsx', import.meta.url));
const settingsIcon = existsSync(settingsIconPath) ? readFileSync(settingsIconPath, 'utf8') : '';
const settingsIconFallback = readFileSync(fileURLToPath(new URL('./ui/settings-icon.tsx', import.meta.url)), 'utf8');
const nativeMenu = readFileSync(fileURLToPath(new URL('./ui/native-action-menu.ios.tsx', import.meta.url)), 'utf8');
const homeHeaderActionsPath = fileURLToPath(new URL('./ui/home-header-actions.ios.tsx', import.meta.url));
const homeHeaderActions = existsSync(homeHeaderActionsPath) ? readFileSync(homeHeaderActionsPath, 'utf8') : '';
const homeHeaderActionsFallback = readFileSync(fileURLToPath(new URL('./ui/home-header-actions.tsx', import.meta.url)), 'utf8');
const homeView = readFileSync(fileURLToPath(new URL('./ui/home-view.tsx', import.meta.url)), 'utf8');
const pathHeaderMenuPath = fileURLToPath(new URL('./ui/path-header-menu.ios.tsx', import.meta.url));
const pathHeaderMenu = existsSync(pathHeaderMenuPath) ? readFileSync(pathHeaderMenuPath, 'utf8') : '';
const pathHeaderMenuFallbackPath = fileURLToPath(new URL('./ui/path-header-menu.tsx', import.meta.url));
const pathHeaderMenuFallback = existsSync(pathHeaderMenuFallbackPath) ? readFileSync(pathHeaderMenuFallbackPath, 'utf8') : '';
const nativeRoutePresentation = readFileSync(fileURLToPath(new URL('./ui/native-route-presentation.tsx', import.meta.url)), 'utf8');
const primitives = readFileSync(fileURLToPath(new URL('./ui/primitives.tsx', import.meta.url)), 'utf8');
const pathCard = readFileSync(fileURLToPath(new URL('./ui/path-card.tsx', import.meta.url)), 'utf8');
const timerControl = readFileSync(fileURLToPath(new URL('./ui/timer-control.tsx', import.meta.url)), 'utf8');
const nativeTimerButtonPath = fileURLToPath(new URL('./ui/native-timer-button.ios.tsx', import.meta.url));
const nativeTimerButton = existsSync(nativeTimerButtonPath) ? readFileSync(nativeTimerButtonPath, 'utf8') : '';
const nativeTimerButtonFallback = readFileSync(fileURLToPath(new URL('./ui/native-timer-button.tsx', import.meta.url)), 'utf8');
const nativeTrackingButtonPath = fileURLToPath(new URL('./ui/native-tracking-button.ios.tsx', import.meta.url));
const nativeTrackingButton = existsSync(nativeTrackingButtonPath) ? readFileSync(nativeTrackingButtonPath, 'utf8') : '';
const nativeTrackingButtonFallbackPath = fileURLToPath(new URL('./ui/native-tracking-button.tsx', import.meta.url));
const nativeTrackingButtonFallback = existsSync(nativeTrackingButtonFallbackPath) ? readFileSync(nativeTrackingButtonFallbackPath, 'utf8') : '';
const manualActivityFormPath = fileURLToPath(new URL('./ui/manual-activity-form.tsx', import.meta.url));
const manualActivityForm = existsSync(manualActivityFormPath) ? readFileSync(manualActivityFormPath, 'utf8') : '';
const humanDurationEditorPath = fileURLToPath(new URL('./ui/human-duration-editor.tsx', import.meta.url));
const humanDurationEditor = existsSync(humanDurationEditorPath) ? readFileSync(humanDurationEditorPath, 'utf8') : '';
const manualOccurrenceFieldsPath = fileURLToPath(new URL('./ui/manual-occurrence-fields.ios.tsx', import.meta.url));
const manualOccurrenceFields = existsSync(manualOccurrenceFieldsPath) ? readFileSync(manualOccurrenceFieldsPath, 'utf8') : '';
const manualOccurrenceFieldsFallbackPath = fileURLToPath(new URL('./ui/manual-occurrence-fields.tsx', import.meta.url));
const manualOccurrenceFieldsFallback = existsSync(manualOccurrenceFieldsFallbackPath) ? readFileSync(manualOccurrenceFieldsFallbackPath, 'utf8') : '';
const onboardingFormPath = fileURLToPath(new URL('./ui/onboarding-form.tsx', import.meta.url));
const onboardingForm = existsSync(onboardingFormPath) ? readFileSync(onboardingFormPath, 'utf8') : '';
const signedOutScreenPath = fileURLToPath(new URL('./ui/signed-out-screen.tsx', import.meta.url));
const signedOutScreen = existsSync(signedOutScreenPath) ? readFileSync(signedOutScreenPath, 'utf8') : '';
const duplicateEmailRecoveryScreenPath = fileURLToPath(new URL('./ui/duplicate-email-recovery-screen.tsx', import.meta.url));
const duplicateEmailRecoveryScreen = existsSync(duplicateEmailRecoveryScreenPath)
  ? readFileSync(duplicateEmailRecoveryScreenPath, 'utf8')
  : '';
const activityHistoryViewPath = fileURLToPath(new URL('./ui/activity-history-view.tsx', import.meta.url));
const activityHistoryView = existsSync(activityHistoryViewPath) ? readFileSync(activityHistoryViewPath, 'utf8') : '';
const activityDetailViewPath = fileURLToPath(new URL('./ui/activity-detail-view.tsx', import.meta.url));
const activityDetailView = existsSync(activityDetailViewPath) ? readFileSync(activityDetailViewPath, 'utf8') : '';
const nativeChildRoutePath = fileURLToPath(new URL('./ui/native-child-route-presentation.tsx', import.meta.url));
const nativeChildRoute = existsSync(nativeChildRoutePath) ? readFileSync(nativeChildRoutePath, 'utf8') : '';
const nativePrimaryButtonPath = fileURLToPath(new URL('./ui/native-button.ios.tsx', import.meta.url));
const nativePrimaryButton = existsSync(nativePrimaryButtonPath) ? readFileSync(nativePrimaryButtonPath, 'utf8') : '';
const nativeContentUnavailablePath = fileURLToPath(new URL('./ui/native-content-unavailable.ios.tsx', import.meta.url));
const nativeContentUnavailable = existsSync(nativeContentUnavailablePath)
  ? readFileSync(nativeContentUnavailablePath, 'utf8')
  : '';
const contentUnavailableFallbackPath = fileURLToPath(new URL('./ui/native-content-unavailable.tsx', import.meta.url));
const contentUnavailableFallback = existsSync(contentUnavailableFallbackPath)
  ? readFileSync(contentUnavailableFallbackPath, 'utf8')
  : '';
const providerAuth = readFileSync(fileURLToPath(new URL('./provider-auth.ts', import.meta.url)), 'utf8');

test('mobile rows reflow for narrow viewports and accessibility text sizes', () => {
  assert.equal(needsCompactVerticalLayout(320, 1), true);
  assert.equal(needsCompactVerticalLayout(390, 1.3), true);
  assert.equal(needsCompactVerticalLayout(390, 1), false);
  assert.equal(needsCompactVerticalLayout(375, 1), false);
  assert.doesNotMatch(settingsList, /numberOfLines=/);
});

test('mobile shell uses a scalable themed safe-area layout', () => {
  assert.match(page, /import \{[^}]*ActionButton[^}]*StatusBanner[^}]*\} from '\.\.\/src\/ui\/primitives';/);
  assert.match(page, /style=\{styles\.screen\}/);
  assert.match(homeView, /contentContainerStyle=\{styles\.content\}/);
  assert.doesNotMatch(page, /<SafeAreaView style=\{\{ flex: 1, padding: 32, gap: 16 \}\}>/);
  assert.match(page, /import \{ SafeAreaView \} from 'react-native-safe-area-context';/);
  assert.match(
    page,
    /edges=\{destination \? \['left', 'right'\] : \['top', 'left', 'right', 'bottom'\]\}/,
  );
  assert.match(primitives, /import \{ SafeAreaView \} from 'react-native-safe-area-context';/);
  assert.match(nativeRoutePresentation, /import \{ SafeAreaView \} from 'react-native-safe-area-context';/);
  assert.doesNotMatch(page, /import \{[^}]*SafeAreaView[^}]*\} from 'react-native';/);
  assert.doesNotMatch(primitives, /import \{[^}]*SafeAreaView[^}]*\} from 'react-native';/);
  assert.doesNotMatch(nativeRoutePresentation, /import \{[^}]*SafeAreaView[^}]*\} from 'react-native';/);
  assert.match(primitives, /mobileTheme\.sizes\.minimumTouchTarget/);
  assert.doesNotMatch(primitives, /maxFontSizeMultiplier/);
  assert.doesNotMatch(onboardingForm, /maxFontSizeMultiplier/);
  assert.doesNotMatch(settingsList, /maxFontSizeMultiplier/);
  assert.doesNotMatch(timerControl, /maxFontSizeMultiplier/);
  assert.doesNotMatch(primitives, /allowFontScaling=\{false\}/);
  assert.match(page, /ThemedText as Text/);
  assert.match(pathCreateForm, /ThemedTextInput as TextInput/);
  const nativeImport = page.match(/import \{([^}]*)\} from 'react-native';/)?.[1] ?? '';
  assert.doesNotMatch(nativeImport, /\bText(?:Input)?\b/);
  assert.match(primitives, /export function ThemedText\(/);
  assert.match(primitives, /forwardRef<TextInput, TextInputProps>\(function ThemedTextInput/);
  assert.match(primitives, /placeholderTextColor:\s*mobileTheme\.colors\.textMuted/);
  assert.equal(mobileTheme.sizes.minimumTouchTarget, 48);
});

test('native sheets expose compact navigation chrome and fail closed for busy dismissal', () => {
  assert.match(primitives, /allowSwipeDismissal=\{dismissible\}/);
  assert.match(primitives, /onRequestClose=\{dismissible \? closeSheet : undefined\}/);
  assert.match(primitives, /<NativeSheetFrame/);
  
  assert.match(primitives, /sheetCompactContent/);
  assert.match(settingsList, /accessibilityValue=\{value \? \{ text: value \} : undefined\}/);
});

test('existing form workflows use native iOS sheet and button primitives', () => {
  assert.match(primitives, /Modal,[\s\S]*Text,[\s\S]*TextInput/);
  assert.match(primitives, /presentationStyle="pageSheet"/);
  assert.match(primitives, /automaticallyAdjustKeyboardInsets/);
  assert.match(primitives, /keyboardDismissMode="interactive"/);
  assert.match(primitives, /keyboardShouldPersistTaps="handled"/);
  assert.match(pathCreateForm, /<NativeSheet[\s\S]*visible=\{visible\}/);
  assert.match(page, /manualPathID && manualForm && manualDefaults \? <ManualActivityForm[\s\S]*onCancel=\{closeManualActivity\}/);
  assert.match(pathGoalManagementForm, /<NativeSheet[\s\S]*onRequestClose=\{\(\) =>/);
});

test('activity create and edit sheets are owned by their currently visible native routes', () => {
  const pathDetail = page.slice(
    page.indexOf('{selectedPath ? <NativeRouteSource'),
    page.indexOf('{selectedPath && activityHistoryOpen ? <NativeChildRouteSource'),
  );
  const activityDetail = page.slice(
    page.indexOf('{selectedPath && activityHistoryOpen && activeActivityID ? <NativeChildRouteSource'),
    page.indexOf('{pathCreated ?'),
  );

  assert.match(
    pathDetail,
    /ownsManualActivityPresentation\(manualActivityPresentationOwner, 'path-details'\) \? manualActivityPresentation : null/,
  );
  assert.match(
    activityDetail,
    /ownsManualActivityPresentation\(manualActivityPresentationOwner, 'activity-details'\) \? manualActivityPresentation : null/,
  );
  assert.ok(activityDetail.indexOf('<ActivityDetailView') < activityDetail.indexOf('manualActivityPresentation'));
  assert.match(
    page,
    /const manualActivityPresentation = manualPathID && manualForm && manualDefaults \? <ManualActivityForm/,
  );

  const edit = page.slice(page.indexOf('async function editSelectedPathActivity()'), page.indexOf('function mobileManualNow'));
  const create = page.slice(page.indexOf('async function openManualActivity('), page.indexOf('function changeManualOccurrence'));
  const submit = page.slice(page.indexOf('async function submitManualActivity()'), page.indexOf('function closeManualActivity'));
  const closeDetail = page.slice(page.indexOf('function closeActivityDetailRoute('), page.indexOf('async function activate('));
  assert.ok(edit.indexOf("claimManualActivityPresentation('activity-details')") < edit.indexOf('await validateSessionCredential'));
  assert.ok(create.indexOf("claimManualActivityPresentation('path-details')") < create.indexOf('await validateSessionCredential'));
  assert.doesNotMatch(submit, /claimManualActivityPresentation/);
  assert.match(closeDetail, /manualActivityPresentationOwnerRef\.current === 'activity-details'[\s\S]*resetManualActivity\(\)/);
});

test('manual activity presentation ownership survives save and clears with its opening route', () => {
  let owner: ManualActivityPresentationOwner = 'path-details';
  assert.equal(ownsManualActivityPresentation(owner, 'path-details'), true);
  assert.equal(ownsManualActivityPresentation(owner, 'activity-details'), false);
  assert.equal(dismissManualActivityPresentation(owner, 'activity-details'), owner);

  owner = 'activity-details';
  assert.equal(ownsManualActivityPresentation(owner, 'activity-details'), true);
  assert.equal(dismissManualActivityPresentation(owner, 'activity-details'), null);
});

test('Manage Path extracts a native human-duration editor and distinct review state', () => {
  assert.match(page, /<PathGoalManagementForm/);
  assert.doesNotMatch(page, /goalManagementPathID && goalManagementForm \? <NativeSheet/);
  assert.match(pathGoalManagementForm, /<PathGoalFields/);
  assert.match(pathGoalManagementForm, /review\s*\? <GoalChangeReview/);
  assert.match(pathGoalManagementForm, /<GoalConfigurationSummary/);
  assert.match(pathGoalManagementForm, /accessibilityRole="alert"/);
  assert.match(pathGoalManagementForm, /i18n\.t\('pathManage\.goalWarning'\)/);
  assert.match(pathGoalManagementForm, /i18n\.t\('pathManage\.current'\)/);
  assert.match(pathGoalManagementForm, /i18n\.t\('pathManage\.proposed'\)/);
  assert.match(pathGoalManagementForm, /onCancelReview/);
  assert.match(pathGoalManagementForm, /onConfirm/);
  assert.doesNotMatch(pathGoalManagementForm, /<Button/);
  assert.doesNotMatch(pathGoalManagementForm, /<TextInput/);
  assert.doesNotMatch(pathGoalManagementForm, /pathCreate\.targetSeconds/);
});

test('Create Path is an extracted native, progressively disclosed form', () => {
  assert.match(page, /import \{ PathCreateForm \} from '\.\.\/src\/ui\/path-create-form';/);
  assert.match(page, /<PathCreateForm[\s\S]*form=\{pathGoalForm\}[\s\S]*onCreate=/);
  assert.doesNotMatch(
    page.slice(page.indexOf('<PathCreateForm'), page.indexOf('</ScrollView>')),
    /pathRecurrences\.map/,
  );
  assert.match(pathCreateForm, /<NativeSheet/);
  assert.match(pathCreateForm, /<HumanDurationEditor/);
  assert.match(pathCreateForm, /<NativeChoicePicker/);
  assert.match(pathCreateForm, /accessibilityRole="switch"/);
  assert.match(pathCreateForm, /busy=\{busy\}/);
  assert.match(humanDurationEditor, /keyboardType="number-pad"/);
});

test('mobile routes are hosted by a localized native stack with iOS back gestures enabled', () => {
  assert.match(workspaceConfig, /'@react-navigation\/native-stack': '7\.17\.10'/);
  assert.match(workspaceConfig, /'@react-navigation\/native': '7\.3\.8'/);
  assert.match(workspaceConfig, /'@react-navigation\/elements': '2\.9\.30'/);
  assert.match(layout, /import \{ Stack \} from 'expo-router';/);
  assert.match(layout, /gestureEnabled:\s*true/);
  assert.match(layout, /headerLargeTitle:\s*true/);
  assert.match(layout, /headerBackButtonDisplayMode:\s*'minimal'/);
  assert.match(layout, /name="notifications"/);
  assert.match(layout, /name="invitations"/);
  assert.match(tabHomeLayout, /title:\s*i18n\.t\('home\.heading'\)/);
  assert.match(homeView, /<ScrollView[\s\S]*contentInsetAdjustmentBehavior="automatic"/);
  assert.match(page, /router\.push\(\{ pathname: '\/path\/\[pathID\]'/);
  assert.match(page, /<NativeRouteSource[\s\S]*pathID=\{selectedPath\.id\}/);
  assert.doesNotMatch(page, /label=\{i18n\.t\('pathDetails\.back'\)\}/);
  assert.match(pathRoute, /useNativeRoutePresentation\(pathID\)/);
  assert.match(pathRoute, /dismissNativeRoute\(pathID\)/);
  assert.match(pathRoute, /<Stack\.Screen options=\{\{ title: activePresentation\.title \}\}>/);
  assert.match(pathRoute, /<PathHeaderMenu[\s\S]*actions=\{activePresentation\.actions\}[\s\S]*\/>/);
  assert.doesNotMatch(pathRoute, /headerRight/);
  assert.doesNotMatch(pathRoute, /unstable_headerRightItems/);
  assert.match(pathHeaderMenu, /<Stack\.Toolbar placement="right">/);
  assert.match(pathHeaderMenu, /<Stack\.Toolbar\.Menu accessibilityLabel=\{accessibilityLabel\} icon="ellipsis">/);
  assert.match(pathHeaderMenu, /<Stack\.Toolbar\.MenuAction[\s\S]*onPress=\{action\.onPress\}/);
  assert.doesNotMatch(pathHeaderMenu, /asChild/);
  assert.match(pathHeaderMenuFallback, /<Stack\.Toolbar asChild placement="right">[\s\S]*<NativeActionMenu/);
  assert.doesNotMatch(pathRoute, /ActionSheetIOS/);
  assert.match(page, /actions=\{\[[\s\S]*pathDetails\.openHistory[\s\S]*pathManage\.action/);
});

test('implemented Home and Following surfaces share the native tab shell without a Stats placeholder', () => {
  assert.match(layout, /<Stack\.Screen name="index" options=\{\{ headerShown: false \}\} \/>/);
  assert.match(layout, /<Stack\.Screen name="\(tabs\)" options=\{\{ headerShown: false \}\} \/>/);
  assert.match(tabLayout, /import \{ NativeTabs \} from 'expo-router\/unstable-native-tabs';/);
  assert.match(tabLayout, /<NativeTabs[\s\S]*<NativeTabs\.Trigger name="home">/);
  assert.match(tabLayout, /<NativeTabs\.Trigger\.Label>\{i18n\.t\('home\.heading'\)\}<\/NativeTabs\.Trigger\.Label>/);
  assert.match(tabLayout, /<NativeTabs\.Trigger name="following">/);
  assert.match(tabLayout, /<NativeTabs\.Trigger\.Label>\{i18n\.t\('social\.following'\)\}<\/NativeTabs\.Trigger\.Label>/);
  assert.equal((tabLayout.match(/<NativeTabs\.Trigger name=/g) ?? []).length, 2);
  assert.doesNotMatch(tabLayout, /Stats|stats/);
  assert.match(tabHomeLayout, /<Stack\.Screen name="index" options=\{\{ title: i18n\.t\('home\.heading'\) \}\} \/>/);
  assert.match(tabHome, /export \{ HomeScreen as default \} from '\.\.\/\.\.\/index';/);
  assert.match(page, /export default function IndexRedirect\(\)[\s\S]*return <HomeScreen \/>/);
  assert.match(page, /shellDestination === 'home-tabs'[\s\S]*router\.replace\('\/\(tabs\)\/home'\)/);
});

test('Home creation, Path visibility, and account actions use native header controls', () => {
  assert.match(page, /<HomeHeaderActions/);
  assert.match(homeHeaderActions, /<Stack\.Toolbar placement="right">/);
  assert.match(homeHeaderActions, /<Stack\.Toolbar\.Button[\s\S]*icon="plus"[\s\S]*onPress=\{onCreate\}/);
  assert.match(homeHeaderActions, /<Stack\.Toolbar\.Menu[\s\S]*icon="line\.3\.horizontal\.decrease"/);
  assert.match(homeHeaderActions, /<Stack\.Toolbar\.MenuAction[\s\S]*isOn=\{mode === 'active'\}/);
  assert.match(homeHeaderActions, /<Stack\.Toolbar\.MenuAction[\s\S]*isOn=\{mode === 'archived'\}/);
  assert.match(homeHeaderActions, /<Stack\.Toolbar\.Button[\s\S]*icon="person\.crop\.circle"[\s\S]*onPress=\{onOpenSettings\}/);
  assert.match(homeHeaderActionsFallback, /<NativeActionMenu[\s\S]*accessibilityLabel=\{menuAccessibilityLabel\}/);
  assert.match(homeHeaderActionsFallback, /onModeChange\('active'\)[\s\S]*onModeChange\('archived'\)/);
  assert.doesNotMatch(homeHeaderActionsFallback, /flexWrap:\s*'wrap'/);
  assert.match(homeHeaderActionsFallback, /order === value \? '✓ ' : ''/);
  assert.match(homeHeaderActionsFallback, /minWidth:\s*mobileTheme\.sizes\.minimumTouchTarget/);
  assert.doesNotMatch(page, /auth\.signedInAs/);
  assert.doesNotMatch(page, /<SectionHeading>\{i18n\.t\(archivedPathsOpen \? 'home\.archivedPaths' : 'home\.heading'\)\}<\/SectionHeading>/);
});

test('Path detail content relies on native navigation insets without a duplicate top safe area', () => {
  assert.match(nativeRoutePresentation, /<SafeAreaView edges=\{\['left', 'right', 'bottom'\]\}/);
  assert.match(nativeRoutePresentation, /contentInsetAdjustmentBehavior="automatic"/);
  assert.match(nativeRoutePresentation, /automaticallyAdjustContentInsets/);
});

test('Path Details uses the native title and a compact smart-duration summary', () => {
  assert.match(page, /<PathDetailView/);
  assert.doesNotMatch(page, /<SectionHeading>\{selectedPath\.name\}<\/SectionHeading>/);
  assert.match(pathDetailView, /formatCompactDuration\(accumulatedSeconds, i18n\)/);
  assert.match(pathDetailView, /i18n\.t\('pathDetails\.totalTimeValue'/);
  assert.match(pathDetailView, /intervalProgress \|\| overallProgress/);
  assert.match(pathDetailView, /<NativePrimaryButton/);
  assert.match(pathDetailView, /systemImage="plus"/);
  assert.match(pathDetailView, /<StatusBanner[^>]*common\.loading/);
  assert.doesNotMatch(pathDetailView, /numberOfLines=/);
  assert.doesNotMatch(pathDetailView, /maxFontSizeMultiplier/);
});

test('activity history and details use nested native routes with accessible list presentation', () => {
  assert.match(layout, /name="path\/\[pathID\]\/history\/index"[\s\S]*title: i18n\.t\('pathDetails\.history'\)/);
  assert.match(layout, /name="path\/\[pathID\]\/history\/\[activityID\]"[\s\S]*title: i18n\.t\('pathDetails\.activityHeading'\)/);
  assert.match(page, /router\.push\(\{[\s\S]*pathname: '\/path\/\[pathID\]\/history'[\s\S]*pathID/);
  assert.match(page, /router\.push\(\{[\s\S]*pathname: '\/path\/\[pathID\]\/history\/\[activityID\]'[\s\S]*activityID[\s\S]*pathID/);
  assert.match(page, /<NativeChildRouteSource[\s\S]*activityHistoryRouteKey\(selectedPath\.id\)/);
  assert.match(page, /<ActivityHistoryView/);
  assert.match(page, /<ActivityDetailView/);
  assert.doesNotMatch(page, /activityHistoryOpen \? groupActivitiesByOccurrenceDay\(activityHistory\)\.map/);
  assert.match(activityHistoryRoute, /useLocalSearchParams<\{ pathID: string \}>/);
  assert.match(activityHistoryRoute, /const routeKey = activityHistoryRouteKey\(pathID\);[\s\S]*useNativeChildRoutePresentation\(routeKey\)/);
  assert.match(activityDetailRoute, /useLocalSearchParams<\{ activityID: string; pathID: string \}>/);
  assert.match(activityDetailRoute, /const routeKey = activityDetailRouteKey\(pathID, activityID\);[\s\S]*useNativeChildRoutePresentation\(routeKey\)/);
  assert.match(nativeChildRoute, /useSyncExternalStore/);
  assert.match(nativeChildRoute, /dismissNativeChildRoute/);
  assert.match(activityHistoryView, /<Pressable[\s\S]*accessibilityRole="button"/);
  assert.match(activityHistoryView, /minHeight:\s*mobileTheme\.sizes\.minimumTouchTarget/);
  assert.match(activityHistoryView, /groupActivitiesByOccurrenceDay\(activities\)/);
  assert.match(activityHistoryView, /timeZone: detail\.activity\.occurrenceTimeZone/);
  assert.match(activityHistoryView, /activityWasEdited\(detail\)/);
  assert.match(activityHistoryView, /import \{ needsCompactVerticalLayout \} from '\.\/adaptive-layout';/);
  assert.match(activityHistoryView, /import \{ formatCompactDuration \} from '\.\/compact-duration';/);
  assert.match(activityHistoryView, /useWindowDimensions\(\)/);
  assert.match(activityHistoryView, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(activityHistoryView, /formatCompactDuration\(detail\.activity\.durationSeconds, i18n\)/);
  assert.match(
    activityHistoryView,
    /i18n\.date\(retainedCalendarDayDate\(day\.localDate\), \{[\s\S]*timeZone: retainedCalendarDayTimeZone/,
  );
  assert.match(activityHistoryView, /styles\.rowCompact/);
  assert.match(activityHistoryView, /styles\.metadataCompact/);
  assert.match(activityHistoryView, /<ThemedText style=\{styles\.duration\}>\{duration\}<\/ThemedText>/);
  assert.match(
    activityHistoryView,
    /accessibilityLabel=\{i18n\.t\(hideParticipant[\s\S]*'pathDetails\.memberHistoryRowEdited'[\s\S]*'pathDetails\.historyRowEdited'[\s\S]*duration,[\s\S]*participant:[\s\S]*time,/,
  );
  assert.doesNotMatch(
    activityHistoryView,
    /accessibilityLabel=\{i18n\.t\(edited \? 'pathDetails\.historyRowEdited'[\s\S]*seconds:/,
  );
  assert.match(activityHistoryView, /accessibilityRole="header" style=\{styles\.dayHeading\}/);
  assert.match(
    activityHistoryView,
    /<NativeContentUnavailable[\s\S]*description=\{i18n\.t\('pathDetails\.historyEmptyExplanation'\)\}[\s\S]*systemImage="clock\.arrow\.circlepath"[\s\S]*title=\{i18n\.t\('pathDetails\.historyEmpty'\)\}/,
  );
  assert.match(activityHistoryView, /<StatusBanner[\s\S]*tone="error"/);
  assert.doesNotMatch(activityDetailView, /Stack\.Toolbar/);
  assert.match(
    activityDetailView,
    /<NativePrimaryButton[\s\S]*disabled=\{deletionBusy \|\| manualBusy\}[\s\S]*label=\{i18n\.t\('pathDetails\.edit'\)\}[\s\S]*onPress=\{onEdit\}[\s\S]*systemImage="pencil"/,
  );
  assert.match(
    activityDetailView,
    /label=\{i18n\.t\('pathDetails\.participantLabel'\)\}[\s\S]*value=\{participantLabel \?\? activity\.activity\.participantId\}/,
  );
  assert.match(activityDetailView, /import \{ needsCompactVerticalLayout \} from '\.\/adaptive-layout';/);
  assert.match(activityDetailView, /import \{ formatCompactDuration \} from '\.\/compact-duration';/);
  assert.match(activityDetailView, /useWindowDimensions\(\)/);
  assert.match(activityDetailView, /styles\.detailRowCompact/);
  assert.match(activityDetailView, /value \? styles\.label : styles\.valueStandalone/);
  assert.match(activityDetailView, /formatCompactDuration\(activity\.activity\.durationSeconds, i18n\)/);
  assert.match(activityDetailView, /formatCompactDuration\(revision\.durationSeconds, i18n\)/);
  assert.match(activityDetailView, /manualBusy \? <StatusBanner text=\{i18n\.t\('common\.loading'\)\} \/>/);
  assert.match(activityDetailView, /const revisionPresentation = pagedCollectionPresentation/);
  assert.match(activityDetailView, /revisionPresentation\.showEmpty/);
  assert.match(
    activityDetailView,
    /selectable style=\{\[styles\.value, compact \? styles\.valueCompact : null\]\}/,
  );
  assert.match(
    activityDetailView,
    /i18n\.t\('pathDetails\.revisionChanged', \{[\s\S]*date: revisionTimestamp\(revision\.replacedAt\)/,
  );
  assert.match(nativeContentUnavailable, /<Host matchContents/);
  assert.match(nativeContentUnavailable, /<ContentUnavailableView[\s\S]*description=\{description\}[\s\S]*systemImage=\{systemImage\}[\s\S]*title=\{title\}/);
  assert.match(contentUnavailableFallback, /<Surface>[\s\S]*<SectionHeading>\{title\}<\/SectionHeading>[\s\S]*\{description\}/);
  assert.match(activityDetailView, /priorNoteForProfile\(revision, profileID\)/);
  assert.match(activityDetailView, /activityBelongsToProfile\(activity, profileID\)/);
  assert.match(activityDetailView, /<StatusBanner[\s\S]*tone="error"/);
  assert.doesNotMatch(activityHistoryView, /\bText\b.*from 'react-native'/);
  assert.doesNotMatch(activityDetailView, /\bText\b.*from 'react-native'/);
  assert.doesNotMatch(activityHistoryView, /numberOfLines=/);
  assert.doesNotMatch(activityDetailView, /numberOfLines=/);
  assert.doesNotMatch(activityHistoryView, /maxFontSizeMultiplier/);
  assert.doesNotMatch(activityDetailView, /maxFontSizeMultiplier/);
});

test('Home path cards keep identity, progress, and quick tracking in one accessible surface', () => {
  assert.match(page, /<PathCard/);
  assert.match(page, /name=\{path\.name\}/);
  assert.match(page, /accumulatedText=\{state \? i18n\.t\('path\.progress\.accumulatedCompact'/);
  assert.match(page, /<TimerControl/);
  assert.match(pathCard, /accessibilityRole="button"/);
  assert.match(pathCard, /accessibilityLabel=\{name\}/);
  assert.match(pathCard, /mobileTheme\.colors\.surface/);
  assert.match(pathCard, /borderBottomColor:\s*mobileTheme\.colors\.separator/);
  assert.match(pathCard, /<SettingsIcon systemName="chevron\.right" variant="disclosure" \/>/);
  assert.match(pathCard, /minHeight:\s*mobileTheme\.sizes\.minimumTouchTarget/);
  assert.doesNotMatch(pathCard, /markerFrame|styles\.marker/);
  assert.match(pathCard, /flexDirection:\s*'row'/);
  assert.match(pathCard, /minHeight:\s*80/);
  assert.match(pathCard, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(pathCard, /styles\.accessibilityRow/);
  assert.doesNotMatch(pathCard, /maxFontSizeMultiplier/);
  assert.doesNotMatch(pathCard, />›</);
  assert.match(settingsIcon, /Image as SwiftUIImage/);
  assert.match(settingsIcon, /variant === 'disclosure'/);
  assert.match(settingsIcon, /accessibilityElementsHidden/);
  assert.match(settingsIconFallback, /variant === 'disclosure'/);
});

test('running Paths temporarily lead Home in a localized accessible section', () => {
  assert.match(page, /import \{ organizeHomePaths,[^\n]+\} from '\.\.\/src\/ui\/home-organization';/);
  assert.match(page, /const homeSections = ownedHomeDestination/);
  assert.match(page, /organizeHomePaths\(ownedHomeDestination\.profile\.paths, ownedHomeDestination\.profile\.timers, homePreferences, homeFilter\)/);
  assert.match(page, /homeSections\.active\.length > 0/);
  assert.match(page, /key: 'active',[\s\S]*title: i18n\.t\('home\.activeTimersHeading'\)/);
  assert.match(page, /homeSections\.active\.map\(renderHomePath\)/);
  assert.match(page, /homeSections\.trackable\.map\(renderHomePath\)/);
  assert.doesNotMatch(page, /destination\.profile\.paths\.map\(\(path\) =>/);
});

test('timer presentation exposes running, busy, and failure state without changing orchestration', () => {
  assert.match(timerControl, /import \{ NativeTrackingButton \} from '\.\/native-tracking-button';/);
  assert.match(timerControl, /<NativeTrackingButton[\s\S]*running=\{running\}/);
  assert.doesNotMatch(timerControl, /<Button/);
  assert.match(nativeTrackingButton, /systemName=\{running \? 'stop\.fill' : 'play\.fill'\}/);
  assert.match(nativeTrackingButton, /accessibilityLabel\(label\)/);
  assert.match(nativeTrackingButton, /nativeDisabled\(busy\)/);
  assert.match(nativeTrackingButtonFallback, /accessibilityState=\{\{ busy, disabled: busy \}\}/);
  assert.match(nativeTrackingButtonFallback, /const buttonSize = Math\.max\(48, 40 \+ 8 \* fontScale\)/);
  assert.doesNotMatch(nativeTrackingButtonFallback, /allowFontScaling=\{false\}[^>]*style=\{styles\.elapsed\}/);
  assert.match(nativeTimerButton, /buttonStyle\('borderedProminent'\)/);
  assert.match(nativeTimerButton, /frame\(\{ maxWidth: Number\.POSITIVE_INFINITY, minHeight: mobileTheme\.sizes\.minimumTouchTarget \}\)/);
  assert.match(nativeTimerButton, /nativeDisabled\(busy\)/);
  assert.match(timerControl, /accessibilityLabel=\{elapsedAccessibilityLabel\}[\s\S]*style=\{styles\.elapsed\}[\s\S]*\{elapsedText\}/);
  assert.doesNotMatch(nativeTrackingButton, /elapsedText|Animated\.Text/);
  assert.match(timerControl, /accessibilityRole="alert"/);
  assert.doesNotMatch(timerControl, /position:\s*'absolute'/);
  assert.doesNotMatch(timerControl, /<Pressable/);
  assert.match(page, /onPress=\{\(\) => void toggleTimer\(path\.id\)\}/);
  assert.match(page, /timerBusy\[path\.id\]/);
  assert.match(page, /timerErrorKeys\[path\.id\]/);
});

test('manual activity create and edit use an extracted native, accessible form', () => {
  assert.match(page, /import \{ ManualActivityForm \} from '\.\.\/src\/ui\/manual-activity-form';/);
  assert.match(page, /<ManualActivityForm[\s\S]*form=\{manualForm\}[\s\S]*onSave=\{\(\) => void submitManualActivity\(\)\}/);
  assert.match(page, /const \[manualSavedVersion, setManualSavedVersion\] = useState<number \| null>\(null\)/);
  assert.match(page, /setManualSavedVersion\(result\.version\)/);
  assert.match(page, /activityVersion=\{manualSavedVersion \?\? undefined\}/);
  assert.match(page, /key=\{manualActivity[\s\S]*manualActivity\.id[\s\S]*manualActivity\.version[\s\S]*manualPathID/);
  assert.doesNotMatch(page, /<TextInput accessibilityLabel=\{i18n\.t\('activity\.date'\)\}/);
  assert.match(manualActivityForm, /<NativeSheet[\s\S]*<ManualOccurrenceFields/);
  assert.match(manualActivityForm, /title=\{i18n\.t\(editing \? 'activity\.editHeading' : 'activity\.addHeading'\)\}/);
  assert.doesNotMatch(manualActivityForm, /<Surface>/);
  assert.match(manualActivityForm, /activity\.timeZone/);
  assert.match(manualActivityForm, /activity\.notePrivacy/);
  assert.match(manualActivityForm, /activity\.saving/);
  assert.match(manualActivityForm, /busy \? <StatusBanner text=\{i18n\.t\('activity\.saving'\)\} \/>/);
  assert.match(manualActivityForm, /trailingAction=\{\{[\s\S]*disabled: busy[\s\S]*activity\.save/);
  assert.doesNotMatch(manualActivityForm, /<ActionButton/);
  assert.match(manualActivityForm, /accessibilityLiveRegion="polite"/);
  assert.match(manualActivityForm, /<StatusBanner[\s\S]*tone="error"/);
  assert.match(manualActivityForm, /multiline/);
  assert.match(manualActivityForm, /useState\(Boolean\(note\)\)/);
  assert.match(manualActivityForm, /AccessibilityInfo\.isReduceMotionEnabled\(\)/);
  assert.match(manualActivityForm, /reduceMotionChanged/);
  assert.match(manualActivityForm, /animationType=\{reduceMotion === false \? 'slide' : 'none'\}/);
  assert.match(manualActivityForm, /noteExpanded \? <View/);
  assert.match(manualActivityForm, /activity\.(?:add|hide)Note/);
  assert.match(
    manualActivityForm,
    /<HumanDurationEditor[\s\S]*compact[\s\S]*label=\{i18n\.t\('activity\.durationValue'\)\}[\s\S]*onChange=\{onChangeDuration\}[\s\S]*activity\.(?:hideNote|addNote)[\s\S]*noteExpanded \? <View[\s\S]*activity\.notePrivacy/,
  );
  assert.doesNotMatch(manualActivityForm, /activity\.durationSeconds/);
  assert.match(pathCreateForm, /<HumanDurationEditor/);
  assert.match(humanDurationEditor, /keyboardType="number-pad"/);
  assert.doesNotMatch(manualActivityForm, /maxFontSizeMultiplier/);
  assert.match(manualOccurrenceFields, /displayedComponents=\{\['date'\]\}/);
  assert.match(manualOccurrenceFields, /displayedComponents=\{\['hourAndMinute'\]\}/);
  assert.match(manualOccurrenceFields, /environment\('timeZone', 'UTC'\)/);
  assert.match(manualOccurrenceFields, /title=\{i18n\.t\('activity\.date'\)\}/);
  assert.match(manualOccurrenceFields, /title=\{i18n\.t\('activity\.startTime'\)\}/);
  assert.doesNotMatch(manualOccurrenceFields, /activity\.exactStartTime/);
  assert.doesNotMatch(manualOccurrenceFields, /TextInput/);
  assert.match(manualOccurrenceFieldsFallback, /<ThemedTextInput/);
});

test('signed-out, loading, offline, and error states use explicit accessible presentation', () => {
  assert.match(page, /export default function IndexRedirect\(\)[\s\S]*return <HomeScreen \/>/);
  assert.match(page, /accountShellDestination\(\{[\s\S]*destinationKind: destination\?\.kind \?\? null,[\s\S]*pathname,[\s\S]*ready/);
  assert.match(page, /shellDestination === 'home-tabs'[\s\S]*router\.replace\('\/\(tabs\)\/home'\)/);
  assert.match(page, /shellDestination === 'account-entry'[\s\S]*router\.replace\('\/'\)/);
  assert.match(homeView, /presentation\.kind === 'loading'[\s\S]*text=\{i18n\.t\('home\.loading'\)\}/);
  assert.match(page, /const homeNotice[\s\S]*accessState === 'authenticated_offline' && !errorKey && !offlineStatusDismissed/);
  assert.match(page, /const homeNotice[\s\S]*<StatusBanner[\s\S]*onAction=[\s\S]*text=\{i18n\.t\('auth\.offline'\)\}/);
  assert.match(page, /<HomeView[\s\S]*notice=\{<>[\s\S]*\{homeNotice\}/);
  assert.match(page, /if \(next\.retryable\)[\s\S]*setErrorKey\(null\)/);
  assert.match(page, /shouldTransitionMobileSessionForFeatureFailure\(failure\)/);
  assert.doesNotMatch(page, /announceForAccessibility\(i18n\.t\('auth\.offline'\)\)/);
  assert.match(page, /setDestination\(nextDestination\);[\s\S]*setAccessState\('authenticated_online'\);[\s\S]*setErrorKey\(null\)/);
  assert.match(primitives, /import \{[\s\S]*ActivityIndicator,/);
  assert.match(primitives, /tone = 'loading'/);
  assert.match(primitives, /tone\?: 'loading' \| 'offline' \| 'error'/);
  assert.match(primitives, /accessibilityRole=\{tone === 'loading' \? 'progressbar' : 'alert'\}/);
  assert.match(primitives, /tone === 'loading' \? <ActivityIndicator/);
  assert.match(primitives, /onAction && actionLabel \? <NativeButton/);
  assert.match(page, /<SignedOutScreen[\s\S]*providerBusy=\{providerSignIn\.busy\}/);
  assert.match(page, /providerDiscoveryFailed=\{providerSignIn\.discoveryFailed\}/);
  assert.match(page, /providerReady=\{providerSignIn\.ready\}/);
  assert.match(page, /onRetry=\{providerSignIn\.retry\}/);
  assert.match(page, /onSignIn=\{\(\) => void beginSignIn\(\)\}/);
  assert.match(signedOutScreen, /contentInsetAdjustmentBehavior="never"/);
  assert.match(signedOutScreen, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(signedOutScreen, /contentContainerStyle=\{\[styles\.content, accessibilityLayout && styles\.accessibilityContent\]\}/);
  assert.match(signedOutScreen, /flexGrow:\s*1/);
  assert.match(signedOutScreen, /compact=\{accessibilityLayout\}/);
  assert.match(signedOutScreen, /title=\{i18n\.t\(accessibilityLayout \? 'app\.title' : 'auth\.welcomeHeading'\)\}/);
  assert.match(signedOutScreen, /accessibilityLayout \? null : <Text/);
  assert.match(signedOutScreen, /accessibilityLayout \? primaryAction : null/);
  assert.match(signedOutScreen, /accessibilityLayout \? null : primaryAction/);
  assert.match(primitives, /compact \? styles\.compactTitle : undefined/);
  assert.match(signedOutScreen, /<ActivityIndicator/);
  assert.match(signedOutScreen, /accessibilityRole="progressbar"/);
  assert.match(signedOutScreen, /const recoveryErrorText = providerDiscoveryFailed/);
  assert.match(signedOutScreen, /sessionExpired[\s\S]*auth\.expiredProviderUnavailable/);
  assert.equal((signedOutScreen.match(/<StatusBanner/g) ?? []).length, 1);
  assert.match(page, /sessionExpired=\{errorKey === 'errors\.sessionExpired'\}/);
  assert.match(signedOutScreen, /label=\{i18n\.t\('common\.retry'\)\}/);
  assert.match(signedOutScreen, /label=\{i18n\.t\('auth\.signIn'\)\}/);
  assert.doesNotMatch(signedOutScreen, /label=\{i18n\.t\(providerBusy \?/);
  assert.match(signedOutScreen, /<NativePrimaryButton[\s\S]*fullWidth/);
  assert.match(providerAuth, /discoveryFailed:\s*boolean/);
  assert.match(providerAuth, /busy:\s*boolean/);
  assert.match(providerAuth, /retry:\s*\(\) => void/);
  assert.match(providerAuth, /setDiscoveryFailed\(next === null\)/);
  assert.match(providerAuth, /if \(busy \|\| !request \|\| !discovery\) return/);
  assert.match(providerAuth, /setBusy\(true\)/);
  assert.match(providerAuth, /setBusy\(false\)/);
});

test('reopening Notifications preserves last-good history until a successful first-page response', () => {
  const openStart = page.indexOf('async function openNotifications(');
  const openEnd = page.indexOf('async function refreshNotifications()', openStart);
  const openNotifications = page.slice(openStart, openEnd);
  assert.doesNotMatch(openNotifications, /setNotificationHistory\(empty\)/);
  assert.match(openNotifications, /mergeNotificationHistoryPage\(empty, page, ''\)/);
  assert.match(openNotifications, /setNotificationHistory\(history\)/);
});

test('Home omits an empty progress wrapper when a Path has no configured progress', () => {
  assert.match(page, /progress=\{currentIntervalProgress \|\| progress \? <>/);
});

test('Home exposes account and sign-out through a native Settings entry point', () => {
  assert.match(page, /<HomeHeaderActions[\s\S]*settingsAccessibilityLabel=\{i18n\.t\('settings\.openLabel'\)\}/);
  assert.match(homeHeaderActions, /icon="person\.crop\.circle"/);
  assert.match(page, /router\.push\('\/settings'\)/);
  assert.doesNotMatch(page, /visible=\{settingsOpen\}/);
  assert.match(layout, /name="settings\/index"[\s\S]*i18n\.t\('settings\.heading'\)/);
  assert.match(layout, /name="settings\/account"[\s\S]*i18n\.t\('settings\.account'\)/);
  assert.match(settingsRoute, /router\.push\('\/settings\/account'\)/);
  assert.match(accountRoute, /Alert\.alert/);
  assert.match(accountRoute, /\.signOut\(resolution\)/);
  assert.match(settingsList, /minHeight:\s*52/);
  assert.match(settingsList, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(settingsList, /<SettingsIcon systemName="chevron\.right" variant="disclosure" \/>/);
  assert.doesNotMatch(settingsList, />›<\/Text>/);
  assert.match(settingsList, /accessibilityRole="button"/);
  assert.doesNotMatch(
    page,
    /destination && destination\.kind !== 'home' && destination\.kind !== 'duplicate_email_recovery' \? <ActionButton/,
  );
  assert.match(page, /<OnboardingForm[\s\S]*onSignOut=\{\(\) => void clearSession\(\)\}/);
  assert.match(page, /<DuplicateEmailRecoveryScreen[\s\S]*onReturnToSignIn=\{\(\) => void clearSession\(\)\}/);
});

test('duplicate-account recovery keeps both choices reachable at accessibility text sizes', () => {
  assert.match(
    duplicateEmailRecoveryScreen,
    /<ScrollView[\s\S]*automaticallyAdjustContentInsets[\s\S]*contentContainerStyle=\{styles\.content\}[\s\S]*contentInsetAdjustmentBehavior="automatic"/,
  );
  assert.match(duplicateEmailRecoveryScreen, /screen:\s*\{[\s\S]*flex:\s*1/);
  assert.match(duplicateEmailRecoveryScreen, /content:\s*\{[\s\S]*paddingBottom:\s*mobileTheme\.spacing\.xxl/);
  assert.match(
    duplicateEmailRecoveryScreen,
    /<NativePrimaryButton[\s\S]*duplicateEmailRecovery\.returnToSignIn[\s\S]*<ActionButton[\s\S]*duplicateEmailRecovery\.decline/,
  );
  assert.match(
    duplicateEmailRecoveryScreen,
    /declining \? <StatusBanner text=\{i18n\.t\('duplicateEmailRecovery\.declining'\)\} \/>/,
  );
});

test('Settings follows the native grouped account hierarchy used by StuffStash', () => {
  assert.match(settingsRoute, /<SettingsIcon[\s\S]*systemName="person\.crop\.circle"/);
  assert.match(settingsRoute, /settings\.account\.openLabel', \{ email: presentation\.email, name \}/);
  assert.match(settingsRoute, /context=\{presentation\.email\}/);
  assert.match(settingsList, /context\?: string/);
  assert.match(settingsList, /icon\?: ReactNode/);
  assert.match(settingsList, /styles\.rowIconFrame/);
  assert.match(settingsIcon, /Image as SwiftUIImage/);
  assert.match(settingsIcon, /systemName=\{systemName\}/);
  assert.match(accountRoute, /<SettingsValueRow[\s\S]*<SettingsSeparator \/>[\s\S]*<SettingsValueRow/);
  assert.match(settingsList, /accessibilityRole="header"/);
  assert.match(settingsList, /minHeight:\s*52/);
  assert.doesNotMatch(settingsList, /borderWidth:\s*mobileTheme\.sizes\.border/);
});

test('onboarding presentation is extracted into an accessible native form', () => {
  assert.match(page, /import \{ OnboardingForm \} from '\.\.\/src\/ui\/onboarding-form';/);
  assert.match(page, /<OnboardingForm[\s\S]*profile=\{destination\.profile\}/);
  assert.doesNotMatch(page, /ready && destination\?\.kind === 'onboarding' \? <ScrollView/);
  assert.match(onboardingForm, /<NativeSheet/);
  assert.match(onboardingForm, /title=\{i18n\.t\('onboarding\.heading'\)\}/);
  assert.match(onboardingForm, /<SectionHeading>/);
  assert.match(onboardingForm, /<Surface>/);
  assert.match(onboardingForm, /<StatusBanner/);
  assert.match(onboardingForm, /<ActionButton/);
  assert.match(onboardingForm, /<Switch[\s\S]*accessibilityLabel=/);
  assert.match(onboardingForm, /selected=\{profile\.profileVisibility === 'public'\}/);
  assert.match(onboardingForm, /selected=\{profile\.profileVisibility === 'private'\}/);
  assert.match(onboardingForm, /returnKeyType="next"/);
  assert.match(onboardingForm, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(onboardingForm, /minHeight:\s*mobileTheme\.sizes\.minimumTouchTarget/);
  assert.doesNotMatch(onboardingForm, /#[0-9a-fA-F]{3,8}/);
});
