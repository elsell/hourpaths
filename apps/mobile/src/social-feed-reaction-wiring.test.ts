import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('reaction mutations use generated set/remove contracts and authoritative summaries', () => {
  assert.match(page, /api\.setSocialFeedReaction\(event\.id, reaction, idempotencyKey\)/);
  assert.match(page, /api\.removeSocialFeedReaction\(event\.id, idempotencyKey\)/);
  assert.match(page, /applySocialReactionSummary\([\s\S]*socialReactionSummaryFromAPI\(envelope\.data\)/);
  assert.match(page, /socialFeedPage\.current = next;[\s\S]*setSocialFeed\(\(current\) => \(\{ \.\.\.current, items: next\.items \}\)\)/);
});

test('reaction commits fail closed across session, owner, and event replacement', () => {
  assert.match(page, /const ticket = operationOwner\.issue\(\)/);
  assert.match(page, /createSocialSessionTarget\(ownerID, currentSession, reactionIntentKey\)/);
  assert.match(page, /ownsCurrentSocialOperation\([\s\S]*ownerID,[\s\S]*reactionIntentKey,[\s\S]*currentSession/);
  assert.match(page, /socialFeedPage\.current\.items\.some\(\(\{ id \}\) => id === event\.id\)/);
  assert.match(page, /if \(!currentTarget\(\)\) return;/);
  assert.match(page, /handleFeatureSessionFailure\(cause, currentSession, \{ current: currentTarget \}\)/);
});

test('session reset invalidates every in-flight event reaction', () => {
  assert.match(page, /for \(const owner of socialFeedReactionOperations\.values\(\)\) owner\.invalidate\(\)/);
  assert.match(page, /socialFeedReactionOperations\.clear\(\)/);
  assert.match(page, /socialFeedReactionRevisions\.clear\(\)/);
});

test('feed loads preserve reaction summaries committed after request admission', () => {
  assert.match(page, /const admittedReactionRevisions = new Map\(socialFeedReactionRevisions\)/);
  assert.match(page, /preserveNewerSocialReactionSummaries\([\s\S]*admittedReactionRevisions,[\s\S]*socialFeedReactionRevisions\)/);
  assert.match(page, /socialFeedReactionRevisions\.set\([\s\S]*event\.id,[\s\S]*\+ 1/);
});
