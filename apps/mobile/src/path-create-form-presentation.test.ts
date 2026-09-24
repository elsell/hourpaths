import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const form = readFileSync(
  fileURLToPath(new URL('./ui/path-create-form.tsx', import.meta.url)),
  'utf8',
);
const durationEditor = readFileSync(
  fileURLToPath(new URL('./ui/human-duration-editor.tsx', import.meta.url)),
  'utf8',
);
const choicePicker = readFileSync(
  fileURLToPath(new URL('./ui/native-choice-picker.tsx', import.meta.url)),
  'utf8',
);

test('Create Path uses one compact adaptive sheet hierarchy with persistent native actions', () => {
  assert.match(form, /<NativeSheet[\s\S]*compact[\s\S]*dismissible=\{!busy\}/);
  assert.match(form, /title=\{i18n\.t\('pathCreate\.heading'\)\}/);
  assert.match(form, /leadingAction=\{\{[\s\S]*disabled: busy[\s\S]*common\.cancel[\s\S]*onPress: onCancel/);
  assert.match(form, /trailingAction=\{\{[\s\S]*disabled: busy \|\| !canCreate[\s\S]*label: i18n\.t\(errorText[\s\S]*pathCreate\.retry[\s\S]*pathCreate\.submit[\s\S]*onPress: onCreate/);
  assert.match(form, /<PathGoalFields[\s\S]*compact/);
  assert.doesNotMatch(form, /<SectionHeading|<ActionButton|home\.empty\.explanation/);
  assert.doesNotMatch(form, /numberOfLines|maxFontSizeMultiplier|allowFontScaling=\{false\}/);
});

test('a trimmed name is the only presentation gate while optional goals stay progressive', () => {
  assert.match(form, /const canCreate = name\.trim\(\)\.length > 0/);
  assert.match(form, /form\.intervalEnabled \? <View/);
  assert.match(form, /form\.overallEnabled \? <View/);
  assert.match(form, /onUpdate\(\{ intervalEnabled \}\)/);
  assert.match(form, /onUpdate\(\{ overallEnabled \}\)/);
  assert.match(form, /editable=\{!busy\}/);
  assert.match(form, /disabled=\{busy\}/);
});

test('visibility stays controlled and the native fallback announces its selection', () => {
  assert.match(form, /visibility: PathVisibility/);
  assert.match(form, /visibilityOptions: readonly PathVisibility\[\]/);
  assert.match(form, /onVisibilityChange: \(visibility: PathVisibility\) => void/);
  assert.match(form, /visibilityOptions\.map\(\(option\) => \(\{[\s\S]*pathVisibility\.option\.\$\{option\}/);
  assert.match(form, /onChange=\{onVisibilityChange\}/);
  assert.match(form, /value=\{visibility\}/);
  assert.match(choicePicker, /accessibilityRole="radiogroup"/);
  assert.match(choicePicker, /<Text style=\{styles\.groupLabel\}>\{label\}<\/Text>/);
  assert.match(choicePicker, /accessibilityRole="radio"/);
  assert.match(choicePicker, /accessibilityState=\{\{ disabled, selected \}\}/);
  assert.match(choicePicker, /if \(!selected\) onChange\(choice\.value\)/);
});

test('busy and retry states preserve the controlled draft and expose accessible status', () => {
  assert.match(form, /const requestClose = \(\) => \{[\s\S]*if \(!busy\) \{[\s\S]*onCancel\(\)/);
  assert.match(form, /onRequestClose=\{requestClose\}/);
  assert.match(form, /busy \? <StatusBanner[\s\S]*pathCreate\.submitting[\s\S]*tone="loading"/);
  assert.match(form, /errorText \? <StatusBanner text=\{errorText\} tone="error"/);
  assert.match(form, /value=\{name\}/);
  assert.match(form, /form=\{form\}/);
});

test('Create Path follows the system reduced-motion preference', () => {
  assert.match(form, /AccessibilityInfo\.isReduceMotionEnabled\(\)/);
  assert.match(form, /reduceMotionChanged/);
  assert.match(form, /animationType=\{reduceMotion === false \? 'slide' : 'none'\}/);
});

test('every Create Path number pad has an operable Done keyboard path', () => {
  assert.match(form, /<InputAccessoryView nativeID=\{keyboardAccessoryID\}>/);
  assert.match(form, /accessibilityRole="button"[\s\S]*onPress=\{Keyboard\.dismiss\}[\s\S]*common\.done/);
  assert.match(form, /inputAccessoryViewID=\{keyboardAccessoryID\}/);
  assert.match(durationEditor, /inputAccessoryViewID\?: string/);
  assert.match(durationEditor, /onSubmitEditing\?: \(\) => void/);
  assert.match(durationEditor, /inputAccessoryViewID=\{inputAccessoryViewID\}/);
  assert.match(durationEditor, /returnKeyType="done"/);
  assert.match(durationEditor, /onSubmitEditing=\{onSubmitEditing\}/);
  assert.match(form, /<HumanDurationEditor[\s\S]*onSubmitEditing=\{Keyboard\.dismiss\}/);
  assert.match(form, /keyboardType="number-pad"[\s\S]*onSubmitEditing=\{Keyboard\.dismiss\}[\s\S]*returnKeyType="done"/);
});
