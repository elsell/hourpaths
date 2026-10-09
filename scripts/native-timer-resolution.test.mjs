import assert from 'node:assert/strict';
import { existsSync, realpathSync, statSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../apps/mobile/', import.meta.url));
const require = createRequire(path.join(root, 'package.json'));
const { getDefaultConfig } = require('expo/metro-config');
const metroRequire = createRequire(require.resolve('expo/metro-config'));
const { resolve } = metroRequire('metro-resolver');
const config = getDefaultConfig(root).resolver;

// Exercise Metro's real resolver against the source tree: extension precedence
// can silently select a no-op adapter even when the native build succeeds.
for (const [platform, filename] of [['ios', 'timer-surface.ios.tsx'], ['android', 'timer-surface.android.ts']]) {
  test(`${platform} bundles the operating-system timer adapter`, () => {
    const result = resolve({
      ...config,
      assetExts: new Set(config.assetExts),
      originModulePath: path.join(root, 'src/timers/native-timer-surfaces.ts'),
      preferNativePlatform: true,
      doesFileExist: existsSync,
      fileSystemLookup(candidate) {
        if (!existsSync(candidate)) return { exists: false };
        return { exists: true, type: statSync(candidate).isDirectory() ? 'd' : 'f', realPath: realpathSync(candidate) };
      },
      getPackageForModule: () => null,
      redirectModulePath: candidate => candidate,
    }, './timer-surface', platform);
    assert.equal(result.type, 'sourceFile');
    assert.equal(result.filePath, path.join(root, 'src/timers', filename));
  });
}
