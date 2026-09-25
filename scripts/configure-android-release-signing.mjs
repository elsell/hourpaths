#!/usr/bin/env node
import { readFile, writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const marker = '// HOURPATHS RELEASE SIGNING';

function blockRange(source, anchor, from = 0) {
  const anchorIndex = source.indexOf(anchor, from);
  if (anchorIndex < 0) throw new Error(`missing Gradle block: ${anchor}`);
  const openIndex = source.indexOf('{', anchorIndex + anchor.length);
  if (openIndex < 0) throw new Error(`missing opening brace for Gradle block: ${anchor}`);
  let depth = 0;
  for (let index = openIndex; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    if (source[index] === '}') depth -= 1;
    if (depth === 0) return { anchorIndex, openIndex, closeIndex: index };
  }
  throw new Error(`unterminated Gradle block: ${anchor}`);
}

export function configureAndroidReleaseSigning(source) {
  if (source.includes(marker)) return source;

  const buildTypes = blockRange(source, 'buildTypes');
  const release = blockRange(source, 'release', buildTypes.openIndex + 1);
  if (release.closeIndex > buildTypes.closeIndex) {
    throw new Error('release build type is outside buildTypes');
  }
  const releaseBody = source.slice(release.openIndex + 1, release.closeIndex);
  const debugSigning = /signingConfig\s+signingConfigs\.debug/;
  if (!debugSigning.test(releaseBody)) {
    throw new Error('release build type does not use the expected generated debug signing config');
  }
  const updatedReleaseBody = releaseBody.replace(
    debugSigning,
    'signingConfig signingConfigs.hourPathsRelease',
  );
  let updated = source.slice(0, release.openIndex + 1)
    + updatedReleaseBody
    + source.slice(release.closeIndex);

  const updatedBuildTypes = updated.indexOf('buildTypes', buildTypes.anchorIndex);
  const buildTypesLineStart = updated.lastIndexOf('\n', updatedBuildTypes) + 1;
  const signingBlock = `    ${marker}\n    signingConfigs {\n        hourPathsRelease {\n            def hourPathsStorePath = System.getenv("HOURPATHS_ANDROID_UPLOAD_KEYSTORE_PATH")\n            def hourPathsStorePassword = System.getenv("HOURPATHS_ANDROID_UPLOAD_KEYSTORE_PASSWORD")\n            def hourPathsKeyAlias = System.getenv("HOURPATHS_ANDROID_UPLOAD_KEY_ALIAS")\n            def hourPathsKeyPassword = System.getenv("HOURPATHS_ANDROID_UPLOAD_KEY_PASSWORD")\n            if (!hourPathsStorePath || !hourPathsStorePassword || !hourPathsKeyAlias || !hourPathsKeyPassword) {\n                throw new GradleException("HourPaths Android release signing environment is incomplete")\n            }\n            storeFile file(hourPathsStorePath)\n            storePassword hourPathsStorePassword\n            keyAlias hourPathsKeyAlias\n            keyPassword hourPathsKeyPassword\n        }\n    }\n\n`;
  updated = updated.slice(0, buildTypesLineStart) + signingBlock + updated.slice(buildTypesLineStart);
  return updated;
}

export async function configureAndroidReleaseSigningFile(path) {
  const source = await readFile(path, 'utf8');
  const updated = configureAndroidReleaseSigning(source);
  if (updated !== source) await writeFile(path, updated, 'utf8');
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  const path = process.argv[2];
  if (!path) throw new Error('usage: configure-android-release-signing.mjs <app/build.gradle>');
  await configureAndroidReleaseSigningFile(path);
}
