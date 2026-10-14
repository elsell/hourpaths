import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { readFileSync } from 'node:fs';
const root = fileURLToPath(new URL('../', import.meta.url));
const require = createRequire(path.join(root, 'apps/mobile/package.json'));
const { build } = createRequire(require.resolve('tsx/package.json'))('esbuild');
const outfile = path.join(root, 'apps/mobile/modules/hourpaths-timers/ios/TimerDomain.js');
const check = process.argv.includes('--check');
const result = await build({
  absWorkingDir: root,
  entryPoints: [path.join(root, 'packages/client-core/src/native-timer-action.ts')],
  outfile, write: !check,
  bundle: true, format: 'iife', globalName: 'HourPathsTimerDomain', platform: 'neutral', target: 'es2020',
});

if (check && readFileSync(outfile, 'utf8') !== result.outputFiles[0].text) throw new Error('Native timer domain bundle is stale; run scripts/build-native-timer-domain.mjs');
