import assert from 'node:assert/strict';
import test from 'node:test';
import { homePresentation, type HomePresentationInput } from './ui/home-presentation';

const ready: HomePresentationInput = {
  failure: null,
  filter: 'all',
  homeAvailable: true,
  loading: false,
  mode: 'active',
  totalCount: 2,
  visibleCount: 2,
};

test('authenticated Home never projects a blank loading or recovery state', () => {
  assert.deepEqual(homePresentation({ ...ready, homeAvailable: false, loading: true }), { kind: 'loading' });
  assert.deepEqual(homePresentation({ ...ready, homeAvailable: false, failure: 'offline' }), { kind: 'offline' });
  assert.deepEqual(homePresentation({ ...ready, homeAvailable: false, failure: 'error' }), { kind: 'error' });
});

test('ordinary refresh retains last-good Home instead of replacing it with loading or failure', () => {
  assert.deepEqual(homePresentation({ ...ready, loading: true }), { kind: 'ready' });
  assert.deepEqual(homePresentation({ ...ready, failure: 'offline' }), { kind: 'ready' });
});

test('Home distinguishes account-empty, archived-empty, and filtered-empty content', () => {
  assert.deepEqual(homePresentation({ ...ready, totalCount: 0, visibleCount: 0 }), { kind: 'empty', mode: 'active' });
  assert.deepEqual(homePresentation({ ...ready, mode: 'archived', totalCount: 0, visibleCount: 0 }), { kind: 'empty', mode: 'archived' });
  assert.deepEqual(homePresentation({ ...ready, filter: 'shared', visibleCount: 0 }), { kind: 'filtered-empty' });
});
