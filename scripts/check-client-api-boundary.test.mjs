#!/usr/bin/env node
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';

const checker = resolve('scripts/check-client-api-boundary.mjs');
const protectedPaths = [
  'apps/mobile/src/provider-auth.ts',
  'apps/mobile/src/provider-auth-state.ts',
  'apps/mobile/src/provider-discovery.ts',
  'apps/mobile/src/push-notifications-native.ts',
  'apps/mobile/src/push-notifications.ts',
  'apps/web/src/lib/provider-auth.ts',
  'packages/api-client/src/index.ts',
  'apps/mobile/src/policy-link-native.ts',
  'apps/mobile/src/policy-link.ts',
  'apps/web/src/lib/accessibility-focus.ts',
  'apps/web/src/lib/device-locale.ts',
  'apps/web/src/lib/external-policy-link.ts',
  'apps/web/src/lib/notification-convergence-browser.ts',
  'scripts/protected-api-client-adapter.sha256',
  'scripts/protected-client-capability-adapters.sha256',
  'scripts/protected-provider-adapters.sha256',
];
const protectedBaseline = Object.fromEntries(protectedPaths.map((path) => [path, readFileSync(path, 'utf8')]));
const appShellBaseline = readFileSync('apps/web/src/app.html', 'utf8');
const workspaceManifestPaths = [
  'packages/api-client/package.json',
  'packages/client-core/package.json',
  'packages/i18n/package.json',
];
const workspaceManifestBaseline = Object.fromEntries(
  workspaceManifestPaths.map((path) => [path, readFileSync(path, 'utf8')]),
);
const regexEscape = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

function check(files, symlinks = {}) {
  const root = mkdtempSync(join(tmpdir(), 'client-api-boundary-'));
  try {
    for (const [relative, source] of Object.entries({
      ...protectedBaseline,
      ...workspaceManifestBaseline,
      'apps/web/src/app.html': appShellBaseline,
      ...files,
    })) {
      const path = join(root, relative);
      mkdirSync(dirname(path), { recursive: true });
      writeFileSync(path, source);
    }
    for (const [relative, target] of Object.entries(symlinks)) {
      const path = join(root, relative);
      mkdirSync(dirname(path), { recursive: true });
      symlinkSync(target, path);
    }
    return spawnSync(process.execPath, [checker, root], { encoding: 'utf8' });
  } finally { rmSync(root, { recursive: true, force: true }); }
}

const bypasses = {
  'apps/web/src/lib/direct.ts': "fetch(apiURL + '/v1/me');",
  'apps/web/src/lib/computed.ts': "(globalThis as any)['fe' + 'tch'](apiURL + '/v1/me');",
  'apps/web/src/lib/window.ts': "window['fetch']('/v1/me');",
  'apps/web/src/lib/window-alias.ts': "const transport = window; transport.fetch('/v1/me');",
  'apps/web/src/lib/window-destructure.ts': "const { fetch: send } = window; send('/v1/me');",
  'apps/web/src/lib/document-default-view.ts': "const transport = document.defaultView; transport?.fetch('/v1/me');",
  'apps/web/src/lib/frames.ts': "const transport = frames[0]; transport.fetch('/v1/me');",
  'apps/web/src/lib/top.ts': "const transport = top; transport?.fetch('/v1/me');",
  'apps/web/src/lib/parent.ts': "const transport = parent; transport.fetch('/v1/me');",
  'apps/web/src/lib/opener-assignment.ts': "let transport; transport = opener; transport?.fetch('/v1/me');",
  'apps/web/src/lib/proxy-reassignment.ts': "let transport = window.location; transport = parent; transport.fetch('/v1/me');",
  'apps/web/src/lib/scoped-shadow.ts': "function shadow(document) { return document; } const transport = document.defaultView; transport?.fetch('/v1/me');",
  'apps/web/src/lib/before-shadow.ts': "const transport = window; function unrelated(window) { return window; } transport.fetch('/v1/me');",
  'apps/web/src/lib/sibling-shadow.ts': "{ const window = {}; void window; } const transport = window; transport.fetch('/v1/me');",
  'apps/web/src/lib/event-view.ts': "export function leak(event) { const transport = event.view; transport?.fetch('/v1/me'); }",
  'apps/web/src/lib/open-alias.ts': "const createWindow = open; const transport = createWindow('/child'); transport?.fetch('/v1/me');",
  'apps/web/src/lib/open-assignment.ts': "let createWindow; createWindow = open; const transport = createWindow('/child'); transport?.fetch('/v1/me');",
  'apps/web/src/lib/fetch-alias.ts': "const send = fetch; send('/v1/me');",
  'apps/web/src/lib/fetch-assignment.ts': "let send; send = fetch; send('/v1/me');",
  'apps/web/src/lib/xhr-alias.ts': "const Transport = XMLHttpRequest; new Transport();",
  'apps/web/src/lib/websocket-assignment.ts': "let Transport; Transport = WebSocket; new Transport('/v1/me');",
  'apps/web/src/lib/eventsource-alias.ts': "const Transport = EventSource; new Transport('/v1/me');",
  'apps/web/src/lib/request-assignment.ts': "let Transport; Transport = Request; new Transport('/v1/me');",
  'apps/web/src/lib/webtransport-alias.ts': "const Transport = WebTransport; new Transport('/v1/me');",
  'apps/web/src/lib/worker-assignment.ts': "let Transport; Transport = Worker; new Transport('/worker.js');",
  'apps/web/src/lib/broadcast-channel.ts': "new BroadcastChannel('notification-history');",
  'apps/web/src/lib/shared-worker-alias.ts': "const Transport = SharedWorker; new Transport('/worker.js');",
  'apps/web/src/lib/rtc-assignment.ts': "let Transport; Transport = RTCPeerConnection; new Transport();",
  'apps/web/src/lib/beacon-alias.ts': "const send = sendBeacon; send('/v1/me');",
  'apps/web/src/lib/computed-primitive-key.ts': "const adapter = { [fetch]() { return 'network'; } }; adapter[fetch]();",
  'apps/web/src/lib/shorthand-primitive.ts': "const adapter = { fetch }; adapter.fetch('/v1/me');",
  'apps/web/src/lib/ambient-const.ts': "declare const fetch: (url: string) => Promise<unknown>; const send = fetch; send('/v1/me');",
  'apps/web/src/lib/ambient-function.ts': "declare function fetch(url: string): Promise<unknown>; const send = fetch; send('/v1/me');",
  'apps/web/src/lib/ambient-class.ts': "declare class XMLHttpRequest { open(): void; } const Transport = XMLHttpRequest; new Transport();",
  'apps/web/src/lib/ambient-global.ts': "export {}; declare global { const fetch: (url: string) => Promise<unknown>; } const send = fetch; send('/v1/me');",
  'apps/web/src/lib/dotted-namespace.ts': "namespace Local.WebSocket { export const local = 'yes'; } new WebSocket('/v1/me');",
  'apps/web/src/lib/export-named.ts': "export { default as send } from 'openapi-fetch';",
  'apps/mobile/src/export-star.ts': "export * from 'openapi-fetch';",
  'apps/web/src/lib/import-equals.ts': "import transport = require('openapi-fetch'); transport('/v1/me');",
  'apps/mobile/src/require-alias.cjs': "const load = require; const transport = load('openapi-fetch'); transport('/v1/me');",
  'apps/mobile/src/beacon.ts': "navigator.sendBeacon('/v1/session');",
  'apps/mobile/src/dynamic.ts': "import('ax' + 'ios').then((transport) => transport.default(apiURL));",
  'apps/mobile/src/imported.ts': "import transport from 'openapi-fetch'; transport('/v1/me');",
  'apps/mobile/src/alias.ts': "import transport from '$shared/transport'; transport('/v1/me');",
  'apps/mobile/src/escaped.ts': "import { send } from '../../shared/transport'; send('/v1/me');",
  'apps/shared/transport.ts': "export const send = (url) => fetch(url);",
  'apps/mobile/src/same-dir.mjs': "export const send = (url) => fetch(url);",
  'apps/mobile/src/import-cjs.ts': "import { send } from './transport.cjs'; send('/v1/me');",
  'apps/mobile/src/transport.cjs': "exports.send = (url) => fetch(url);",
  'apps/web/src/lib/alias-cts.ts': "import { send } from '$lib/transport.cts'; send('/v1/me');",
  'apps/web/src/lib/transport.cts': "export const send = (url) => fetch(url);",
  'apps/web/src/lib/missing-alias.ts': "import { send } from '$lib/missing'; send('/v1/me');",
  'packages/client-core/src/transport.ts': "export const send = (url) => fetch(url);",
  'packages/i18n/src/transport.ts': "export const send = (url) => fetch(url);",
};
let result = check(bypasses);
assert.notEqual(result.status, 0, result.stderr);
for (const path of Object.keys(bypasses).filter((path) =>
  !path.startsWith('apps/shared/') &&
  path !== 'apps/mobile/src/import-cjs.ts' &&
  path !== 'apps/web/src/lib/alias-cts.ts')) {
  assert.match(result.stderr, new RegExp(path.replaceAll('/', '\\/')));
}

