#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const apiRoot = 'https://androidpublisher.googleapis.com/androidpublisher/v3';
const uploadRoot = 'https://androidpublisher.googleapis.com/upload/androidpublisher/v3';

function required(value, name) {
  const normalized = value?.trim() ?? '';
  if (!normalized) throw new Error(`${name} is required`);
  return normalized;
}

async function responseJSON(response, operation) {
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${operation} failed (${response.status}): ${text.slice(0, 500)}`);
  }
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    throw new Error(`${operation} returned malformed JSON`);
  }
}

export function internalTrackPayload(versionCode, releaseName) {
  const normalizedCode = String(versionCode);
  if (!/^[1-9][0-9]*$/.test(normalizedCode)) {
    throw new Error('Google Play version code must be a positive integer');
  }
  return {
    track: 'internal',
    releases: [{
      name: required(releaseName, 'release name'),
      status: 'completed',
      versionCodes: [normalizedCode],
    }],
  };
}

export async function uploadGooglePlayInternal({
  accessToken,
  bundlePath,
  fetchImpl = fetch,
  packageName,
  readFileImpl = readFile,
  releaseName,
}) {
  const token = required(accessToken, 'Google OAuth access token');
  const appPackage = required(packageName, 'Android package name');
  const path = required(bundlePath, 'Android App Bundle path');
  const name = required(releaseName, 'release name');
  const encodedPackage = encodeURIComponent(appPackage);
  const authHeaders = { Authorization: `Bearer ${token}` };

  const edit = await responseJSON(await fetchImpl(
    `${apiRoot}/applications/${encodedPackage}/edits`,
    { method: 'POST', headers: { ...authHeaders, 'Content-Type': 'application/json' }, body: '{}' },
  ), 'create edit');
  const editId = required(edit.id, 'Google Play edit ID');

  const bundle = await readFileImpl(path);
  const uploaded = await responseJSON(await fetchImpl(
    `${uploadRoot}/applications/${encodedPackage}/edits/${encodeURIComponent(editId)}/bundles?uploadType=media`,
    { method: 'POST', headers: { ...authHeaders, 'Content-Type': 'application/octet-stream' }, body: bundle },
  ), 'upload bundle');
  const payload = internalTrackPayload(uploaded.versionCode, name);

  await responseJSON(await fetchImpl(
    `${apiRoot}/applications/${encodedPackage}/edits/${encodeURIComponent(editId)}/tracks/internal`,
    { method: 'PUT', headers: { ...authHeaders, 'Content-Type': 'application/json' }, body: JSON.stringify(payload) },
  ), 'assign internal track');

  await responseJSON(await fetchImpl(
    `${apiRoot}/applications/${encodedPackage}/edits/${encodeURIComponent(editId)}:commit`,
    { method: 'POST', headers: { ...authHeaders, 'Content-Type': 'application/json' }, body: '{}' },
  ), 'commit edit');

  return { editId, versionCode: String(uploaded.versionCode) };
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  const result = await uploadGooglePlayInternal({
    accessToken: process.env.GOOGLE_PLAY_ACCESS_TOKEN,
    bundlePath: process.env.GOOGLE_PLAY_BUNDLE_PATH,
    packageName: process.env.GOOGLE_PLAY_PACKAGE_NAME,
    releaseName: process.env.GOOGLE_PLAY_RELEASE_NAME,
  });
  console.log(`Google Play internal release committed at version code ${result.versionCode}`);
}
