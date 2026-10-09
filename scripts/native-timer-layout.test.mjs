import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = fileURLToPath(new URL('../apps/mobile/', import.meta.url));
const require = createRequire(path.join(root, 'package.json'));
const { transformSync } = createRequire(require.resolve('tsx/package.json'))('esbuild');
const widgets = path.dirname(require.resolve('expo-widgets/package.json'));
const aliases = {
  expo: path.join(widgets, 'bundle/expo-module-stub.ts'),
  react: path.join(widgets, 'bundle/react-stub.ts'),
  'react/jsx-runtime': path.join(widgets, 'bundle/jsx-runtime-stub.ts'),
  'react/jsx-dev-runtime': path.join(widgets, 'bundle/jsx-runtime-stub.ts'),
};
const modules = new Map();
// Execute the installed widget runtime using expo-widgets/metro.config.js's
// aliases. React's renderer would flatten the arrays that Swift drops.
function load(filename) {
  if (modules.has(filename)) return modules.get(filename).exports;
  const module = { exports: {} };
  modules.set(filename, module);
  const localRequire = createRequire(filename);
  const code = transformSync(readFileSync(filename, 'utf8'), {
    loader: filename.endsWith('.tsx') ? 'tsx' : 'ts',
    format: 'cjs', jsx: 'automatic',
  }).code;
  const runtimeRequire = name => {
    if (name === 'react-native') return {}; // Metro's empty module in widget context.
    if (aliases[name]) return load(aliases[name]);
    let target;
    try { target = localRequire.resolve(name); }
    catch (error) {
      if (!name.startsWith('.')) throw error;
      const base = path.resolve(path.dirname(filename), name);
      target = ['.ts', '.tsx', '/index.ts', '/index.tsx'].map(suffix => base + suffix).find(existsSync);
      if (!target) throw error;
    }
    return load(target);
  };
  vm.runInThisContext(`(function(require,module,exports){${code}\n})`, { filename })(runtimeRequire, module, module.exports);
  return module.exports;
}
const { timerLayout } = load(path.join(root, 'src/timers/ios-timer-layout.tsx'));

function descendants(node) {
  if (!node || typeof node !== 'object' || Array.isArray(node)) return [];
  const children = node.props?.children;
  const list = Array.isArray(children) ? children : [children];
  // DynamicView.swift accepts dictionaries at each immediate child position.
  return [node, ...list.flatMap(descendants)];
}

for (const count of [1, 2, 4]) {
  test(`${count} running timers retain Path names and native clocks on expanded surfaces`, () => {
    const timers = Array.from({ length: Math.min(count, 3) }, (_, i) => ({
      id: `timer-${i}`, name: `Path ${i + 1}`, startedAt: 1_790_000_000_000 + i * 60_000,
    }));
    const remaining = count > 3 ? '1 more timer' : '';
    // JSON is the boundary consumed by the native extension.
    const layout = JSON.parse(JSON.stringify(timerLayout({
      title: `${count} running timers`, count: String(count), timers, remaining,
    })));
    for (const surface of ['banner', 'expandedBottom']) {
      const nodes = descendants(layout[surface]);
      const text = nodes.filter(node => node.type === 'TextView');
      for (const timer of timers) {
        assert.ok(text.some(node => node.props.text === timer.name), `${surface}: missing ${timer.name}`);
        assert.ok(text.some(node => node.props.date === timer.startedAt && node.props.dateStyle === 'timer'),
          `${surface}: missing native clock for ${timer.name}`);
      }
      if (remaining) assert.ok(text.some(node => node.props.text === remaining), `${surface}: missing overflow summary`);
    }
  });
}