result = check({
  'apps/mobile/app/index.tsx': "import { Button, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native'; export default function IndexRedirect() { return null; } export function HomeScreen() { return <></>; }",
  'apps/mobile/app/(tabs)/following/_layout.tsx': "import { Stack } from 'expo-router'; export default function Layout() { return <Stack />; }",
  'apps/mobile/app/(tabs)/following/index.tsx': "import { router, Stack, useFocusEffect } from 'expo-router'; import { AppState } from 'react-native'; export function open() { router.push('/profile/person'); } export default function Page() { useFocusEffect(() => {}); AppState.addEventListener('change', () => undefined); return <Stack.Screen />; }",
  'apps/mobile/app/profile/[username].tsx': "import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router'; import { AccessibilityInfo, Alert } from 'react-native'; export default function Profile() { useFocusEffect(() => {}); useLocalSearchParams(); router.back(); Alert.alert('x'); AccessibilityInfo.announceForAccessibility('x'); return <Stack.Screen />; }",
  'apps/mobile/app/settings/blocked-accounts.tsx': "import { router, useFocusEffect } from 'expo-router'; import { AccessibilityInfo, Alert } from 'react-native'; export default function BlockedAccounts() { useFocusEffect(() => {}); router.replace('/(tabs)/home'); Alert.alert('x'); AccessibilityInfo.announceForAccessibility('x'); return null; }",
  'apps/mobile/src/config.ts': "export const config = 'safe';",
  'apps/mobile/src/config.test.ts': "import { config } from './config.js'; export { config };",
  'apps/mobile/src/ui/social-profile-avatar.ios.tsx': "import { Image as SwiftUIImage } from '@expo/ui/swift-ui'; export const Avatar = () => <><SwiftUIImage systemName='person.crop.circle.fill' /></>;",
  'apps/web/src/lib/generated.ts': "import { createSessionApiClient } from '@hourpaths/api-client'; export const session = createSessionApiClient(base, token);",
});
assert.equal(result.status, 0, result.stderr);

for (const [scenario, files, expected] of [
  ['renamed-api-client', {
    'packages/api-client/package.json': '{"name":"@attacker/api-client"}',
    'apps/web/src/lib/generated.ts': "import client from '@attacker/api-client'; export { client };",
  }, 'packages/api-client/package.json'],
  ['swapped-workspace-name', {
    'packages/api-client/package.json': '{"name":"@hourpaths/client-core"}',
  }, 'packages/api-client/package.json'],
  ['renamed-client-core', {
    'packages/client-core/package.json': '{"name":"@hourpaths/client-core-renamed"}',
  }, 'packages/client-core/package.json'],
]) {
  result = check(files);
  assert.notEqual(result.status, 0, `${scenario}: ${result.stderr}`);
  assert.match(result.stderr, new RegExp(regexEscape(expected)), scenario);
}

const apiNavigationBypasses = {
  'apps/web/src/lib/location-href.ts': "window.location.href = '/v1/me';",
  'apps/web/src/lib/location-direct.ts': "window.location = '/v1/me';",
  'apps/web/src/lib/location-assign.ts': "window.location.assign('/v1/sessions');",
  'apps/web/src/lib/location-replace.ts': "window.location.replace(apiURL + '/v1/me');",
  'apps/web/src/lib/goto-api.ts': "import { goto } from '@sveltejs/kit'; void goto('/v1/me');",
  'apps/web/src/lib/goto-alias.ts': "import { goto as navigate } from '@sveltejs/kit'; void navigate(apiURL + '/v1/me');",
  'apps/web/src/lib/goto-namespace.ts': "import * as kit from '@sveltejs/kit'; void kit.goto('/v1/me');",
  'apps/web/src/lib/location-alias.ts': "const location = window.location; location.assign('/v1/me');",
  'apps/web/src/lib/location-value.ts': "const apiPath = '/v1/me'; window.location.assign(apiPath);",
  'apps/web/src/lib/goto-value-alias.ts': "import { goto } from '@sveltejs/kit'; const navigate = goto; navigate('/v1/me');",
  'apps/web/src/lib/goto-callback.ts': "import { goto } from '@sveltejs/kit'; function invoke(navigate) { navigate('/v1/me'); } invoke(goto);",
  'apps/web/src/lib/goto-callback-value.ts': "import { goto } from '@sveltejs/kit'; const invoke = (navigate, target) => navigate(target); invoke(goto, '/v1/me');",
  'apps/web/src/lib/assigned-aliases.ts': "let location; location = window.location; const first = '/v1/me'; const target = first; location.assign(target);",
  'apps/web/src/lib/destructured-location.ts': "const { assign: navigate } = window.location; navigate('/v1/me');",
  'apps/web/src/lib/returned-location.ts': "function browserLocation() { return window.location; } browserLocation().assign('/v1/me');",
  'apps/web/src/lib/returned-destination.ts': "function apiPath() { return '/v1/me'; } window.location.assign(apiPath());",
  'apps/web/src/lib/callback-dispatcher-alias.ts': "import { goto } from '@sveltejs/kit'; const dispatch = (navigate, target) => navigate(target); const alias = dispatch; alias(goto, '/v1/me');",
  'apps/web/src/lib/destructuring-assignment.ts': "let navigate; ({ assign: navigate } = window.location); navigate('/v1/me');",
  'apps/web/src/routes/action/+page.svelte': '<form action="/v1/sessions" method="post"><button>Submit</button></form>',
  'apps/web/src/routes/formaction/+page.svelte': '<form><button formaction="/v1/sessions">Submit</button></form>',
  'apps/web/src/routes/expression/+page.svelte': "<form action={apiURL + '/v1/sessions'}></form>",
  'apps/web/src/routes/bound-action/+page.svelte': "<script>const target = '/v1/sessions';</script><form action={target}></form>",
  'apps/web/src/routes/api-link/+page.svelte': '<a href="/v1/me">Account</a>',
  'apps/web/src/routes/shorthand-link/+page.svelte': "<script>const href = '/v1/me';</script><a {href}>Account</a>",
  'apps/web/src/routes/entity-link/+page.svelte': '<a href="&#47;v1&#47;me">Account</a>',
  'apps/web/src/routes/spread-link/+page.svelte': "<script>const attributes = { href: '/v1/me' };</script><a {...attributes}>Account</a>",
  'apps/web/src/routes/direct-spread-link/+page.svelte': "<a {...{ href: '/v1/me' }}>Account</a>",
  'apps/web/src/routes/api-image/+page.svelte': '<img src="/v1/me" alt="">',
  'apps/web/src/lib/api-link.tsx': "export const Link = () => <a href='/v1/me'>Account</a>;",
  'apps/web/src/lib/unused-api-destination.ts': "export const endpoint = '/v1/me';",
  'apps/web/src/lib/composed-api-destination.ts': "const version = 1; export const endpoint = `/v${version}/me`;",
  'apps/web/src/lib/encoded-api-destination.ts': String.raw`export const endpoint = '\x2fv\x31/me';`,
  'apps/web/src/lib/dynamic-api-template.ts': "export function navigate(id) { window.location.assign(`/v1/${id}`); }",
  'apps/web/src/lib/dynamic-api-link.tsx': "export const Link = ({ id }) => <a href={`/v1/${id}`}>Account</a>;",
  'apps/web/src/lib/composed-dynamic-template.ts': "const slash = '/'; const version = 'v1'; export function navigate(id) { window.location.assign(`${slash}${version}/${id}`); }",
  'apps/web/src/lib/middle-dynamic-template.ts': "export function navigate(origin, id) { window.location.assign(`${origin}/v1/${id}`); }",
  'apps/web/src/lib/tail-dynamic-template.ts': "export function navigate(origin) { window.location.assign(`${origin}/v1/`); }",
  'apps/web/src/lib/dynamic-join-destination.ts': "export function navigate(id) { const target = ['', 'v1', id].join('/'); window.location.assign(target); }",
  'apps/web/src/lib/constant-join-destination.ts': "const target = ['', 'v1', 'me'].join('/'); window.location.assign(target);",
  'apps/web/src/lib/constant-concat-destination.ts': "const target = '/'.concat('v1', '/me'); window.location.assign(target);",
  'apps/web/src/lib/string-raw-destination.ts': "const target = String.raw({ raw: ['/', '1/me'] }, 'v'); window.location.assign(target);",
  'apps/web/src/lib/string-code-destination.ts': 'const target = String.fromCharCode(47, 118, 49, 47, 109, 101); window.location.assign(target);',
  'apps/web/src/lib/unknown-navigation.ts': 'export function navigate(target) { window.location.assign(target); }',
  'apps/web/src/lib/unknown-navigation-method.ts': 'export function navigate(method, target) { window.location[method](target); }',
  'apps/web/src/lib/unknown-goto.ts': "import { goto } from '@sveltejs/kit'; export function navigate(target) { void goto(target); }",
  'apps/web/src/routes/onclick-api/+page.svelte': "<button onclick={() => fetch('/v1/me')}>Load</button>",
  'apps/web/src/routes/onclick-location/+page.svelte': "<button onclick={() => window.location.assign('/v1/me')}>Load</button>",
  'apps/web/src/routes/legacy-onclick-api/+page.svelte': "<button on:click={() => fetch('/v1/me')}>Load</button>",
  'apps/web/src/routes/quoted-angle-action/+page.svelte': '<form title=">" action="/v1/sessions"></form>',
  'apps/web/src/routes/quoted-script/+page.svelte': '<script data-x=">//">fetch("/v1/me")</script><p>x</p>',
  'apps/web/src/routes/quoted-module-script/+page.svelte': '<script module data-x=">//">fetch("/v1/me")</script><p>x</p>',
  'apps/web/src/routes/quoted-script-exact/+page.svelte': '<script data-x=">">fetch("/v1/me")</script><p>x</p>',
  'apps/web/src/routes/quoted-context-module/+page.svelte': '<script context="module" data-x=">">fetch("/v1/me")</script><p>x</p>',
  'apps/web/src/routes/api-style-block/+page.svelte': "<style>.probe{background:url('/v1/me')}</style>",
  'apps/web/src/routes/api-style-attribute/+page.svelte': "<div style=\"background-image:url('/v1/me')\"></div>",
  'apps/web/src/routes/api-meta-refresh/+page.svelte': '<svelte:head><meta http-equiv="refresh" content="0;url=/v1/me"></svelte:head>',
  'apps/web/src/routes/api-ping/+page.svelte': '<a href="/paths" ping="/v1/me">Path</a>',
  'apps/web/src/routes/api-xlink/+page.svelte': '<svg><use xlink:href="/v1/me"></use></svg>',
  'apps/web/src/routes/api-image-preload/+page.svelte': '<svelte:head><link rel="preload" as="image" imagesrcset="/v1/me 1x"></svelte:head>',
  'apps/web/src/routes/unknown-href/+page.svelte': '<script>export let target;</script><a href={target}>Go</a>',
  'apps/web/src/routes/unknown-component-prop/+page.svelte': '<script>export let target;</script><Nav {target}/>',
  'apps/web/src/lib/unknown-href.tsx': 'export const Link = ({ target }) => <a href={target}>Go</a>;',
  'apps/web/src/lib/unknown-version-template.ts': "export function navigate(version, id) { window.location.assign(`/v${version}/${id}`); }",
  'apps/web/src/lib/backslash-api-navigation.ts': "window.location.assign('\\\\v1\\\\me');",
  'apps/web/src/lib/javascript-navigation.ts': "window.location.assign('javascript:alert(1)');",
  'apps/web/src/lib/data-navigation.ts': "window.location.assign('data:text/html,unsafe');",
  'apps/web/src/lib/blob-navigation.ts': "window.location.assign('blob:https://example.test/id');",
  'apps/web/src/lib/file-navigation.ts': "window.location.assign('file:///tmp/unsafe');",
  'apps/web/src/lib/custom-scheme-navigation.ts': "window.location.assign('hourpaths:unsafe');",
  'apps/web/src/routes/backslash-link/+page.svelte': '<a href="\\v1\\me">Account</a>',
  'apps/web/src/routes/javascript-link/+page.svelte': '<a href="javascript:alert(1)">Account</a>',
  'apps/web/src/lib/data-link.tsx': "export const Link = () => <a href='data:text/html,unsafe'>Account</a>;",
};
result = check(apiNavigationBypasses);
assert.notEqual(result.status, 0, result.stderr);
for (const path of Object.keys(apiNavigationBypasses)) {
  assert.match(result.stderr, new RegExp(regexEscape(path)), path);
}

