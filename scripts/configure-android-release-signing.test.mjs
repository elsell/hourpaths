import assert from 'node:assert/strict';
import test from 'node:test';
import { configureAndroidReleaseSigning } from './configure-android-release-signing.mjs';

const generatedGradle = `android {
    signingConfigs {
        debug { keyAlias 'androiddebugkey' }
    }
    buildTypes {
        debug {
            signingConfig signingConfigs.debug
        }
        release {
            signingConfig signingConfigs.debug
            minifyEnabled false
        }
    }
}
`;

test('configures only the generated release build for protected upload signing', () => {
  const configured = configureAndroidReleaseSigning(generatedGradle);
  assert.match(configured, /HOURPATHS_ANDROID_UPLOAD_KEYSTORE_PATH/);
  assert.match(configured, /release \{\s+signingConfig signingConfigs\.hourPathsRelease/);
  assert.match(configured, /debug \{\s+signingConfig signingConfigs\.debug/);
  assert.equal(configureAndroidReleaseSigning(configured), configured);
});

test('fails closed when the generated Gradle signing shape drifts', () => {
  assert.throws(
    () => configureAndroidReleaseSigning(generatedGradle.replace(
      'signingConfig signingConfigs.debug\n            minifyEnabled',
      'minifyEnabled',
    )),
    /expected generated debug signing config/,
  );
});
