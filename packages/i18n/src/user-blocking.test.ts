import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index';

test('blocking workflow copy is complete and interpolates in every locale', () => {
  for (const locale of ['en', 'es']) {
    const i18n = createTranslator([locale]);
    for (const key of [
      'blocking.blockAction',
      'blocking.reviewing',
      'blocking.blocking',
      'blocking.confirmTitle',
      'blocking.confirmDescription',
      'blocking.sharedPathsWarning',
      'blocking.leavePathsSeparately',
      'blocking.blockedSuccess',
      'blocking.blockUnavailable',
      'blocking.settingsHeading',
      'blocking.settingsOpenLabel',
      'blocking.loading',
      'blocking.emptyHeading',
      'blocking.emptyDescription',
      'blocking.unavailableHeading',
      'blocking.unavailableDescription',
      'blocking.loadMore',
      'blocking.loadingMore',
      'blocking.unblock',
      'blocking.unblockConfirmTitle',
      'blocking.unblockConfirmDescription',
      'blocking.unblockedSuccess',
      'blocking.unblockUnavailable',
    ] as const) {
      const value = i18n.t(key, { count: 2, paths: 'Piano, Reading', username: 'alex.r' });
      assert.notEqual(value, key);
      assert.doesNotMatch(value, /{{/);
    }
  }
});
