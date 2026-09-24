import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  commentDraftAfterFailedSubmit,
  commentDraftHasMeaningfulText,
  commentDraftIsDirty,
  normalizedComment,
} from './ui/comment-presentation';
import {
  clearAllPracticeCommentDrafts,
  practiceCommentComposerDraft,
  practiceCommentDraftGeneration,
  practiceCommentRouteRemovalDecision,
  restorePracticeCommentComposerDraftIfEmpty,
  setPracticeCommentComposerDraft,
} from './ui/comment-draft-presentation';
import { createCommentSubmissionOwner } from './ui/comment-submission-admission';
import {
  nativeBackSocialInteractionState,
  normalizeSocialInteractionRouteState,
} from './social-interaction-route-ancestry';

function source(path: string) {
  const file = fileURLToPath(new URL(path, import.meta.url));
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const commentsRoute = source('../app/(tabs)/following/comments/[eventID].tsx');
const heartsRoute = source('../app/(tabs)/following/comments/[eventID]/hearts/[commentID].tsx');
const activityRoute = source('../app/(tabs)/following/activity/[pathID]/[activityID].tsx');
const followingLayout = source('../app/(tabs)/following/_layout.tsx');
const commentsView = source('./ui/practice-comments-view.tsx');
const editSheet = source('./ui/comment-edit-sheet.tsx');
const rosterView = source('./ui/comment-heart-roster-view.tsx');
const sendIOS = source('./ui/native-comment-send-button.ios.tsx');
const reactionIOS = source('./ui/social-reaction-menu.ios.tsx');
const feedView = source('./ui/social-feed-view.tsx');
const en = JSON.parse(source('../../../packages/i18n/src/locales/en.json')) as Record<string, string>;
const es = JSON.parse(source('../../../packages/i18n/src/locales/es.json')) as Record<string, string>;

test('comment normalization and dirty comparison preserve exact revert semantics', () => {
  assert.equal(normalizedComment('  hello  '), 'hello');
  assert.equal(normalizedComment(' \u0001 '), undefined);
  assert.equal(commentDraftHasMeaningfulText('   \n  '), false);
  assert.equal(commentDraftHasMeaningfulText(' \u0001 '), true);
  assert.equal(commentDraftHasMeaningfulText('x'.repeat(2_001)), true);
  assert.equal(commentDraftIsDirty('hello', ' hello '), false);
  assert.equal(commentDraftIsDirty('hello', 'hello again'), true);
  assert.equal(commentDraftAfterFailedSubmit('', 'submitted'), 'submitted');
  assert.equal(commentDraftAfterFailedSubmit('newer typing', 'submitted'), 'newer typing');
});

test('edit and dirty-discard task copy is complete in English and Spanish', () => {
  for (const catalog of [en, es]) for (const key of [
    'social.commentsEditTitle',
    'social.commentsDiscardTitle',
    'social.commentsDiscardDescription',
    'social.commentsDiscardAction',
    'social.commentsSaving',
  ]) assert.ok(catalog[key]?.trim(), `${key} must be localized`);
});

test('comment editing is a native keyboard-safe task with safe dismissal and stable actions', () => {
  assert.match(commentsView, /<CommentEditSheet/);
  assert.doesNotMatch(commentsView, /editing \? <View|styles\.editActions/);
  assert.match(editSheet, /<NativeSheet/);
  assert.match(editSheet, /dismissible=\{!busy\}/);
  assert.match(editSheet, /commentDraftIsDirty/);
  assert.match(editSheet, /Alert\.alert/);
  assert.match(editSheet, /AccessibilityInfo\.isReduceMotionEnabled/);
  assert.match(editSheet, /InputAccessoryView/);
  assert.match(editSheet, /Keyboard\.dismiss/);
  assert.match(editSheet, /label: i18n\.t\('social\.commentsSave'\)/);
  assert.match(editSheet, /accessibilityLiveRegion="polite"/);
  assert.match(editSheet, /submission\.current\.admit\(\)/);
  assert.match(editSheet, /submission\.current\.owns\(admission\)/);
  assert.match(editSheet, /submission\.current\.release\(admission\)/);
});

test('composer separates disabled from busy, scales vertically, and preserves newer typing on failure', () => {
  assert.match(sendIOS, /busy: boolean; disabled: boolean/);
  assert.match(commentsView, /busy=\{presentation\.busy\}/);
  assert.match(commentsView, /disabled=\{!validDraft\}/);
  assert.match(commentsView, /InputAccessoryView/);
  assert.match(commentsView, /needsCompactVerticalLayout/);
  assert.doesNotMatch(commentsView, /keyboardVerticalOffset=\{88\}/);
  assert.doesNotMatch(commentsView, /automaticallyAdjustKeyboardInsets/);
  assert.match(commentsView, /restorePracticeCommentComposerDraftIfEmpty\([\s\S]*presentation\.eventID,[\s\S]*value,[\s\S]*admittedDraftGeneration/);
  assert.match(commentsView, /const admittedDraftGeneration = practiceCommentDraftGeneration\(\)/);
  assert.match(commentsView, /composerSubmission\.current\.admit\(\)/);
  assert.match(commentsView, /composerSubmission\.current\.owns\(admission\)/);
});

test('local comment admission synchronously rejects duplicate presses and only its owner releases', () => {
  const owner = createCommentSubmissionOwner();
  const first = owner.admit();
  assert.ok(first);
  assert.equal(owner.active(), true);
  assert.equal(owner.admit(), null);
  assert.equal(owner.release(Symbol('stale')), false);
  assert.equal(owner.active(), true);
  assert.equal(owner.release(first), true);
  assert.equal(owner.active(), false);
  assert.ok(owner.admit());
});

test('iOS send control exposes one native actionable button without a suppressing wrapper', () => {
  assert.equal((sendIOS.match(/<Button\b/g) ?? []).length, 1);
  assert.doesNotMatch(sendIOS, /<View|\baccessible\b|accessibilityRole/);
  assert.match(sendIOS, /<Host[^>]*><Button/);
});

test('comments and heart roster are flat intrinsic rows with native system interaction icons', () => {
  assert.doesNotMatch(commentsView, /numberOfLines=\{1\}/);
  assert.doesNotMatch(rosterView, /numberOfLines=\{1\}/);
  assert.doesNotMatch(rosterView, /list:\s*\{[^}]*backgroundColor/);
  assert.doesNotMatch(rosterView, /minHeight: 60/);
  assert.match(reactionIOS, /SwiftUIImage/);
  assert.match(reactionIOS, /systemName=\{selectedEmoji \? 'heart\.fill' : 'heart'\}/);
  assert.doesNotMatch(feedView, /reactionCount:\s*\{[\s\S]*?borderWidth/);
});

test('all direct interaction routes retain native titles and visible cold recovery', () => {
  for (const route of [activityRoute, commentsRoute, heartsRoute]) {
    assert.match(route, /SocialRouteRecoveryView/);
    assert.match(route, /scheduleSocialRouteBootstrap/);
    assert.doesNotMatch(route, /return null/);
  }
  assert.match(activityRoute, /<Stack\.Screen options=\{\{ title:/);
  assert.match(followingLayout, /activity\/\[pathID\]\/\[activityID\][\s\S]*pathDetails\.activityHeading/);
});

test('comment route removal allows pristine and replacement exits but guards dirty and admitted work', () => {
  clearAllPracticeCommentDrafts();
  assert.equal(practiceCommentRouteRemovalDecision('event-1', false), 'allow');
  setPracticeCommentComposerDraft('event-1', 'unsent draft');
  assert.equal(practiceCommentRouteRemovalDecision('event-1', false), 'confirm-discard');
  assert.equal(practiceCommentRouteRemovalDecision('event-1', true), 'block-busy');
  setPracticeCommentComposerDraft('event-1', '   \n  ');
  assert.equal(practiceCommentRouteRemovalDecision('event-1', false), 'allow');
  setPracticeCommentComposerDraft('event-1', '\u0001');
  assert.equal(practiceCommentRouteRemovalDecision('event-1', false), 'confirm-discard');
  setPracticeCommentComposerDraft('event-1', 'x'.repeat(2_001));
  assert.equal(practiceCommentRouteRemovalDecision('event-1', false), 'confirm-discard');
  clearAllPracticeCommentDrafts();
  assert.equal(practiceCommentRouteRemovalDecision('event-1', true), 'allow');
  assert.match(commentsRoute, /navigation\.addListener\('beforeRemove'/);
  assert.match(commentsRoute, /navigation\.dispatch\(event\.data\.action\)/);
  assert.match(commentsRoute, /presentNativeDestructiveConfirmation/);
});

test('disposed draft generations reject late failed-submit restoration for a replacement account', () => {
  clearAllPracticeCommentDrafts();
  setPracticeCommentComposerDraft('event-1', 'account A submission');
  const accountAGeneration = practiceCommentDraftGeneration();
  setPracticeCommentComposerDraft('event-1', '');

  clearAllPracticeCommentDrafts();
  assert.equal(
    restorePracticeCommentComposerDraftIfEmpty('event-1', 'account A submission', accountAGeneration),
    false,
  );
  assert.equal(practiceCommentComposerDraft('event-1'), '');

  setPracticeCommentComposerDraft('event-1', 'account B typing');
  assert.equal(
    restorePracticeCommentComposerDraftIfEmpty('event-1', 'account A submission', accountAGeneration),
    false,
  );
  assert.equal(practiceCommentComposerDraft('event-1'), 'account B typing');
  clearAllPracticeCommentDrafts();
});

test('cold heart routes construct keyed Following to Comments to Roster native ancestry', () => {
  const intent = {
    commentID: 'comment-1',
    eventID: 'event-1',
    kind: 'comment-hearts' as const,
    routeKey: 'social:comment-hearts:event-1:comment-1',
  };
  const following = { key: 'following-root', name: 'index', state: { retained: true } };
  const comments = { key: 'comments-route', name: 'comments/[eventID]', params: { eventID: 'event-1' } };
  const roster = { key: 'roster-route', name: 'comments/[eventID]/hearts/[commentID]', params: { commentID: 'comment-1', eventID: 'event-1' } };
  const normalized = normalizeSocialInteractionRouteState({
    index: 2,
    key: 'following-stack',
    routes: [roster, following, comments],
  }, intent);
  assert.deepEqual(normalized.routes.map(({ name }) => name), [
    'index',
    'comments/[eventID]',
    'comments/[eventID]/hearts/[commentID]',
  ]);
  assert.equal(normalized.key, 'following-stack');
  assert.equal(normalized.routes[0]?.key, 'following-root');
  assert.equal(normalized.routes[1]?.key, 'comments-route');
  assert.equal(normalized.routes[2]?.key, 'roster-route');
  assert.strictEqual(normalized.routes[0]?.state, following.state);
  const backToComments = nativeBackSocialInteractionState(normalized);
  const backToFollowing = nativeBackSocialInteractionState(backToComments);
  assert.equal(backToComments.routes.at(-1)?.name, 'comments/[eventID]');
  assert.equal(backToFollowing.routes.at(-1)?.name, 'index');
});
