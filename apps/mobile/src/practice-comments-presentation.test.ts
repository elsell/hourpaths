import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = (path: string) => readFileSync(decodeURIComponent(new URL(path, import.meta.url).pathname), 'utf8');
const page = source('../app/index.tsx');
const route = source('../app/following/comments/[eventID].tsx');
const layout = source('../app/(tabs)/following/_layout.tsx') + source('../app/_layout.tsx') + source('./ui/navigation-theme.ts');
const feed = source('./ui/social-feed-view.tsx');
const view = source('./ui/practice-comments-view.tsx');
const nativeMenu = source('./ui/native-action-menu.ios.tsx');
const fallbackMenu = source('./ui/comment-action-menu.tsx');
const rosterRoute = source('../app/following/comments/[eventID]/hearts/[commentID].tsx');
const rosterPresentation = source('./ui/comment-heart-roster-route-presentation.tsx');
const rosterView = source('./ui/comment-heart-roster-view.tsx');
const nativeHeart = source('./ui/comment-heart-icon.ios.tsx');

test('each compact feed event and resolved comment notification open the dedicated native route', () => {
  assert.match(feed, /onOpenComments/);
  assert.match(feed, /SettingsIcon systemName="bubble\.left"/);
  assert.match(feed, /event\.type === 'goal_achievement'[\s\S]*onOpenComments=\{\(\) => onOpenComments\(event\)\}/);
  assert.match(page, /openPracticeComments\(event\.id, event\.participant\.userId/);
  assert.match(page, /notification\.type === 'practice_comment' \|\| notification\.type === 'comment_heart'[\s\S]*notification\.socialFeedEventId[\s\S]*notification\.commentId/);
  assert.match(layout, /comments\/\[eventID\][\s\S]*social\.commentsHeading/);
  assert.match(route, /usePracticeCommentsRoutePresentation/);
  assert.doesNotMatch(route, /Modal|NativeSheet/);
});

test('comments use compact native rows, accessible menus, and a keyboard-safe bottom composer', () => {
  assert.match(view, /KeyboardAvoidingView/);
  assert.match(view, /ThemedTextInput/);
  assert.match(view, /NativeCommentSendButton/);
  assert.match(view, /SocialProfileAvatar/);
  assert.match(view, /profilePictureURL=\{comment\.author\.profilePictureURL\}/);
  assert.match(view, /commentsAuthorAvatar/);
  assert.match(view, /CommentActionMenu/);
  assert.match(view, /NativeContentUnavailable/);
  assert.match(view, /accessibilityLiveRegion/);
  assert.match(view, /allowFontScaling/);
  assert.match(nativeMenu, /@expo\/ui\/swift-ui/);
  assert.match(nativeMenu, /Menu/);
  assert.match(fallbackMenu, /NativeActionMenu/);
  assert.match(view, /comment\.edited && !comment\.pending[\s\S]*commentsHistory/);
});

test('comment mutations use caller versions, optimistic projections, stable keys, and authoritative responses', () => {
  assert.match(page, /appendOptimisticPracticeComment/);
  assert.match(page, /updatePracticeCommentOptimistically/);
  assert.match(page, /expectedVersion: comment\.version/);
  assert.match(page, /createSocialFeedComment/);
  assert.match(page, /editSocialFeedComment/);
  assert.match(page, /deleteSocialFeedComment/);
  assert.match(page, /practiceCommentFromAPI/);
  assert.match(page, /replacePracticeComment/);
  assert.match(page, /Crypto\.randomUUID\(\)/);
});

test('overlapping mutations and crossed history responses are operation-owned', () => {
  assert.match(page, /rollbackPracticeCommentEdit\(practiceCommentPage\.current, comment, text\)/);
  assert.match(page, /restoreDeletedPracticeComment\(practiceCommentPage\.current, comment\)/);
  assert.match(page, /const ticket = practiceCommentHistoryOperations\.issue\(\)/);
  assert.match(page, /practiceCommentHistoryTarget\.current = historyTarget/);
  assert.match(page, /practiceCommentHistoryPageFromAPI\(await response\.json\(\), comment\.id\)/);
  assert.match(page, /mergePracticeCommentHistoryPage\(baseline, incoming, cursor\)/);
  assert.match(page, /!ticket\.current\(\) \|\| !currentPracticeCommentHistoryTarget\(historyTarget\)/);
  assert.match(page, /if \(cursor !== baseline\.nextCursor\) return/);
});

test('comment hearts are compact, accessible, optimistic controls with selected-only brand color', () => {
  assert.match(view, /accessibilityLabel=\{i18n\.t\(comment\.heartedByViewer \? 'social\.commentHeartRemove' : 'social\.commentHeartAdd'\)\}/);
  assert.match(view, /accessibilityState=\{\{ busy: comment\.heartPending, selected: comment\.heartedByViewer \}\}/);
  assert.match(view, /CommentHeartIcon selected=\{comment\.heartedByViewer\}/);
  assert.match(view, /comment\.heartedByViewer \? styles\.heartSelected : null/);
  assert.match(view, /comment\.heartCount > 0/);
  assert.match(view, /accessibilityLabel=\{i18n\.t\('social\.commentHeartCount'/);
  assert.match(view, /presentation\.openHeartRoster\(comment\)/);
  assert.match(rosterPresentation, /PracticeCommentHeartRosterPage/);
  assert.match(nativeHeart, /@expo\/ui\/swift-ui/);
  assert.match(nativeHeart, /systemName=\{selected \? 'heart\.fill' : 'heart'\}/);
  assert.match(nativeHeart, /selected \? mobileTheme\.colors\.accent : mobileTheme\.colors\.textMuted/);
  assert.doesNotMatch(view, /reactionsEnabled/);
});

test('heart mutations remain responsive while operation ownership rejects crossed responses', () => {
  assert.match(page, /createPracticeCommentHeartOperationOwner\(\(\) => Crypto\.randomUUID\(\)\)/);
  assert.match(page, /updatePracticeCommentHeartOptimistically/);
  assert.match(page, /practiceCommentHeartOperations\.submit/);
  assert.match(page, /setPracticeCommentHeart/);
  assert.match(page, /removePracticeCommentHeart/);
  assert.match(page, /result\.kind === 'superseded'/);
  assert.match(page, /applyPracticeCommentHeartState/);
  assert.match(page, /rollbackPracticeCommentHeart/);
  assert.match(page, /practiceCommentMutationFromAPI\(envelope\.data, comment\.author, comment\)/);
  assert.doesNotMatch(view, /disabled=\{comment\.heartPending/);
});

test('a positive heart count opens its own native-stack roster with compact native rows', () => {
  assert.match(layout, /comments\/\[eventID\]\/hearts\/\[commentID\][\s\S]*social\.commentHeartRosterHeading/);
  assert.match(rosterRoute, /useCommentHeartRosterRoutePresentation/);
  assert.match(rosterRoute, /CommentHeartRosterView/);
  assert.doesNotMatch(rosterRoute, /Modal|NativeSheet/);
  assert.match(rosterView, /FlatList/);
  assert.match(rosterView, /SocialProfileAvatar/);
  assert.match(rosterView, /SettingsIcon systemName="chevron\.right" variant="disclosure"/);
  assert.match(rosterView, /NativeContentUnavailable/);
  assert.match(rosterView, /accessibilityRole="button"/);
  assert.match(rosterView, /onRefresh=\{presentation\.refresh\}/);
  assert.match(page, /prepareCommentHeartRosterRoute/);
  assert.match(page, /practiceCommentHeartRosterPageFromAPI/);
  assert.match(page, /mergePracticeCommentHeartRosterPage\(baseline, incoming, cursor, requestedRevision\)/);
  assert.match(page, /practiceCommentHeartRosterPage\.current\?\.revision !== requestedRevision/);
  assert.match(page, /CommentHeartRosterRouteSource/);
});