result = check({
  'apps/web/src/lib/local-navigation.ts': "function local(window) { window.location.href = '/paths'; window.location.assign('/paths'); } export { local };",
  'apps/web/src/lib/page-link.tsx': "export const Link = () => <a href='/paths'>Paths</a>;",
  'apps/web/src/lib/https-link.tsx': "export const Link = () => <a href='https://identity.example/authorize'>Identity</a>;",
  'apps/web/src/routes/search/+page.svelte': '<form action="/search" method="get"><button>Search</button></form>',
  'apps/web/src/routes/bound-search/+page.svelte': "<script>const target = '/search';</script><form action={target}></form><a href={target}>Search</a>",
  'apps/web/src/routes/search-image/+page.svelte': '<img src="/logo.svg" alt="">',
  'apps/web/src/routes/https-link/+page.svelte': '<a href="https://identity.example/authorize">Identity</a>',
  'apps/web/src/routes/onclick-page/+page.svelte': "<script>const local = (value) => value;</script><button onclick={() => local('/paths')}>Load</button><button on:click={() => local('/paths')}>Legacy</button>",
  'apps/web/src/routes/quoted-angle-search/+page.svelte': '<form title=">" action="/search"></form>',
  'apps/web/src/routes/quoted-page-script/+page.svelte': '<script data-x=">//">const target = "/paths";</script><p>{target}</p>',
  'apps/web/src/routes/page-style-block/+page.svelte': "<style>.probe{background:url('/logo.svg')}</style>",
  'apps/web/src/routes/page-style-attribute/+page.svelte': "<div style=\"background-image:url('/logo.svg')\"></div>",
  'apps/web/src/routes/page-ping/+page.svelte': '<a href="/paths" ping="/telemetry">Path</a>',
  'apps/web/src/routes/page-xlink/+page.svelte': '<svg><use xlink:href="/icons.svg#path"></use></svg>',
  'apps/web/src/routes/page-image-preload/+page.svelte': '<svelte:head><link rel="preload" as="image" imagesrcset="/images/path.webp 1x"></svelte:head>',
});
assert.equal(result.status, 0, `legitimate-navigation: ${result.stderr}`);

for (const [scenario, shell] of Object.entries({
  script: '<!doctype html><html><head></head><body><script>fetch("/v1/me")</script>%sveltekit.body%</body></html>',
  handler: '<!doctype html><html><head></head><body><button onclick="fetch(\'/v1/me\')">Go</button>%sveltekit.body%</body></html>',
  request: '<!doctype html><html><head></head><body><img src="/v1/me">%sveltekit.body%</body></html>',
  style: '<!doctype html><html><head><style>body{background:url("/v1/me")}</style></head><body>%sveltekit.body%</body></html>',
  refresh: '<!doctype html><html><head><meta http-equiv="refresh" content="0;url=/v1/me"></head><body>%sveltekit.body%</body></html>',
  malformed: '<!doctype html><html><head><script>',
})) {
  result = check({ 'apps/web/src/app.html': shell });
  assert.notEqual(result.status, 0, `app-shell-${scenario}: ${result.stderr}`);
  assert.match(result.stderr, /apps\/web\/src\/app\.html/, scenario);
}

result = check({
  'apps/web/static/raw.js': "fetch('/v1/me');",
  'apps/web/static/logo.svg': '<svg xmlns="http://www.w3.org/2000/svg"></svg>',
});
assert.notEqual(result.status, 0, `static-executable-asset: ${result.stderr}`);
assert.match(result.stderr, /apps\/web\/static\/raw\.js/);

for (const [relative, source] of Object.entries({
  'apps/web/static/event.svg': '<svg xmlns="http://www.w3.org/2000/svg" onload="fetch(\'/v1/me\')"></svg>',
  'apps/web/static/script.svg': '<svg xmlns="http://www.w3.org/2000/svg"><script>fetch("/v1/me")</script></svg>',
  'apps/web/static/request.svg': '<svg xmlns="http://www.w3.org/2000/svg"><image href="/v1/me"/></svg>',
  'apps/web/static/raw.xhtml': '<html xmlns="http://www.w3.org/1999/xhtml"><script>fetch("/v1/me")</script></html>',
  'apps/web/static/raw.xml': '<?xml version="1.0"?><root><script>fetch("/v1/me")</script></root>',
})) {
  result = check({ [relative]: source });
  assert.notEqual(result.status, 0, `active-static-${relative}: ${result.stderr}`);
  assert.match(result.stderr, new RegExp(regexEscape(relative)), relative);
}

