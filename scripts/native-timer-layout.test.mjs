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
      emoji: '🎸', color: '#DFDEFF', goal: { fraction: 0.5, text: '30m of 1h' }, id: `timer-${i}`, name: `Path ${i + 1}`, startedAt: 1_790_000_000_000 + i * 60_000,
    }));
    const remaining = count > 3 ? '1 more timer' : '';
    // JSON is the boundary consumed by the native extension.
    const layout = JSON.parse(JSON.stringify(timerLayout({
      title: `${count} running timers`, count: String(count), timers, remaining,
    })));
    for (const surface of ['banner', 'expandedBottom']) {
      const nodes = descendants(layout[surface]);
      assert.ok(nodes.some(node => node.props?.text === '🎸'), `${surface}: missing Path emoji`);
      assert.ok(nodes.some(node => node.props?.text === '30m of 1h'), `${surface}: missing goal progress`);
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

test('native goal clocks retain bounded count-up intervals through widget serialization', () => {
  const lower = 1_790_000_000_000, upper = lower + 3_600_000;
  const layout = JSON.parse(JSON.stringify(timerLayout({ title: 'Running', count: '1', remaining: '', timers: [{
    id: 'timer', pathId: 'path', name: 'Guitar', emoji: '🎸', startedAt: lower,
    goal: { fraction: 0.5, text: 'Fallback', targetText: 'of 1h this period', lower, upper, progressUpper: upper },
  }] })));
  for (const surface of ['banner', 'expandedBottom']) {
    const nodes = descendants(layout[surface]);
    for (const type of ['TextView', 'ProgressView']) {
      assert.ok(nodes.some(node => node.type === type && node.props.countsDown === false
        && node.props.timerInterval?.lower === lower && node.props.timerInterval?.upper === upper),
      `${surface}: missing native ${type} interval`);
    }
  }
});

test('stopped Path exposes Start and a paused clock rather than a still-running timer', () => {
  const start = 1_790_000_000_000, stop = start + 60000;
  const layout = JSON.parse(JSON.stringify(timerLayout({ title: 'Ready', count: '0', remaining: '', timers: [{
    id: 'timer', pathId: 'path', name: 'Guitar', emoji: '🎸', startedAt: start, stoppedAt: stop,
    action: { target: 'native-action', label: 'Start' },
  }] })));
  for (const surface of ['banner', 'expandedBottom']) {
    const nodes = descendants(layout[surface]);
    assert.ok(nodes.some(node => node.props?.target === 'native-action' && node.props.label === 'Start'));
    assert.ok(nodes.some(node => node.type === 'TextView' && node.props.pauseTime === stop));
  }
  assert.equal(layout.compactTrailing.props.pauseTime, stop);
});

test('large personal labels retain an overflow route within the encoded ActivityKit payload budget', () => {
  const { fitNativePresentation, nativePresentationBytes } = load(path.join(root, 'src/timers/native-presentation-size.ts'));
  const props = { title: 'Running', count: '20', remaining: '17 more', timers: Array.from({ length: 3 }, (_, i) => ({
    id: `timer-${i}`, pathId: `path-${i}`, startedAt: 1_790_000_000_000, name: '🎸'.repeat(240), emoji: '🎸',
    goal: { text: 'Goal', fraction: 0.5, lower: 1_790_000_000_000, upper: 1_790_003_600_000 },
    action: { label: 'Stop', target: JSON.stringify({ owner: 'a'.repeat(128), lease: 'b'.repeat(36), path: `path-${i}`, operation: 'c'.repeat(36) }) },
  })) };
  fitNativePresentation(props, 20, count => `${count} more`);
  assert.ok(nativePresentationBytes(props) <= 3400);
  assert.ok(props.timers.length >= 1 && props.timers.length < 3);
  assert.equal(props.remaining, `${20 - props.timers.length} more`);
});

test('background render advances the goal period without replacing the local Stop action', () => {
  const start = 1_790_000_000_000, boundary = start + 3600000, now = boundary + 300000;
  const props = { title: 'Running', count: '1', remaining: '', timers: [{
    id: 'new-local-timer', pathId: 'path', name: 'Guitar', startedAt: start,
    action: { target: 'current-local-stop', label: 'Stop' },
    goal: { fraction: 0.5, text: 'Old period', targetText: 'of 10m', lower: start, upper: boundary },
    goalPeriods: [
      { startsAt: start, endsAt: boundary, targetSeconds: 600, savedSeconds: 300 },
      { startsAt: boundary, endsAt: boundary + 3600000, targetSeconds: 600, savedSeconds: 120 },
    ],
  }] };
  const nodes = descendants(JSON.parse(JSON.stringify(timerLayout(props, { timestamp: now, isStale: false }))).banner);
  assert.ok(nodes.some(node => node.type === 'TextView' && node.props.timerInterval?.lower === boundary - 120000
    && node.props.timerInterval?.upper === boundary + 3600000));
  assert.ok(nodes.some(node => node.props?.target === 'current-local-stop'));
  const stale = descendants(JSON.parse(JSON.stringify(timerLayout(props, { timestamp: now, isStale: true }))).banner);
  assert.ok(stale.some(node => node.props?.target === 'current-local-stop'));
  assert.ok(!stale.some(node => node.props?.text === 'of 10m'));
});

test('stopped timer in a new period freezes goal progress at rendering time, not the prior period stop', () => {
  const now = 1_790_004_000_000;
  const props = { title: 'Ready', count: '0', remaining: '', timers: [{
    id: 'stopped', pathId: 'path', name: 'Guitar', startedAt: now - 7200000, stoppedAt: now - 3600000,
    action: { target: 'current-start', label: 'Start' }, goal: { fraction: 0, text: '', targetText: 'of 10m' },
    goalPeriods: [{ startsAt: now - 300000, endsAt: now + 3300000, targetSeconds: 600, savedSeconds: 0 }],
  }] };
  const nodes = descendants(JSON.parse(JSON.stringify(timerLayout(props, { timestamp: now, isStale: false }))).banner);
  assert.ok(nodes.some(node => node.type === 'TextView' && node.props.timerInterval?.lower === now && node.props.pauseTime === now));
  assert.ok(nodes.some(node => node.props?.target === 'current-start'));
});


test('Live Activity controls use explicit shared colors and keep goal clocks bounded', () => {
  const props = {title:'Running',count:'1',remaining:'',
    controlAppearance:{startBackground:'#FFD84D',startForeground:'#141414',stopBackground:'#3B2424',stopForeground:'#FF8A80'},
    timers:[{id:'timer',pathId:'path',name:'Test',startedAt:1790000000000,
      action:{target:'stop',label:'Stop'},goal:{fraction:0.5,text:'Goal',targetText:'of 2m',lower:1790000000000,upper:1790003600000,progressUpper:1790000120000}}]};
  const nodes = descendants(JSON.parse(JSON.stringify(timerLayout(props))).expandedBottom);
  const button = nodes.find(node => node.type === 'Button');
  assert.ok(button.props.modifiers.some(mod => mod.$type === 'tint' && mod.color === '#3B2424'));
  const progress = nodes.find(node => node.type === 'ProgressView');
  assert.ok(progress.props.modifiers.some(mod => mod.$type === 'labelsHidden'));
  const clock = nodes.find(node => node.type === 'TextView' && node.props.timerInterval);
  assert.ok(clock.props.modifiers.some(mod => mod.$type === 'frame' && mod.width === 80));
});
