import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createProfileSearchOwner,
  mergeProfileSearchPage,
  publicProfileFromAPI,
  profileSearchPageFromAPI,
  profileSearchQuery,
  type ProfileSearchPage,
  type ProfileSearchState,
  type PublicProfile,
} from './index.js';

const alex: PublicProfile = {
  userId: 'user-alex',
  username: 'alex',
  displayName: 'Alex Rivera',
  followerCount: 3,
  followingCount: 5,
  relationship: 'none',
};

const alexa: PublicProfile = {
  userId: 'user-alexa',
  username: 'alexa.reads',
  displayName: 'Alexa Morgan',
  description: 'Reading every day.',
  followerCount: 7,
  followingCount: 2,
  relationship: 'following',
};

test('profile queries trim, normalize, and require two non-whitespace characters', () => {
  assert.equal(profileSearchQuery('  al  '), 'al');
  assert.equal(profileSearchQuery(' A\u0301l '), 'Ál');
  for (const query of ['', ' ', 'a', ' a ', 'a \t']) {
    assert.equal(profileSearchQuery(query), undefined);
  }
});

test('profile search owner makes no request for an ineligible query', async () => {
  const owner = createProfileSearchOwner();
  let requests = 0;
  const result = await owner.search(' a ', async () => {
    requests += 1;
    return { items: [], nextCursor: '' };
  });
  assert.equal(requests, 0);
  assert.deepEqual(result, {
    kind: 'cleared',
    state: { query: '', items: [], nextCursor: '' },
  });
});

test('a newer search supersedes an older response', async () => {
  const owner = createProfileSearchOwner();
  let resolveOld!: (page: ProfileSearchPage) => void;
  const old = owner.search('al', () => new Promise((resolve) => { resolveOld = resolve; }));
  const current = await owner.search('be', async () => ({
    items: [{ ...alex, userId: 'user-bea', username: 'bea', displayName: 'Bea Moss' }],
    nextCursor: '',
  }));
  resolveOld({ items: [alex], nextCursor: '' });

  assert.equal((await old).kind, 'superseded');
  assert.equal(current.kind, 'loaded');
  if (current.kind === 'loaded') assert.equal(current.state.query, 'be');
});

test('profile pages validate the exact public projection and merge once', () => {
  const initial: ProfileSearchState = { query: 'al', items: [], nextCursor: '' };
  const first = mergeProfileSearchPage(initial, {
    items: [alex, alexa],
    nextCursor: 'signed+/=',
  }, 'al', '');
  assert.deepEqual(first, {
    query: 'al',
    items: [alex, alexa],
    nextCursor: 'signed+/=',
  });
  assert.deepEqual(
    mergeProfileSearchPage(first, { items: [alexa], nextCursor: '' }, 'al', 'signed+/='),
    { query: 'al', items: [alex, alexa], nextCursor: '' },
  );
});

test('profile pages reject stale cursors, stale queries, duplicates, and private fields', () => {
  const state: ProfileSearchState = { query: 'al', items: [], nextCursor: '' };
  assert.throws(
    () => mergeProfileSearchPage(state, { items: [], nextCursor: '' }, 'bo', ''),
    /stale profile query/,
  );
  assert.throws(
    () => mergeProfileSearchPage({ ...state, nextCursor: 'next' }, { items: [], nextCursor: '' }, 'al', ''),
    /stale profile cursor/,
  );
  assert.throws(
    () => mergeProfileSearchPage(state, { items: [alex, alex], nextCursor: '' }, 'al', ''),
    /invalid profile page/,
  );
  for (const privateField of [
    { email: 'alex@example.test' },
    { profileVisibility: 'private' },
    { providerSubject: 'subject-1' },
    { paths: [] },
    { status: 'active' },
  ]) {
    assert.throws(
      () => mergeProfileSearchPage(state, {
        items: [{ ...alex, ...privateField }],
        nextCursor: '',
      }, 'al', ''),
      /invalid public profile/,
    );
  }
});

test('profile pages reject malformed identity, counts, optional copy, and picture URLs', () => {
  const state: ProfileSearchState = { query: 'al', items: [], nextCursor: '' };
  for (const malformed of [
    { ...alex, userId: '' },
    { ...alex, username: ' alex' },
    { ...alex, displayName: '' },
    { ...alex, followerCount: -1 },
    { ...alex, followingCount: 1.5 },
    { ...alex, description: '' },
    { ...alex, profilePictureUrl: 'http://example.test/avatar.jpg' },
    { ...alex, profilePictureUrl: 'https://user:secret@example.test/avatar.jpg' },
  ]) {
    assert.throws(
      () => mergeProfileSearchPage(state, { items: [malformed], nextCursor: '' }, 'al', ''),
      /invalid public profile/,
    );
  }
});

test('generated profile envelopes map id to the public client identity and reject extra metadata', () => {
  assert.deepEqual(profileSearchPageFromAPI({
    $schema: 'https://api.example.test/schemas/SearchOutputBody.json',
    data: [{ id: alex.userId, username: alex.username, displayName: alex.displayName, followerCount: 3, followingCount: 5, relationship: 'none' }],
    meta: { nextCursor: 'signed' },
  }), { items: [alex], nextCursor: 'signed' });
  assert.deepEqual(publicProfileFromAPI({
    $schema: 'https://api.example.test/schemas/ProfileOutputBody.json',
    data: { id: alexa.userId, username: alexa.username, displayName: alexa.displayName, description: alexa.description, followerCount: 7, followingCount: 2, relationship: 'following' },
  }), alexa);
  for (const envelope of [
    { data: [{ id: alex.userId, username: alex.username, displayName: alex.displayName, followerCount: 3, followingCount: 5, relationship: 'none', email: 'private@example.test' }], meta: {} },
    { data: [], meta: { nextCursor: '', totalUsers: 1 } },
    { data: [], meta: {}, viewer: { id: 'viewer' } },
    { $schema: 42, data: [], meta: {} },
  ]) assert.throws(() => profileSearchPageFromAPI(envelope), /invalid profile response/);
});

test('pagination requests only the current signed cursor and can be canceled', async () => {
  const owner = createProfileSearchOwner();
  const first = await owner.search('al', async () => ({ items: [alex], nextCursor: 'next' }));
  assert.equal(first.kind, 'loaded');
  if (first.kind !== 'loaded') return;

  let requested: readonly string[] = [];
  const more = owner.loadMore(first.state, async (query, cursor) => {
    requested = [query, cursor];
    return { items: [alexa], nextCursor: '' };
  });
  owner.cancel();
  assert.deepEqual(await more, { kind: 'superseded' });
  assert.deepEqual(requested, ['al', 'next']);

  const stale = await owner.loadMore(first.state, async () => ({ items: [], nextCursor: '' }));
  assert.deepEqual(stale, { kind: 'failed', reason: 'stale' });
});