result = check({
  'apps/web/static/logo.png': 'inert-raster-fixture',
  'apps/web/static/app.woff2': 'inert-font-fixture',
});
assert.equal(result.status, 0, `inert-static-assets: ${result.stderr}`);

for (const source of ['{@html payload}', '{@html "<strong>safe</strong>"}']) {
  result = check({ 'apps/web/src/routes/raw-html/+page.svelte': source });
  assert.notEqual(result.status, 0, `raw-html-policy: ${result.stderr}`);
  assert.match(result.stderr, /apps\/web\/src\/routes\/raw-html\/\+page\.svelte/);
}

const forbiddenPresentationCapabilities = {
  'apps/web/src/lib/location-page.ts': "window.location.assign('/paths');",
  'apps/web/src/lib/goto-page.ts': "import { goto } from '@sveltejs/kit'; void goto('/paths');",
  'apps/web/src/lib/goto-alias-page.ts': "import { goto as frameworkGoto } from '@sveltejs/kit'; const jump = frameworkGoto; jump('/paths');",
  'apps/web/src/lib/dynamic-page.ts': 'export function link(id) { window.location.assign(`/paths/${id}`); }',
  'apps/web/src/lib/static-location.ts': "window.location.assign('/paths');",
  'apps/web/src/lib/dynamic-page-link.tsx': 'export const Link = ({ id }) => <a href={`/paths/${id}`}>Path</a>;',
  'apps/web/src/routes/bind-this/+page.svelte': '<script>let form;</script><form bind:this={form}></form>',
  'apps/web/src/routes/dynamic-style/+page.svelte': '<script>export let target;</script><div style={target}></div>',
  'apps/web/src/routes/style-directive/+page.svelte': '<script>export let target;</script><div style:background-image={target}></div>',
  'apps/web/src/routes/dynamic-refresh/+page.svelte': '<script>export let target;</script><svelte:head><meta http-equiv="refresh" content={target}></svelte:head>',
  'apps/web/src/routes/spread/+page.svelte': '<script>export let attrs;</script><a {...attrs}>Link</a>',
  'apps/mobile/src/router.ts': "import { router } from 'expo-router'; export const go = () => router.push('/paths');",
  'apps/mobile/src/linking.ts': "import * as Linking from 'expo-linking'; export const go = () => Linking.openURL('https://identity.example/authorize');",
  'apps/mobile/src/prefetch.tsx': "import { Image } from 'react-native'; export const load = () => Image.prefetch('https://images.example/path.png');",
  'apps/mobile/src/image.tsx': "import { Image } from 'react-native'; export const Logo = () => <Image source={{ uri: '/logo.png' }} />;",
  'apps/mobile/src/spread.tsx': "import { Image } from 'react-native'; export const Logo = ({ props }) => <Image {...props} />;",
};
result = check(forbiddenPresentationCapabilities);
assert.notEqual(result.status, 0, result.stderr);
for (const relative of Object.keys(forbiddenPresentationCapabilities)) {
  assert.match(result.stderr, new RegExp(regexEscape(relative)), relative);
}

result = check({
  'apps/web/src/routes/callback/+page.svelte': '<script>window.location.replace("/");</script>',
  'apps/web/src/routes/static-style/+page.svelte': '<div style="color:red"></div>',
});
assert.equal(result.status, 0, `reviewed-presentation-capabilities: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/progress-indicator.tsx': "import { ActivityIndicator, StyleSheet, Text, View } from 'react-native'; const styles = StyleSheet.create({ root: { gap: 8 } }); export const Progress = () => <View style={styles.root}><ActivityIndicator /><Text>value</Text></View>;",
  'apps/mobile/src/ui/native-route-presentation.tsx': "import { RefreshControl, ScrollView } from 'react-native'; export const Route = ({ refresh }) => <ScrollView refreshControl={<RefreshControl refreshing={false} onRefresh={refresh} />} />;",
  'apps/mobile/app/notifications.tsx': "import { router, useFocusEffect } from 'expo-router'; import { AppState } from 'react-native'; export default function Notifications() { useFocusEffect(() => undefined); AppState.addEventListener('change', () => undefined); router.replace('/(tabs)/home'); return null; }",
});
assert.equal(result.status, 0, `reviewed-mobile-ui-presentation-primitives: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/action.tsx': "import { Pressable, StyleSheet, Text } from 'react-native'; const styles = StyleSheet.create({ root: { minHeight: 48 } }); export const Action = () => <Pressable accessibilityRole='button' style={styles.root}><Text>value</Text></Pressable>;",
  'apps/mobile/src/ui/settings-list.tsx': "import { StyleSheet, Switch, View } from 'react-native'; export const Setting = () => <View><Switch value={false} /><View style={StyleSheet.create({ row: {} }).row} /></View>;",
});
assert.equal(result.status, 0, `reviewed-mobile-ui-interaction-primitives: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/onboarding-form.tsx': "import { StyleSheet, Switch, View, useWindowDimensions } from 'react-native'; export const OnboardingForm = () => { const { width } = useWindowDimensions(); return <View style={{ width }}><Switch value={false} /><View style={StyleSheet.create({ field: {} }).field} /></View>; };",
});
assert.equal(result.status, 0, `reviewed-native-onboarding-presentation: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/path-create-form.tsx': "import { AccessibilityInfo, InputAccessoryView, Keyboard, Pressable, StyleSheet, Switch, View } from 'react-native'; export const PathCreateForm = () => <Pressable><View><Switch value={false} /></View></Pressable>; void AccessibilityInfo; void InputAccessoryView; void Keyboard;",
  'apps/mobile/src/ui/native-segmented-control.tsx': "import SegmentedControl from '@expo/ui/community/segmented-control'; export const Unit = () => <SegmentedControl values={['Minutes', 'Hours']} />;",
  'apps/mobile/src/ui/native-choice-picker.ios.tsx': "import { Picker, Text } from '@expo/ui/swift-ui'; import { accessibilityLabel, disabled, environment, pickerStyle, tag, tint } from '@expo/ui/swift-ui/modifiers'; export const Choice = () => <><Picker modifiers={[accessibilityLabel('Repeat'), disabled(false), environment('colorScheme', 'dark'), pickerStyle('menu'), tint('#FFD84D')]}><Text modifiers={[tag('daily')]}>Daily</Text></Picker></>;",
});
assert.equal(result.status, 0, `reviewed-native-path-create-presentation: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/comment-edit-sheet.tsx': "import { AccessibilityInfo, Alert, InputAccessoryView, Keyboard, StyleSheet, View } from 'react-native'; export const Edit = () => <View />; void AccessibilityInfo; void Alert; void InputAccessoryView; void Keyboard; void StyleSheet;",
  'apps/mobile/src/ui/practice-comments-view.tsx': "import { ActivityIndicator, Alert, FlatList, InputAccessoryView, Keyboard, KeyboardAvoidingView, Platform, Pressable, StyleSheet, View, useWindowDimensions } from 'react-native'; export const Comments = () => <KeyboardAvoidingView><FlatList data={[]} renderItem={() => <Pressable><View><ActivityIndicator /></View></Pressable>} /></KeyboardAvoidingView>; void Alert; void InputAccessoryView; void Keyboard; void Platform; void StyleSheet; void useWindowDimensions;",
  'apps/mobile/src/ui/social-reaction-menu.tsx': "import { AccessibilityInfo, Modal, Pressable, ScrollView, StyleSheet, View } from 'react-native'; export const Reactions = () => <Modal><ScrollView><Pressable><View /></Pressable></ScrollView></Modal>; void AccessibilityInfo; void StyleSheet;",
  'apps/mobile/src/ui/social-reaction-menu.ios.tsx': "import { Button, Image as SwiftUIImage, Menu } from '@expo/ui/swift-ui'; import { accessibilityLabel as nativeAccessibilityLabel, disabled as nativeDisabled, frame, tint } from '@expo/ui/swift-ui/modifiers'; export const Reactions = () => <><Menu label={<SwiftUIImage systemName='heart' />} modifiers={[nativeAccessibilityLabel('Reactions'), nativeDisabled(false), frame({ height: 44 }), tint('#FFD84D')]}><Button label='Heart' /></Menu></>;",
});
assert.equal(result.status, 0, `reviewed-native-comment-interactions: ${result.stderr}`);

result = check({
  'apps/mobile/src/social-operation-admission.ts': "export const admission = Symbol('social-operation');",
});
assert.equal(result.status, 0, `capability-free-symbol-admission: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/home-filter-chips.ios.tsx': "import { Button, HStack, ScrollView } from '@expo/ui/swift-ui'; import { accessibilityLabel, accessibilityValue, buttonStyle, controlSize, environment, tint } from '@expo/ui/swift-ui/modifiers'; export const Filters = () => <><ScrollView><HStack><Button label='All' modifiers={[accessibilityLabel('All'), accessibilityValue('Selected'), buttonStyle('bordered'), controlSize('regular'), environment('colorScheme', 'dark'), tint('#FFD84D')]} /></HStack></ScrollView></>;",
  'apps/mobile/src/ui/home-arrangement-view.ios.tsx': "import { Button, HStack, List, Section, Spacer, Text } from '@expo/ui/swift-ui'; import { accessibilityLabel, buttonStyle, disabled, environment, listStyle, tint } from '@expo/ui/swift-ui/modifiers'; export const Arrangement = () => <><List modifiers={[disabled(false), environment('colorScheme', 'dark'), listStyle('insetGrouped'), tint('#FFD84D')]}><Section><HStack><Text>Path</Text><Spacer /><Button label='Pin' modifiers={[accessibilityLabel('Pin'), buttonStyle('borderless')]} /></HStack></Section></List></>;",
});
assert.equal(result.status, 0, `reviewed-native-home-organization-presentation: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/signed-out-screen.tsx': "import { ActivityIndicator, ScrollView, StyleSheet, View, useWindowDimensions } from 'react-native'; export const SignedOutScreen = () => { const { width } = useWindowDimensions(); return <ScrollView><View style={{ width }}><ActivityIndicator /></View></ScrollView>; };",
  'apps/mobile/src/ui/native-button.ios.tsx': "import { Button, Text } from '@expo/ui/swift-ui'; import { accessibilityLabel, buttonStyle, controlSize, disabled, foregroundColor, frame, tint } from '@expo/ui/swift-ui/modifiers'; export const Primary = () => <><Button modifiers={[accessibilityLabel('Sign in'), buttonStyle('borderedProminent'), controlSize('large'), disabled(false), foregroundColor('#000000'), tint('#FFD84D')]}><Text modifiers={[frame({ width: 300 }), foregroundColor('#000000')]}>Sign in</Text></Button></>;",
});
assert.equal(result.status, 0, `reviewed-native-signed-out-presentation: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/native-content-unavailable.ios.tsx': "import { ContentUnavailableView } from '@expo/ui/swift-ui'; export const Empty = () => <><ContentUnavailableView title='No activity' /></>;",
});
assert.equal(result.status, 0, `reviewed-native-empty-state-presentation: ${result.stderr}`);

