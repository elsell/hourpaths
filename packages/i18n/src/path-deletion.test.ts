import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

test('Path deletion warning, actions, and standalone notice are localized', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);
  const keys = [
    'pathDelete.action',
    'pathDelete.heading',
    'pathDelete.warning',
    'pathDelete.timerWarning',
    'pathDelete.archiveAlternative',
    'pathDelete.confirm',
    'pathDelete.deleting',
    'notification.pathDeleted',
  ] as const;
  for (const key of keys) {
    assert.notEqual(english.t(key, { displayName: 'Alex', pathName: 'Piano' }), key);
    assert.notEqual(spanish.t(key, { displayName: 'Alex', pathName: 'Piano' }), key);
    assert.notEqual(
      english.t(key, { displayName: 'Alex', pathName: 'Piano' }),
      spanish.t(key, { displayName: 'Alex', pathName: 'Piano' }),
    );
  }
  assert.match(english.t('pathDelete.heading', { pathName: 'Piano' }), /Piano/);
  assert.match(english.t('pathDelete.warning'), /everyone.*recorded activity/i);
  assert.match(english.t('pathDelete.timerWarning'), /running timers.*stop.*elapsed time.*lost/i);
  assert.match(english.t('pathDelete.archiveAlternative'), /archive.*keep/i);
  assert.match(
    english.t('notification.pathDeleted', { displayName: 'Alex', pathName: 'Piano' }),
    /Alex.*Piano.*permanently removed/i,
  );
});
