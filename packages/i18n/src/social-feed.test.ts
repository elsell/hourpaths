import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from './index.js';

const keys = [
  'social.feedHeading',
  'social.people',
  'social.peopleOpen',
  'social.feedEmptyHeading',
  'social.feedEmptyDescription',
  'social.feedUnavailableHeading',
  'social.feedUnavailableDescription',
  'social.feedRow',
  'social.feedRowEdited',
  'social.feedPracticeOnPath',
  'social.feedAchievementInterval',
  'social.feedAchievementOverall',
  'social.feedAchievementTarget',
  'social.feedAchievementRowInterval',
  'social.feedAchievementRowOverall',
  'social.edited',
  'social.activeHeading',
  'social.activeLoading',
  'social.activeEmpty',
  'social.activeUnavailable',
  'social.activeTimer',
  'social.activeGroupAccessibility',
  'social.activeLoadMore',
  'social.activeLoadingMore',
  'social.reactionHeart',
  'social.reactionApplause',
  'social.reactionFire',
  'social.reactionStrong',
  'social.reactionCelebrate',
  'social.reactionPicker',
  'social.reactionPickerSelected',
  'social.reactionRemove',
  'social.reactionCount',
  'social.reactionUpdating',
  'social.reactionUnavailable',
  'social.commentsHeading',
  'social.commentsOpen',
  'social.commentsLoading',
  'social.commentsEmptyHeading',
  'social.commentsEmptyDescription',
  'social.commentsUnavailableHeading',
  'social.commentsUnavailableDescription',
  'social.commentsPlaceholder',
  'social.commentsSend',
  'social.commentsActions',
  'social.commentsAuthorAvatar',
  'social.commentsEdit',
  'social.commentsHistory',
  'social.commentsDelete',
  'social.commentsEdited',
  'social.commentsLoadMore',
  'social.commentsLoadingMore',
  'social.commentsMutationUnavailable',
  'social.commentsDeleteConfirmTitle',
  'social.commentsDeleteConfirmDescription',
  'social.commentsSave',
  'social.commentsCancel',
  'social.commentsHistoryHeading',
  'social.commentsHistoryUnavailable',
  'social.commentHeartAdd',
  'social.commentHeartRemove',
  'social.commentHeartCount',
  'social.commentHeartUpdating',
  'social.commentHeartUnavailable',
  'social.commentHeartRosterHeading',
  'social.commentHeartRosterLoading',
  'social.commentHeartRosterEmptyHeading',
  'social.commentHeartRosterEmptyDescription',
  'social.commentHeartRosterUnavailableHeading',
  'social.commentHeartRosterUnavailableDescription',
  'social.commentHeartRosterLoadMore',
  'social.commentHeartRosterLoadingMore',
] as const;

test('practice and active feed have complete English and Spanish native states', () => {
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of keys) {
      const message = translator.t(key, {
        participant: 'Alex',
        path: 'Piano',
        duration: '30m',
        time: 'Now',
        timers: 'Piano · 30m',
        reaction: 'Heart',
        count: 2,
      });
      assert.notEqual(message, key);
      assert.ok(message.trim().length > 0);
    }
  }
});

test('reaction count is pluralized and emoji names remain spoken localized copy', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.equal(english.t('social.reactionHeart'), 'Heart');
  assert.equal(spanish.t('social.reactionHeart'), 'Corazón');
  assert.equal(english.t('social.reactionCount', { count: 1, reaction: 'Heart' }), 'Heart, 1 reaction');
  assert.equal(english.t('social.reactionCount', { count: 2, reaction: 'Heart' }), 'Heart, 2 reactions');
});

test('shared achievement engagement copy does not describe every event as practice', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.equal(english.t('social.reactionPicker', { participant: 'Alex' }), "React to Alex's update");
  assert.equal(spanish.t('social.reactionPicker', { participant: 'Alex' }), 'Reaccionar a la actualización de Alex');
  assert.equal(english.t('social.commentsEmptyDescription'), 'Start the conversation about this update.');
  assert.equal(spanish.t('social.commentsEmptyDescription'), 'Inicia la conversación sobre esta actualización.');
});

test('comment heart controls and roster counts are localized accessibly', () => {
  const english = createTranslator(['en']);
  const spanish = createTranslator(['es']);

  assert.equal(english.t('social.commentHeartCount', { count: 1 }), '1 heart. Show people');
  assert.equal(english.t('social.commentHeartCount', { count: 2 }), '2 hearts. Show people');
  assert.equal(spanish.t('social.commentHeartAdd'), 'Añadir corazón');
  assert.equal(spanish.t('social.commentHeartRemove'), 'Quitar corazón');
});

test('interaction settings are fully localized in English and Spanish', () => {
  for (const locale of ['en', 'es']) {
    const translator = createTranslator([locale]);
    for (const key of [
      'settings.interactions.heading',
      'settings.interactions.openLabel',
      'settings.interactions.comments',
      'settings.interactions.reactions',
      'settings.interactions.footer',
      'settings.interactions.loading',
      'settings.interactions.unavailableHeading',
      'settings.interactions.unavailableDescription',
      'settings.interactions.retry',
      'settings.interactions.saving',
      'settings.interactions.saveError',
      'settings.interactions.on',
      'settings.interactions.off',
      'social.interactionDisabledComments',
      'social.interactionDisabledReactions',
      'social.interactionDisabledDismiss',
    ] as const) {
      const message = translator.t(key);
      assert.notEqual(message, key);
      assert.ok(message.trim().length > 0);
    }
  }
  assert.equal(
    createTranslator(['en']).t('settings.interactions.footer'),
    'Turning a control off hides its existing comments or reactions without deleting them. Turn it on again to restore interactions that still exist.',
  );
  assert.equal(
    createTranslator(['es']).t('settings.interactions.footer'),
    'Al desactivar un control, se ocultan sus comentarios o reacciones existentes sin eliminarlos. Vuelve a activarlo para restaurar las interacciones que aún existan.',
  );
});