for (const source of [
  "import SwiftUI from '@expo/ui/swift-ui'; export const Empty = SwiftUI;",
  "import * as SwiftUI from '@expo/ui/swift-ui'; export const Empty = SwiftUI;",
  "import { Button, ContentUnavailableView } from '@expo/ui/swift-ui'; export const Empty = () => <><Button label='Retry' /><ContentUnavailableView /></>;",
  "import { tint } from '@expo/ui/swift-ui/modifiers'; export const Empty = tint('#FFD84D');",
  "export { Button } from '@expo/ui/swift-ui';",
]) {
  result = check({ 'apps/mobile/src/ui/native-content-unavailable.ios.tsx': source });
  assert.notEqual(result.status, 0, `unreviewed-native-empty-state-capability: ${source}`);
}

result = check({
  'apps/mobile/src/ui/native-timer-button.ios.tsx': "import { Button } from '@expo/ui/swift-ui'; import { accessibilityLabel, buttonStyle, controlSize, disabled, foregroundColor, frame, tint } from '@expo/ui/swift-ui/modifiers'; export const Timer = () => <><Button label='Start timer' modifiers={[accessibilityLabel('Start timer'), buttonStyle('borderedProminent'), controlSize('large'), frame({ maxWidth: Number.POSITIVE_INFINITY, minHeight: 48 }), disabled(false), foregroundColor('#000000'), tint('#FFD84D')]} /></>;",
  'apps/mobile/src/ui/native-tracking-button.ios.tsx': "import { Button, Image as SwiftUIImage, VStack } from '@expo/ui/swift-ui'; import { accessibilityLabel, buttonStyle, clipShape, controlSize, disabled, frame, tint } from '@expo/ui/swift-ui/modifiers'; import { StyleSheet, View, useWindowDimensions } from 'react-native'; export const Tracking = () => <View><><Button label='Stop timer' modifiers={[accessibilityLabel('Stop timer'), buttonStyle('borderedProminent'), clipShape('circle'), controlSize('large'), disabled(false), frame({ height: 76, width: 76 }), tint('#FFD84D')]}><VStack><SwiftUIImage systemName='stop.fill' /></VStack></Button></></View>;",
  'apps/mobile/src/ui/manual-occurrence-fields.ios.tsx': "import { DatePicker } from '@expo/ui/swift-ui'; import { datePickerStyle, disabled, environment, tint } from '@expo/ui/swift-ui/modifiers'; export const Occurrence = () => <><DatePicker modifiers={[datePickerStyle('compact'), disabled(false), environment('timeZone', 'UTC'), tint('#FFD84D')]} /></>;",
});
assert.equal(result.status, 0, `reviewed-native-timer-presentation: ${result.stderr}`);

for (const [path, source] of Object.entries({
  'apps/mobile/src/ui/signed-out-screen.tsx': "import { ActivityIndicator, Image, ScrollView, StyleSheet, View } from 'react-native'; export const SignedOutScreen = () => <Image source={{ uri: '/remote' }} />;",
  'apps/mobile/src/ui/native-button.ios.tsx': "import { Button, Menu } from '@expo/ui/swift-ui'; export const Primary = () => <><Menu label={<Button label='Sign in' />} /></>;",
})) {
  result = check({ [path]: source });
  assert.notEqual(result.status, 0, `unreviewed-native-signed-out-capability: ${path}`);
  assert.match(result.stderr, new RegExp(regexEscape(path)), path);
}

for (const source of [
  "import ReactNative from 'react-native'; export const OnboardingForm = ReactNative;",
  "import * as ReactNative from 'react-native'; export const OnboardingForm = ReactNative;",
  "import { Button, ScrollView, StyleSheet, Switch, View, useWindowDimensions } from 'react-native'; export const OnboardingForm = () => <Button title='Continue' />;",
  "import { StyleSheet, Switch, TextInput, View, useWindowDimensions } from 'react-native'; export const OnboardingForm = () => <TextInput />;",
  "import { ScrollView, StyleSheet, Switch, View } from 'react-native'; export const OnboardingForm = () => <ScrollView><Switch value={false} /></ScrollView>;",
  "import { ScrollView, StyleSheet, Switch, View, useWindowDimensions } from 'react-native'; export { Button } from 'react-native'; export const OnboardingForm = () => <View />;",
  "import ReactNative = require('react-native'); export const OnboardingForm = ReactNative.Switch;",
  "const ReactNative = require('react-native'); export const OnboardingForm = ReactNative.Switch;",
  "export const OnboardingForm = async () => import('react-native');",
]) {
  result = check({ 'apps/mobile/src/ui/onboarding-form.tsx': source });
  assert.notEqual(result.status, 0, `unreviewed-native-onboarding-capability: ${source}`);
}

result = check({
  'apps/mobile/src/ui/settings-icon.ios.tsx': "import { Image as SwiftUIImage } from '@expo/ui/swift-ui'; export const SettingsIcon = () => <><SwiftUIImage systemName='person.crop.circle' /></>;",
});
assert.equal(result.status, 0, `reviewed-native-settings-icon: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/manual-activity-form.tsx': "import { AccessibilityInfo, StyleSheet, View } from 'react-native'; export const ManualActivityForm = () => <View />;",
});
assert.equal(result.status, 0, `reviewed-manual-activity-presentation: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/manual-activity-form.tsx': "import { AccessibilityInfo, Alert, StyleSheet, View } from 'react-native'; export const ManualActivityForm = () => <View />;",
});
assert.notEqual(result.status, 0, 'unreviewed-manual-activity-capability');

result = check({
  'apps/mobile/src/ui/native-system-image.ios.tsx': "import { Image as SwiftUIImage } from '@expo/ui/swift-ui'; export const NativeSystemImage = () => <><SwiftUIImage systemName='exclamationmark.triangle' /></>;",
});
assert.equal(result.status, 0, `reviewed-native-system-image: ${result.stderr}`);

