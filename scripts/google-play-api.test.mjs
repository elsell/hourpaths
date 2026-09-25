import assert from 'node:assert/strict';
import test from 'node:test';
import { internalTrackPayload, uploadGooglePlayInternal } from './google-play-api.mjs';

test('internal payload completes only the exact uploaded version', () => {
  assert.deepEqual(internalTrackPayload(42, 'v0.1.1'), {
    track: 'internal',
    releases: [{ name: 'v0.1.1', status: 'completed', versionCodes: ['42'] }],
  });
  assert.throws(() => internalTrackPayload(0, 'v0.1.1'));
});

test('creates, uploads, assigns internal track, and commits one edit', async () => {
  const requests = [];
  const responses = [{ id: 'edit-7' }, { versionCode: '42' }, { track: 'internal' }, { id: 'edit-7' }];
  const fetchImpl = async (url, options) => {
    requests.push({ url, options });
    return new Response(JSON.stringify(responses.shift()), { status: 200 });
  };
  const result = await uploadGooglePlayInternal({
    accessToken: 'token', bundlePath: '/tmp/app.aab', fetchImpl,
    packageName: 'com.hourpaths.mobile', readFileImpl: async () => Buffer.from('bundle'),
    releaseName: 'v0.1.1',
  });
  assert.deepEqual(result, { editId: 'edit-7', versionCode: '42' });
  assert.equal(requests.length, 4);
  assert.match(requests[0].url, /applications\/com\.hourpaths\.mobile\/edits$/);
  assert.match(requests[1].url, /edits\/edit-7\/bundles\?uploadType=media$/);
  assert.match(requests[2].url, /edits\/edit-7\/tracks\/internal$/);
  assert.match(requests[3].url, /edits\/edit-7:commit$/);
  assert.equal(JSON.parse(requests[2].options.body).releases[0].status, 'completed');
});

test('fails without committing when an API step is rejected', async () => {
  let calls = 0;
  await assert.rejects(uploadGooglePlayInternal({
    accessToken: 'token', bundlePath: '/tmp/app.aab',
    fetchImpl: async () => {
      calls += 1;
      return new Response(calls === 1 ? JSON.stringify({ id: 'edit-8' }) : 'denied', { status: calls === 1 ? 200 : 403 });
    },
    packageName: 'com.hourpaths.mobile', readFileImpl: async () => Buffer.from('bundle'),
    releaseName: 'v0.1.1',
  }), /upload bundle failed \(403\)/);
  assert.equal(calls, 2);
});
