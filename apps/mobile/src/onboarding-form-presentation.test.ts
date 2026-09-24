import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const form = readFileSync(fileURLToPath(new URL('./ui/onboarding-form.tsx', import.meta.url)), 'utf8');
const primitives = readFileSync(fileURLToPath(new URL('./ui/primitives.tsx', import.meta.url)), 'utf8');
const en = JSON.parse(readFileSync(fileURLToPath(new URL('../../../packages/i18n/src/locales/en.json', import.meta.url)), 'utf8')) as Record<string, string>;
const es = JSON.parse(readFileSync(fileURLToPath(new URL('../../../packages/i18n/src/locales/es.json', import.meta.url)), 'utf8')) as Record<string, string>;

test('onboarding is a compact non-dismissible native sheet with persistent account actions', () => {
  assert.match(form, /<NativeSheet/);
  assert.match(form, /compact/);
  assert.match(form, /dismissible=\{false\}/);
  assert.match(form, /title=\{i18n\.t\('onboarding\.heading'\)\}/);
  assert.match(
    form,
    /leadingAction=\{\{\s*disabled: busy && !homeRecoveryStatus,\s*label: i18n\.t\('auth\.signOut'\),\s*onPress: onSignOut,\s*\}\}/,
  );
  assert.match(
    form,
    /trailingAction=\{\{\s*disabled: busy \|\| !canSubmit,\s*label: i18n\.t\('onboarding\.completeAction'\),\s*onPress: onConfirm,\s*\}\}/,
  );
  assert.doesNotMatch(form, /<ScreenHeader/);
  assert.doesNotMatch(form, /<ActionButton[\s\S]*onPress=\{onConfirm\}/);
});

test('onboarding fields keep controlled drafts and move focus in logical order', () => {
  assert.match(primitives, /forwardRef<TextInput, TextInputProps>\(function ThemedTextInput/);
  assert.match(primitives, /ref,/);
  assert.match(form, /const usernameInput = useRef<ElementRef<typeof ThemedTextInput>>\(null\)/);
  assert.match(
    form,
    /autoComplete="name"[\s\S]*onChangeText=\{onChangeDisplayName\}[\s\S]*onSubmitEditing=\{\(\) => usernameInput\.current\?\.focus\(\)\}[\s\S]*returnKeyType="next"[\s\S]*value=\{profile\.displayName\}/,
  );
  assert.match(
    form,
    /autoComplete="username"[\s\S]*onChangeText=\{onChangeUsername\}[\s\S]*ref=\{usernameInput\}[\s\S]*returnKeyType="done"[\s\S]*value=\{profile\.usernameSuggestion\}/,
  );
  assert.match(form, /accessibilityHint=\{displayNameErrorText\}/);
  assert.match(form, /accessibilityHint=\{usernameErrorText\}/);
  assert.match(form, /accessibilityRole="alert"[\s\S]*displayNameErrorText/);
  assert.match(form, /accessibilityRole="alert"[\s\S]*usernameErrorText/);
  assert.match(form, /label=\{i18n\.t\('onboarding\.usernameReview'\)\}[\s\S]*value=\{usernameReviewed\}/);
});

test('onboarding remains keyboard and Dynamic Type safe without hiding draft errors', () => {
  assert.match(primitives, /automaticallyAdjustKeyboardInsets/);
  assert.match(primitives, /keyboardDismissMode="interactive"/);
  assert.match(primitives, /keyboardShouldPersistTaps="handled"/);
  assert.match(form, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(form, /stackChoices \? styles\.stackedChoices : null/);
  assert.match(form, /!canSubmit && !busy[\s\S]*onboarding\.requirementsPending/);
  assert.match(form, /busy \? <StatusBanner text=\{i18n\.t\('onboarding\.confirming'\)\} \/> : null/);
  assert.match(form, /\{errorText \? <StatusBanner text=\{errorText\} tone="error" \/> : null\}/);
  assert.match(primitives, /needsCompactVerticalLayout\(width, fontScale\)/);
  assert.match(primitives, /stackSheetHeader \? mobileShellStyles\.sheetHeaderStacked : null/);
  assert.match(primitives, /mobileShellStyles\.sheetHeaderStackedActions/);
  assert.match(primitives, /sheetHeaderAction:\s*\{[\s\S]*flexShrink:\s*1,[\s\S]*maxWidth:\s*'45%'/);
  assert.match(primitives, /sheetTitleStacked:\s*\{\s*flex:\s*0/);
  assert.doesNotMatch(primitives, /<Text accessibilityRole="header" numberOfLines=/);
  assert.match(form, /minHeight:\s*mobileTheme\.sizes\.minimumTouchTarget/);
  assert.doesNotMatch(form, /maxFontSizeMultiplier|allowFontScaling=\{false\}/);
});

test('activated Home recovery stays locked with localized loading, failure, and retry presentation', () => {
  assert.match(form, /homeRecoveryStatus\?: 'loading' \| 'offline' \| 'error'/);
  assert.match(form, /disabled: busy && !homeRecoveryStatus/);
  assert.match(form, /homeRecoveryStatus === 'loading'[\s\S]*onboarding\.homeLoading/);
  assert.match(form, /homeRecoveryStatus === 'offline'[\s\S]*onboarding\.homeOffline/);
  assert.match(form, /onboarding\.homeError/);
  assert.match(form, /actionLabel=\{i18n\.t\('common\.retry'\)\}[\s\S]*onAction=\{onRetryHome\}/);
});

test('onboarding toolbar actions remain concise in every supported locale', () => {
  assert.equal(en['onboarding.completeAction'], 'Complete');
  assert.equal(en['onboarding.requirementsPending'], 'Complete every profile and review item to continue.');
  assert.equal(es['onboarding.completeAction'], 'Completar');
  assert.equal(es['onboarding.requirementsPending'], 'Completa todos los datos del perfil y de la revisión para continuar.');
  assert.ok(en['onboarding.displayNameGuidance']);
  assert.ok(en['onboarding.usernameFormatGuidance']);
  assert.ok(en['onboarding.usernameReview']);
  assert.ok(es['onboarding.displayNameGuidance']);
  assert.ok(es['onboarding.usernameFormatGuidance']);
  assert.ok(es['onboarding.usernameReview']);
  assert.doesNotMatch(en['onboarding.displayNameGuidance'] ?? '', /100/);
  assert.doesNotMatch(es['onboarding.displayNameGuidance'] ?? '', /100/);
  assert.ok(en['onboarding.homeLoading']);
  assert.ok(en['onboarding.homeOffline']);
  assert.ok(en['onboarding.homeError']);
  assert.ok(es['onboarding.homeLoading']);
  assert.ok(es['onboarding.homeOffline']);
  assert.ok(es['onboarding.homeError']);
});