for (const source of [
  "import SwiftUI from '@expo/ui/swift-ui'; export const SettingsIcon = SwiftUI;",
  "import * as SwiftUI from '@expo/ui/swift-ui'; export const SettingsIcon = SwiftUI;",
  "import { Button, Image as SwiftUIImage } from '@expo/ui/swift-ui'; export const SettingsIcon = () => <><Button label='Account' /><SwiftUIImage systemName='person.crop.circle' /></>;",
  "import { Image as SwiftUIImage, Menu } from '@expo/ui/swift-ui'; export const SettingsIcon = () => <><Menu label={<SwiftUIImage systemName='person.crop.circle' />} /></>;",
  "import { accessibilityLabel } from '@expo/ui/swift-ui/modifiers'; export const SettingsIcon = accessibilityLabel('Account');",
  "import { Image as SwiftUIImage } from '@expo/ui/swift-ui'; export { Button } from '@expo/ui/swift-ui'; export const SettingsIcon = () => <><SwiftUIImage systemName='person.crop.circle' /></>;",
  "import { Image as SwiftUIImage } from '@expo/ui/swift-ui'; export * from '@expo/ui/swift-ui'; export const SettingsIcon = () => <><SwiftUIImage systemName='person.crop.circle' /></>;",
  "import SwiftUI = require('@expo/ui/swift-ui'); export const SettingsIcon = SwiftUI.Button;",
]) {
  result = check({ 'apps/mobile/src/ui/settings-icon.ios.tsx': source });
  assert.notEqual(result.status, 0, `unreviewed-native-settings-icon-capability: ${source}`);
}

result = check({
  'apps/mobile/app/_layout.tsx': "import { Stack } from 'expo-router'; export default function RootLayout() { return <Stack />; }",
});
assert.equal(result.status, 0, `reviewed-native-stack-layout: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/path-share-sheet.tsx': "import { AccessibilityInfo, StyleSheet, View } from 'react-native'; export const Sheet = () => <View />; AccessibilityInfo.isReduceMotionEnabled(); StyleSheet.create({});",
  'apps/mobile/src/ui/path-archive-confirmation-sheet.tsx': "import { AccessibilityInfo } from 'react-native'; AccessibilityInfo.isReduceMotionEnabled();",
});
assert.equal(result.status, 0, `reviewed-path-admin-motion: ${result.stderr}`);
result = check({ 'apps/mobile/src/ui/path-share-sheet.tsx': "import { AccessibilityInfo, Alert, StyleSheet, View } from 'react-native'; Alert.alert('x');" });
assert.notEqual(result.status, 0, 'path-admin-motion-does-not-admit-alert');

result = check({
  'apps/mobile/app/index.tsx': "import { router, useGlobalSearchParams, usePathname } from 'expo-router'; export default function IndexRedirect() { router.replace('/home'); return null; } export function HomeScreen() { useGlobalSearchParams(); usePathname(); router.push('/settings'); return null; }",
  'apps/mobile/app/(tabs)/_layout.tsx': "import { NativeTabs } from 'expo-router/unstable-native-tabs'; export default function Tabs() { return <NativeTabs />; }",
  'apps/mobile/app/(tabs)/home/_layout.tsx': "import { Stack } from 'expo-router'; export default function HomeLayout() { return <Stack />; }",
  'apps/mobile/src/ui/home-header-actions.ios.tsx': "import { Stack } from 'expo-router'; export default function HomeHeaderActions() { return <Stack.Toolbar />; }",
  'apps/mobile/src/ui/following-header-actions.ios.tsx': "import { Stack } from 'expo-router'; export default function FollowingHeaderActions() { return <Stack.Toolbar />; }",
  'apps/mobile/app/notifications.tsx': "import { router, useFocusEffect } from 'expo-router'; export default function Notifications() { useFocusEffect(() => undefined); router.replace('/(tabs)/home'); return null; }",
  'apps/mobile/app/invitations.tsx': "import { router } from 'expo-router'; export default function Invitations() { router.replace('/(tabs)/home'); return null; }",
  'apps/mobile/app/settings/index.tsx': "import { router } from 'expo-router'; export default function Settings() { router.push('/settings/account'); return null; }",
  'apps/mobile/app/settings/interactions.tsx': "import { router } from 'expo-router'; import { ActivityIndicator, View } from 'react-native'; export default function InteractionSettings() { router.back(); return <View><ActivityIndicator /></View>; }",
  'apps/mobile/app/settings/time-zone.tsx': "import { router } from 'expo-router'; import { ActivityIndicator, Alert, FlatList, InputAccessoryView, Keyboard, Pressable, StyleSheet, Text, TextInput, View } from 'react-native'; export default function TimeZone() { router.replace('/(tabs)/home'); Keyboard.dismiss(); return <View><InputAccessoryView nativeID='keyboard' /><ActivityIndicator /><FlatList data={[]} renderItem={() => <Pressable><Text /><TextInput /></Pressable>} /></View>; }",
  'apps/mobile/app/settings/blocked-accounts.tsx': "import { router, useFocusEffect } from 'expo-router'; import { AccessibilityInfo, Alert } from 'react-native'; export default function BlockedAccounts() { useFocusEffect(() => undefined); router.replace('/(tabs)/home'); Alert.alert('x'); AccessibilityInfo.announceForAccessibility('x'); return null; }",
  'apps/mobile/app/settings/account.tsx': "import { router } from 'expo-router'; export default function Account() { router.dismissAll(); return null; }",
  'apps/mobile/app/path/[pathID].tsx': "import { router, Stack, useLocalSearchParams } from 'expo-router'; export default function Path() { useLocalSearchParams(); router.replace('/(tabs)/home'); return <Stack.Screen />; }",
  'apps/mobile/app/path/[pathID]/history/index.tsx': "import { router, Stack, useLocalSearchParams } from 'expo-router'; export default function History() { const { pathID = '' } = useLocalSearchParams(); router.replace({ pathname: '/path/[pathID]', params: { pathID } }); return <Stack.Screen />; }",
  'apps/mobile/app/path/[pathID]/history/[activityID].tsx': "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; export default function Activity() { const { pathID = '' } = useLocalSearchParams(); useNavigation(); router.replace({ pathname: '/path/[pathID]/history', params: { pathID } }); return <Stack.Screen />; }",
  'apps/mobile/app/path/[pathID]/members/index.tsx': "import { router, Stack, useLocalSearchParams } from 'expo-router'; export default function People() { useLocalSearchParams(); router.replace('/(tabs)/home'); return <Stack.Screen />; }",
  'apps/mobile/app/path/[pathID]/members/[userID].tsx': "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; export default function Person() { useLocalSearchParams(); useNavigation(); router.replace('/(tabs)/home'); return <Stack.Screen />; }",
  'apps/mobile/app/path/[pathID]/nudge-settings.tsx': "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; export default function NudgeSettings() { useLocalSearchParams(); useNavigation(); router.replace('/(tabs)/home'); return <Stack.Screen />; }",
  'apps/mobile/src/use-path-route-ancestry.ts': "import { useFocusEffect, useNavigation } from 'expo-router'; export function usePathRouteAncestry() { const navigation = useNavigation(); useFocusEffect(() => { navigation.reset({ index: 0, routes: [{ name: '(tabs)' }] }); }); }",
  'apps/mobile/src/use-social-interaction-route-ancestry.ts': "import { useFocusEffect, useNavigation } from 'expo-router'; export function useSocialInteractionRouteAncestry() { const navigation = useNavigation(); useFocusEffect(() => { navigation.reset({ index: 0, routes: [{ name: 'index' }] }); }); }",
  'apps/mobile/src/use-notification-journey-route-ancestry.ts': "import { useFocusEffect, useNavigation } from 'expo-router'; export function useNotificationJourneyRouteAncestry() { const navigation = useNavigation(); useFocusEffect(() => { navigation.reset({ index: 0, routes: [{ name: '(tabs)' }] }); }); }",
  'apps/mobile/src/use-settings-journey-route-ancestry.ts': "import { useFocusEffect, useNavigation } from 'expo-router'; export function useSettingsJourneyRouteAncestry() { const navigation = useNavigation(); useFocusEffect(() => { navigation.reset({ index: 0, routes: [{ name: '(tabs)' }] }); }); }",
  'apps/mobile/src/ui/native-header-button.ios.tsx': "import { Button } from '@expo/ui/swift-ui'; import { accessibilityLabel, buttonStyle, disabled, labelStyle } from '@expo/ui/swift-ui/modifiers'; export function Header() { return <><Button label='Read' modifiers={[accessibilityLabel('Read'), buttonStyle('plain'), disabled(false), labelStyle('iconOnly')]} /></>; }",
  'apps/mobile/src/ui/path-header-menu.ios.tsx': "import { Stack } from 'expo-router'; export default function Menu() { return <Stack.Toolbar />; }",
  'apps/mobile/src/ui/path-header-menu.tsx': "import { Stack } from 'expo-router'; export default function Menu() { return <Stack.Toolbar />; }",
});
assert.equal(result.status, 0, `reviewed-native-settings-routes: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/nudge-composer-sheet.tsx': "import { AccessibilityInfo, Pressable, StyleSheet, View } from 'react-native'; export function Composer() { AccessibilityInfo.isReduceMotionEnabled(); return <Pressable><View /></Pressable>; } void StyleSheet;",
});
assert.equal(result.status, 0, `reviewed-nudge-composer-motion: ${result.stderr}`);

