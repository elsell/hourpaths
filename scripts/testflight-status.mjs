import { createPrivateKey, sign } from 'node:crypto';

const buildNumber = process.env.TESTFLIGHT_BUILD_NUMBER;
if (!/^[1-9][0-9]{0,8}$/.test(buildNumber ?? '')) throw new Error('A numeric build number is required');
for (const name of ['ASC_API_KEY_ID', 'ASC_API_ISSUER_ID', 'ASC_API_KEY_P8_BASE64']) {
  if (!process.env[name]) throw new Error(`Missing ${name}`);
}
const encode = (value) => Buffer.from(JSON.stringify(value)).toString('base64url');
const now = Math.floor(Date.now() / 1000);
const unsigned = `${encode({ alg: 'ES256', kid: process.env.ASC_API_KEY_ID, typ: 'JWT' })}.${encode({ iss: process.env.ASC_API_ISSUER_ID, iat: now, exp: now + 300, aud: 'appstoreconnect-v1' })}`;
const key = createPrivateKey(Buffer.from(process.env.ASC_API_KEY_P8_BASE64, 'base64'));
const token = `${unsigned}.${sign('sha256', Buffer.from(unsigned), { key, dsaEncoding: 'ieee-p1363' }).toString('base64url')}`;
async function read(path, query = {}) {
  const url = new URL(path, 'https://api.appstoreconnect.apple.com');
  url.search = new URLSearchParams(query).toString();
  const response = await fetch(url, { headers: { Authorization: `Bearer ${token}` }, signal: AbortSignal.timeout(20000), redirect: 'error' });
  if (!response.ok) throw new Error(`App Store Connect request failed: ${response.status} ${path}`);
  return response.json();
}
const apps = await read('/v1/apps', { 'filter[bundleId]': 'com.hourpaths.mobile', limit: '2' });
if (apps.data.length !== 1) throw new Error('Expected exactly one HourPaths app');
const appID = apps.data[0].id;
const result = await read('/v1/builds', { 'filter[app]': appID, 'filter[version]': buildNumber, include: 'preReleaseVersion,buildBetaDetail', limit: '10' });
const groups = await read(`/v1/apps/${encodeURIComponent(appID)}/betaGroups`, { limit: '200' });
const groupSummary = (group) => ({ id: group.id, internal: group.attributes.isInternalGroup, automatic: group.attributes.hasAccessToAllBuilds });
console.log(JSON.stringify({ requestedBuild: buildNumber, foundBuilds: result.data.length, availableGroups: groups.data.map(groupSummary) }));
for (const build of result.data) {
  const related = (name) => result.included?.find((value) => value.id === build.relationships?.[name]?.data?.id && value.type === build.relationships?.[name]?.data?.type);
  console.log(JSON.stringify({
    id: build.id, buildNumber: build.attributes.version,
    marketingVersion: related('preReleaseVersion')?.attributes.version,
    uploadedDate: build.attributes.uploadedDate, processingState: build.attributes.processingState,
    expired: build.attributes.expired, usesNonExemptEncryption: build.attributes.usesNonExemptEncryption,
    beta: related('buildBetaDetail')?.attributes,

  }));
}

for (const group of groups.data) {
  const builds = await read(`/v1/betaGroups/${encodeURIComponent(group.id)}/builds`, { 'filter[version]': buildNumber, limit: '200' });
  console.log(JSON.stringify({ group: groupSummary(group), assignedBuilds: builds.data.map((build) => ({ id: build.id, version: build.attributes.version })) }));
}
