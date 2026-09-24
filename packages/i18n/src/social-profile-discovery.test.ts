import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

const keys = [
  'social.following',
  'social.searchPlaceholder',
  'social.searchHint',
  'social.searchLoading',
  'social.searchEmptyHeading',
  'social.searchEmptyDescription',
  'social.searchUnavailableHeading',
  'social.searchUnavailableDescription',
  'social.searchResults',
  'social.loadMore',
  'social.loadingMore',
  'social.profileHeading',
  'social.profileFollowers',
  'social.profileFollowing',
  'social.profileUnavailableHeading',
  'social.profileUnavailableDescription',
  'social.profileRefresh',
  'social.neutralAvatarLabel',
] as const;

test('profile discovery has complete localized native states without private identity copy', () => {
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of keys) {
      const message = translator.t(key);
      assert.notEqual(message, key);
      assert.ok(message.trim().length > 0);
      assert.doesNotMatch(message, /e-?mail|correo electr[oó]nico|provider|proveedor/iu);
    }
  }
});