result = check({
  'apps/mobile/src/ui/nudge-composer-sheet.tsx': "import { AccessibilityInfo, Alert, Pressable, StyleSheet, View } from 'react-native'; Alert.alert('x'); void AccessibilityInfo; void Pressable; void StyleSheet; void View;",
});
assert.notEqual(result.status, 0, 'nudge-composer-motion-does-not-admit-alert');

result = check({
  'apps/mobile/app/path/[pathID]/nudge-settings.tsx': "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; export default function NudgeSettings() { useLocalSearchParams(); useNavigation(); router.push('/settings'); return <Stack.Screen />; }",
});
assert.notEqual(result.status, 0, 'path-nudge-router-is-limited-to-home-fallback');

result = check({
  'apps/mobile/app/path/[pathID]/members/index.tsx': "import { router, Stack, useLocalSearchParams } from 'expo-router'; export default function People() { useLocalSearchParams(); router.push('/settings'); return <Stack.Screen />; }",
});
assert.notEqual(result.status, 0, 'path-people-router-is-limited-to-home-fallback');

result = check({
  'apps/mobile/app/path/[pathID]/members/[userID].tsx': "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; export default function Person() { useLocalSearchParams(); useNavigation(); router.replace('/settings'); return <Stack.Screen />; }",
});
assert.notEqual(result.status, 0, 'path-member-router-is-limited-to-home-fallback');

result = check({
  'apps/mobile/app/path/[pathID]/members/index.tsx': "import { router, Stack, useLocalSearchParams } from 'expo-router'; export default function People() { useLocalSearchParams(); router['push']('/settings'); return <Stack.Screen />; }",
});
assert.notEqual(result.status, 0, 'path-people-router-rejects-element-access');

result = check({
  'apps/mobile/app/path/[pathID]/members/[userID].tsx': "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; export default function Person() { useLocalSearchParams(); useNavigation(); const go = router.push; go('/settings'); return <Stack.Screen />; }",
});
assert.notEqual(result.status, 0, 'path-member-router-rejects-aliased-calls');

result = check({
  'apps/mobile/app/_layout.tsx': "import { Stack, useRouter } from 'expo-router'; export default function RootLayout() { useRouter(); return <Stack />; }",
  'apps/mobile/app/(tabs)/_layout.tsx': "import { NativeTabs, useNativeTabs } from 'expo-router/unstable-native-tabs'; export default function Tabs() { useNativeTabs(); return <NativeTabs />; }",
});
assert.notEqual(result.status, 0, 'unreviewed-router-capabilities');

result = check({
  'apps/mobile/app/settings/index.tsx': "import { Stack } from 'expo-router'; export default function Settings() { return <Stack />; }",
  'apps/mobile/app/path/[pathID].tsx': "import { router, Stack, useLocalSearchParams } from 'expo-router'; export default function Path({ destination }) { useLocalSearchParams(); router.push(destination); return <Stack.Screen />; }",
});
assert.notEqual(result.status, 0, 'unreviewed-settings-router-capabilities');

result = check({
  'apps/web/src/routes/component-api/+page.svelte': '<Nav target="/v1/me"/>',
  'apps/web/src/routes/component-api/Nav.svelte': '<script>export let target;</script><button onclick={() => window.location.assign(target)}>Go</button>',
});
assert.notEqual(result.status, 0, `component-api-navigation: ${result.stderr}`);
assert.match(result.stderr, /apps\/web\/src\/routes\/component-api\/\+page\.svelte/);

result = check({
  'apps/web/src/routes/component-page/+page.svelte': '<Nav target="/paths"/>',
  'apps/web/src/routes/component-page/Nav.svelte': '<a href="/paths">Go</a>',
});
assert.equal(result.status, 0, `legitimate-component-navigation: ${result.stderr}`);

const reviewedProfileDiscoveryProps = `
  <script>
    const i18n = {}; const socialQuery = ''; const socialSearchState = 'hint';
    const socialSearch = { items: [], nextCursor: '' }; const socialLoadingMore = false;
    const selectedSocialProfile = null; const socialProfileState = 'idle';
    const socialRelationshipBusy = false; const socialRelationshipError = false;
    const socialFollowRequestsOpen = false; const socialFollowRequestsState = 'idle';
    const socialFollowRequests = { items: [], nextCursor: '' }; const socialFollowRequestBusyID = '';
    const updateSocialQuery = () => {}; const searchSocialProfiles = () => {};
    const openSocialProfile = () => {}; const loadMoreSocialProfiles = () => {};
    const closeSocialProfile = () => {};
  </script>
  <SocialProfileDiscovery
    {i18n}
    query={socialQuery}
    searchState={socialSearchState}
    results={socialSearch.items}
    nextCursor={socialSearch.nextCursor}
    loadingMore={socialLoadingMore}
    selectedProfile={selectedSocialProfile}
    profileState={socialProfileState}
    relationshipBusy={socialRelationshipBusy}
    relationshipError={socialRelationshipError}
    followRequestsOpen={socialFollowRequestsOpen}
    followRequestsState={socialFollowRequestsState}
    followRequests={socialFollowRequests.items}
    followRequestsNextCursor={socialFollowRequests.nextCursor}
    busyFollowRequestID={socialFollowRequestBusyID}
    onQueryChange={updateSocialQuery}
    onSearch={() => void searchSocialProfiles()}
    onSelectProfile={(userID) => void openSocialProfile(userID)}
    onRetrySearch={() => void searchSocialProfiles()}
    onLoadMore={() => void loadMoreSocialProfiles()}
    onCloseProfile={closeSocialProfile}
    onRefreshProfile={() => void openSocialProfile('')}
    onRelationshipAction={() => {}}
    onOpenFollowRequests={() => {}}
    onCloseFollowRequests={() => {}}
    onLoadMoreFollowRequests={() => {}}
    onReviewFollowRequest={() => {}}
  />`;
result = check({ 'apps/web/src/routes/+page.svelte': reviewedProfileDiscoveryProps });
assert.equal(result.status, 0, `reviewed-profile-discovery-props: ${result.stderr}`);

result = check({
  'apps/web/src/routes/+page.svelte': reviewedProfileDiscoveryProps.replace('/>', 'destination={apiTarget} />'),
});
assert.notEqual(result.status, 0, `unreviewed-profile-discovery-prop: ${result.stderr}`);
assert.match(result.stderr, /apps\/web\/src\/routes\/\+page\.svelte/);

result = check({
  'apps/web/src/routes/+page.svelte': reviewedProfileDiscoveryProps.replace(
    'onSearch={() => void searchSocialProfiles()}',
    "onSearch={() => fetch('/v1/profiles')}",
  ),
});
assert.notEqual(result.status, 0, `unsafe-reviewed-profile-discovery-prop: ${result.stderr}`);
assert.match(result.stderr, /apps\/web\/src\/routes\/\+page\.svelte/);

result = check({
  'outside/transport.ts': "export const send = (url) => fetch(url);",
  'apps/web/src/lib/symlink-import.ts': "import { send } from './linked-transport.ts'; send('/v1/me');",
}, {
  'apps/web/src/lib/linked-transport.ts': '../../../../outside/transport.ts',
});
assert.notEqual(result.status, 0, `symlink-import: ${result.stderr}`);
assert.match(result.stderr, /apps\/web\/src\/lib\/linked-transport\.ts/);

result = check({
  'outside/hidden.ts': "fetch('/v1/me');",
}, {
  'apps/mobile/src/linked': '../../../outside',
});
assert.notEqual(result.status, 0, `symlink-directory: ${result.stderr}`);
assert.match(result.stderr, /apps\/mobile\/src\/linked/);

result = check({
  'apps/web/src/lib/js-specifier.ts': "import { value } from './js-source.js'; export const jsValue = value;",
  'apps/web/src/lib/js-source.ts': "export const value = 'typed';",
  'apps/web/src/lib/js-tsx-specifier.ts': "import { value } from './js-tsx-source.js'; export const jsTsxValue = value;",
  'apps/web/src/lib/js-tsx-source.tsx': "export const value = <></>;",
  'apps/web/src/lib/jsx-specifier.ts': "import { value } from './jsx-source.jsx'; export const jsxValue = value;",
  'apps/web/src/lib/jsx-source.tsx': "export const value = <></>;",
  'apps/web/src/lib/jsx-ts-specifier.ts': "import { value } from './jsx-ts-source.jsx'; export const jsxTsValue = value;",
  'apps/web/src/lib/jsx-ts-source.ts': "export const value = 'typed-jsx';",
  'apps/web/src/lib/jsx-js-specifier.ts': "import { value } from './jsx-js-source.jsx'; export const jsxJsValue = value;",
  'apps/web/src/lib/jsx-js-source.js': "export const value = 'runtime-js';",
  'apps/mobile/src/mjs-specifier.ts': "import { value } from './mjs-source.mjs'; export const mjsValue = value;",
  'apps/mobile/src/mjs-source.mts': "export const value = 'module';",
  'apps/mobile/src/cjs-specifier.ts': "import { value } from './cjs-source.cjs'; export const cjsValue = value;",
  'apps/mobile/src/cjs-source.cts': "export const value = 'common';",
});
assert.equal(result.status, 0, `runtime-suffix-source-substitution: ${result.stderr}`);

for (const [scenario, importer, specifier, sourcePath, source] of [
  ['js-to-ts-transport', 'apps/web/src/lib/js-transport-import.ts', './js-transport.js', 'apps/web/src/lib/js-transport.ts', "export const send = (url) => fetch(url);"],
  ['jsx-to-tsx-transport', 'apps/web/src/lib/jsx-transport-import.ts', './jsx-transport.jsx', 'apps/web/src/lib/jsx-transport.tsx', "export const send = (url) => fetch(url);"],
  ['mjs-to-mts-transport', 'apps/mobile/src/mjs-transport-import.ts', './mjs-transport.mjs', 'apps/mobile/src/mjs-transport.mts', "export const send = (url) => fetch(url);"],
  ['cjs-to-cts-transport', 'apps/mobile/src/cjs-transport-import.ts', './cjs-transport.cjs', 'apps/mobile/src/cjs-transport.cts', "export const send = (url) => fetch(url);"],
]) {
  result = check({ [importer]: `import { send } from '${specifier}'; send('/v1/me');`, [sourcePath]: source });
  assert.notEqual(result.status, 0, `${scenario}: ${result.stderr}`);
  assert.match(result.stderr, new RegExp(sourcePath.replaceAll('/', '\\/')), scenario);
}

result = check({
  'apps/web/src/lib/unsupported-import.ts': "import data from './unsupported.json'; export { data };",
  'apps/web/src/lib/unsupported.json': '{}',
});
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /apps\/web\/src\/lib\/unsupported-import\.ts/);

result = check({
  'apps/mobile/src/provider-bypass.ts': "import * as AuthSession from 'expo-auth-session'; AuthSession.exchangeCodeAsync(options, discovery);",
  'apps/web/src/lib/provider-bypass.ts': "import { UserManager } from 'oidc-client-ts'; new UserManager(settings);",
});
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /provider-bypass\.ts/);

for (const [scenario, path, source] of [
  ['web-named', 'apps/web/src/lib/auth.ts', "export { UserManager as Provider } from 'oidc-client-ts';"],
  ['mobile-star', 'apps/mobile/app/index.tsx', "export * from 'expo-auth-session';"],
  ['web-namespace', 'apps/web/src/lib/auth.ts', "export * as Provider from 'oidc-client-ts';"],
  ['auth-import-export', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export { UserManager as Provider };"],
  ['provider-alias', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; const Provider = AuthSession; export { Provider };"],
  ['provider-variable', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; export const Provider = AuthSession;"],
  ['provider-factory', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export function createProvider() { return new UserManager(settings); }"],
  ['provider-factory-alias', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; function createProvider() { return new UserManager(settings); } export { createProvider };"],
  ['provider-object', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; export const Provider = { AuthSession };"],
  ['assignment-destructuring', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; let Provider; ({ AuthSession: Provider } = { AuthSession }); export { Provider };"],
  ['property-mutation', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; const exported = {}; exported.provider = AuthSession; export { exported };"],
  ['object-assign', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; const exported = {}; Object.assign(exported, { UserManager }); export { exported };"],
  ['returned-closure', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export const leak = () => () => UserManager;"],
  ['callback-argument', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export function leak(callback) { callback(UserManager); }"],
  ['exported-class-fields', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export class Leak { provider = UserManager; }"],
  ['default-class', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; export default class Home { provider = AuthSession; }"],
  ['identity-call', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; const identity = (value) => value; export const Leak = identity(UserManager);"],
  ['promise-call', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export const Leak = Promise.resolve(UserManager);"],
  ['logical-composite', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export const Leak = fallback || UserManager;"],
  ['comma-composite', 'apps/web/src/lib/auth.ts', "import { UserManager } from 'oidc-client-ts'; export const Leak = (fallback, UserManager);"],
  ['object-method', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; export const Leak = { provider() { return AuthSession; } };"],
  ['default-closure', 'apps/mobile/app/index.tsx', "import * as AuthSession from 'expo-auth-session'; export default () => AuthSession;"],
]) {
  const adapter = path.startsWith('apps/mobile/') ? 'apps/mobile/src/provider-auth.ts' : 'apps/web/src/lib/provider-auth.ts';
  result = check({ [adapter]: `${protectedBaseline[adapter]}\n// ${scenario}\n${source}` });
  assert.notEqual(result.status, 0, `${scenario}: ${result.stderr}`);
  assert.match(result.stderr, new RegExp(adapter.replaceAll('/', '\\/')));
}

for (const [scenario, source] of [
  ['async-home', 'export default async function Home() { return <></>; }'],
  ['generator-home', 'export default function* Home() { yield null; return <></>; }'],
]) {
  result = check({ 'apps/mobile/app/index.tsx': source });
  assert.notEqual(result.status, 0, `${scenario}: ${result.stderr}`);
  assert.match(result.stderr, /apps\/mobile\/app\/index\.tsx/);
}

result = check({ 'scripts/protected-provider-adapters.sha256': 'not a reviewed manifest\n' });
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /protected-provider-adapters\.sha256/);

result = check({ 'scripts/protected-client-capability-adapters.sha256': 'not a reviewed manifest\n' });
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /protected-client-capability-adapters\.sha256/);

result = check({
  'apps/web/src/lib/external-policy-link.ts': `${protectedBaseline['apps/web/src/lib/external-policy-link.ts']}\n// unreviewed navigation drift\n`,
});
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /apps\/web\/src\/lib\/external-policy-link\.ts/);

result = check({
  'apps/web/src/lib/notification-convergence-browser.ts': `${protectedBaseline['apps/web/src/lib/notification-convergence-browser.ts']}\n// unreviewed convergence drift\n`,
});
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /apps\/web\/src\/lib\/notification-convergence-browser\.ts/);

result = check({
  'apps/mobile/src/provider-auth-state.ts': `${protectedBaseline['apps/mobile/src/provider-auth-state.ts']}\n// unreviewed classifier drift\n`,
});
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /apps\/mobile\/src\/provider-auth-state\.ts/);

result = check({
  'apps/mobile/src/provider-discovery.ts': `${protectedBaseline['apps/mobile/src/provider-discovery.ts']}\n// unreviewed discovery drift\n`,
});
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /apps\/mobile\/src\/provider-discovery\.ts/);

result = check({ 'apps/web/src/lib/malformed.ts': "fetch('/v1/me'" });
assert.notEqual(result.status, 0, result.stderr);
assert.match(result.stderr, /malformed\.ts/);

result = check({
  'apps/web/src/lib/local-primitives.ts': "function fetch(value) { return value; } class WebSocket {} const Request = (value) => value; fetch('local'); new WebSocket(); Request('local'); export function local(sendBeacon) { sendBeacon('local'); }",
  'apps/web/src/lib/block-local.ts': "const localHandler = (value) => value; { const fetch = localHandler; fetch('local'); }",
  'apps/web/src/lib/local-property-names.ts': "const adapter = { fetch() { return 'local'; }, get EventSource() { return 'local'; } }; class Local { Request() { return 'local'; } WebSocket = 'local'; } adapter.fetch(); new Local().Request();",
  'apps/web/src/lib/runtime-type-bindings.ts': "enum Request { Local } namespace WebSocket { export const local = 'local'; } const request = Request.Local; const socket = WebSocket.local; export { request, socket };",
  'apps/web/src/lib/type-only-primitives.ts': "interface Request { local: string; } type EventSource = Request; const value: Request = { local: 'yes' }; export type { EventSource }; export { type Request, value };",
  'apps/web/src/lib/local-array.ts': "export const values = Array.isArray([]);",
  'apps/web/src/lib/local-map.ts': "export const values = new Map([['safe', 1]]);",
});
assert.equal(result.status, 0, result.stderr);

console.log('client API boundary rejects alternate transports and allows only exact provider adapters');
