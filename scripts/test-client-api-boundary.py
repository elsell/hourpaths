#!/usr/bin/env python3
from __future__ import annotations

import pathlib
import subprocess
import tempfile
import unittest


REPOSITORY = pathlib.Path(__file__).resolve().parents[1]
CHECKER = REPOSITORY / "scripts/check-client-api-boundary.py"
FIXTURE_PATHS = (
    "apps/mobile/src/provider-auth.ts",
    "apps/mobile/src/provider-auth-state.ts",
    "apps/mobile/src/provider-discovery.ts",
    "apps/mobile/src/push-notifications-native.ts",
    "apps/mobile/src/push-notifications.ts",
    "apps/web/src/lib/provider-auth.ts",
    "apps/mobile/src/policy-link-native.ts",
    "apps/mobile/src/policy-link.ts",
    "apps/web/src/lib/accessibility-focus.ts",
    "apps/web/src/lib/device-locale.ts",
    "apps/web/src/lib/external-policy-link.ts",
    "apps/web/src/lib/notification-convergence-browser.ts",
    "scripts/protected-client-capability-adapters.sha256",
    "scripts/protected-provider-adapters.sha256",
    "scripts/protected-api-client-adapter.sha256",
    "packages/api-client/package.json",
    "packages/api-client/src/index.ts",
    "packages/api-client/src/schema.d.ts",
    "packages/api-client/src/session.ts",
    "packages/client-core/package.json",
    "packages/i18n/package.json",
    "apps/web/src/app.html",
)


class ClientApiBoundaryTest(unittest.TestCase):
    def run_checker(self, files: dict[str, str]) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            fixtures = {
                relative: (REPOSITORY / relative).read_text(encoding="utf-8")
                for relative in FIXTURE_PATHS
            }
            fixtures.update(files)
            for relative, contents in fixtures.items():
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(contents, encoding="utf-8")
            return subprocess.run(
                ["python3", str(CHECKER), str(root)],
                check=False,
                capture_output=True,
                text=True,
            )

    def test_rejects_application_api_construction_in_web_and_mobile(self) -> None:
        result = self.run_checker({
            "apps/web/src/lib/auth.ts": "fetch(`${apiURL}/v1/sessions`);",
            "apps/mobile/app/index.tsx": "new Request(`${apiURL}/v1/me`);",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("apps/web/src/lib/auth.ts", result.stderr)
        self.assertIn("apps/mobile/app/index.tsx", result.stderr)

    def test_rejects_concatenated_templated_and_aliased_network_bypasses(self) -> None:
        result = self.run_checker({
            "apps/web/src/lib/concatenated.ts": "fetch(apiURL + '/v' + 1 + '/me');",
            "apps/web/src/lib/templated.ts": "fetch(`${apiURL}/v${version}/me`);",
            "apps/mobile/src/aliased.ts": "const send = globalThis.fetch; send(apiURL);",
            "apps/mobile/src/imported.ts": "import transport from 'openapi-fetch'; transport();",
        })
        self.assertNotEqual(result.returncode, 0)
        for path in (
            "apps/web/src/lib/concatenated.ts",
            "apps/web/src/lib/templated.ts",
            "apps/mobile/src/aliased.ts",
            "apps/mobile/src/imported.ts",
        ):
            self.assertIn(path, result.stderr)

    def test_rejects_computed_and_alternate_network_bypasses(self) -> None:
        result = self.run_checker({
            "apps/web/src/lib/computed.ts": (
                "(globalThis as any)['fe' + 'tch'](apiURL + '/' + 'v' + 1 + '/me');"
            ),
            "apps/web/src/lib/beacon.ts": (
                "navigator.sendBeacon(apiURL + '/' + 'v' + 1 + '/session');"
            ),
            "apps/mobile/src/dynamic.ts": (
                "import('ax' + 'ios').then((module) => "
                "module.default(apiURL + '/' + 'v' + 1 + '/me'));"
            ),
        })
        self.assertNotEqual(result.returncode, 0)
        for path in (
            "apps/web/src/lib/computed.ts",
            "apps/web/src/lib/beacon.ts",
            "apps/mobile/src/dynamic.ts",
        ):
            self.assertIn(path, result.stderr)

    def test_rejects_application_api_navigation_bypasses(self) -> None:
        result = self.run_checker({
            "apps/web/src/lib/location.ts": "window.location.href = '/v1/me';",
            "apps/web/src/lib/goto.ts": (
                "import { goto } from '@sveltejs/kit'; void goto('/v1/me');"
            ),
            "apps/web/src/routes/form/+page.svelte": (
                '<form action="/v1/sessions" method="post"></form>'
            ),
            "apps/web/src/lib/location-alias.ts": (
                "const location = window.location; location.assign('/v1/me');"
            ),
            "apps/web/src/lib/location-value.ts": (
                "const apiPath = '/v1/me'; window.location.assign(apiPath);"
            ),
            "apps/web/src/lib/goto-alias.ts": (
                "import { goto } from '@sveltejs/kit'; const navigate = goto; "
                "void navigate('/v1/me');"
            ),
            "apps/web/src/routes/bound/+page.svelte": (
                "<script>const target = '/v1/sessions';</script>"
                "<form action={target}></form>"
            ),
            "apps/web/src/routes/link/+page.svelte": '<a href="/v1/me">Account</a>',
            "apps/web/src/lib/returned-location.ts": (
                "function current() { return window.location; } "
                "current().assign('/v1/me');"
            ),
            "apps/web/src/lib/returned-target.ts": (
                "function target() { return '/v1/me'; } "
                "window.location.assign(target());"
            ),
            "apps/web/src/lib/dispatcher.ts": (
                "import { goto } from '@sveltejs/kit'; "
                "const dispatch = (navigate, target) => navigate(target); "
                "const alias = dispatch; alias(goto, '/v1/me');"
            ),
            "apps/web/src/lib/destructure.ts": (
                "let navigate; ({ assign: navigate } = window.location); "
                "navigate('/v1/me');"
            ),
            "apps/web/src/routes/spread/+page.svelte": (
                "<script>const attrs = { href: '/v1/me' };</script>"
                "<a {...attrs}>Account</a>"
            ),
            "apps/web/src/routes/image/+page.svelte": '<img src="/v1/me" alt="">',
            "apps/web/src/lib/link.tsx": "export const Link = () => <a href='/v1/me'>Account</a>;",
            "apps/web/src/lib/dynamic-template.ts": (
                "export function navigate(id) { "
                "window.location.assign(`/v1/${id}`); }"
            ),
            "apps/web/src/lib/dynamic-link.tsx": (
                "export const Link = ({ id }) => <a href={`/v1/${id}`}>Account</a>;"
            ),
            "apps/web/src/lib/composed-template.ts": (
                "const slash = '/'; const version = 'v1'; "
                "export function navigate(id) { "
                "window.location.assign(`${slash}${version}/${id}`); }"
            ),
            "apps/web/src/lib/join.ts": (
                "export function navigate(id) { "
                "const target = ['', 'v1', id].join('/'); "
                "window.location.assign(target); }"
            ),
            "apps/web/src/lib/constant-join.ts": (
                "const target = ['', 'v1', 'me'].join('/'); "
                "window.location.assign(target);"
            ),
            "apps/web/src/lib/constant-concat.ts": (
                "const target = '/'.concat('v1', '/me'); "
                "window.location.assign(target);"
            ),
            "apps/web/src/lib/string-code.ts": (
                "const target = String.fromCharCode(47, 118, 49, 47, 109, 101); "
                "window.location.assign(target);"
            ),
            "apps/web/src/lib/unknown-navigation.ts": (
                "export function navigate(target) { window.location.assign(target); }"
            ),
            "apps/web/src/lib/unknown-goto.ts": (
                "import { goto } from '@sveltejs/kit'; "
                "export function navigate(target) { void goto(target); }"
            ),
            "apps/web/src/routes/unknown-href/+page.svelte": (
                "<script>export let target;</script><a href={target}>Go</a>"
            ),
            "apps/web/src/lib/backslash.ts": (
                "window.location.assign('\\\\v1\\\\me');"
            ),
            "apps/web/src/lib/javascript.ts": (
                "window.location.assign('javascript:alert(1)');"
            ),
            "apps/web/src/routes/data/+page.svelte": (
                '<a href="data:text/html,unsafe">Unsafe</a>'
            ),
            "apps/web/src/routes/onclick/+page.svelte": (
                "<button onclick={() => fetch('/v1/me')}>Load</button>"
            ),
            "apps/web/src/routes/onclick-location/+page.svelte": (
                "<button onclick={() => "
                "window.location.assign('/v1/me')}>Load</button>"
            ),
            "apps/web/src/routes/legacy/+page.svelte": (
                "<button on:click={() => fetch('/v1/me')}>Load</button>"
            ),
            "apps/web/src/routes/quoted-angle/+page.svelte": (
                '<form title=">" action="/v1/sessions"></form>'
            ),
            "apps/web/src/routes/quoted-script/+page.svelte": (
                '<script data-x=">//">fetch("/v1/me")</script><p>x</p>'
            ),
            "apps/web/src/routes/quoted-module/+page.svelte": (
                '<script module data-x=">//">fetch("/v1/me")</script><p>x</p>'
            ),
            "apps/web/src/routes/quoted-script-exact/+page.svelte": (
                '<script data-x=">">fetch("/v1/me")</script><p>x</p>'
            ),
            "apps/web/src/routes/quoted-context-module/+page.svelte": (
                '<script context="module" data-x=">">fetch("/v1/me")</script><p>x</p>'
            ),
            "apps/web/src/routes/style-block/+page.svelte": (
                "<style>.probe{background:url('/v1/me')}</style>"
            ),
            "apps/web/src/routes/style-attribute/+page.svelte": (
                "<div style=\"background-image:url('/v1/me')\"></div>"
            ),
            "apps/web/src/routes/meta-refresh/+page.svelte": (
                '<svelte:head><meta http-equiv="refresh" '
                'content="0;url=/v1/me"></svelte:head>'
            ),
            "apps/web/src/routes/image-preload/+page.svelte": (
                '<svelte:head><link rel="preload" as="image" '
                'imagesrcset="/v1/me 1x"></svelte:head>'
            ),
        })
        self.assertNotEqual(result.returncode, 0)
        for path in (
            "apps/web/src/lib/location.ts",
            "apps/web/src/lib/goto.ts",
            "apps/web/src/routes/form/+page.svelte",
            "apps/web/src/lib/location-alias.ts",
            "apps/web/src/lib/location-value.ts",
            "apps/web/src/lib/goto-alias.ts",
            "apps/web/src/routes/bound/+page.svelte",
            "apps/web/src/routes/link/+page.svelte",
            "apps/web/src/lib/returned-location.ts",
            "apps/web/src/lib/returned-target.ts",
            "apps/web/src/lib/dispatcher.ts",
            "apps/web/src/lib/destructure.ts",
            "apps/web/src/routes/spread/+page.svelte",
            "apps/web/src/routes/image/+page.svelte",
            "apps/web/src/lib/link.tsx",
            "apps/web/src/lib/dynamic-template.ts",
            "apps/web/src/lib/dynamic-link.tsx",
            "apps/web/src/lib/composed-template.ts",
            "apps/web/src/lib/join.ts",
            "apps/web/src/lib/constant-join.ts",
            "apps/web/src/lib/constant-concat.ts",
            "apps/web/src/lib/string-code.ts",
            "apps/web/src/lib/unknown-navigation.ts",
            "apps/web/src/lib/unknown-goto.ts",
            "apps/web/src/routes/unknown-href/+page.svelte",
            "apps/web/src/lib/backslash.ts",
            "apps/web/src/lib/javascript.ts",
            "apps/web/src/routes/data/+page.svelte",
            "apps/web/src/routes/onclick/+page.svelte",
            "apps/web/src/routes/onclick-location/+page.svelte",
            "apps/web/src/routes/legacy/+page.svelte",
            "apps/web/src/routes/quoted-angle/+page.svelte",
            "apps/web/src/routes/quoted-script/+page.svelte",
            "apps/web/src/routes/quoted-module/+page.svelte",
            "apps/web/src/routes/quoted-script-exact/+page.svelte",
            "apps/web/src/routes/quoted-context-module/+page.svelte",
            "apps/web/src/routes/style-block/+page.svelte",
            "apps/web/src/routes/style-attribute/+page.svelte",
            "apps/web/src/routes/meta-refresh/+page.svelte",
            "apps/web/src/routes/image-preload/+page.svelte",
        ):
            self.assertIn(path, result.stderr)

    def test_rejects_executable_app_shell_and_static_assets(self) -> None:
        result = self.run_checker({
            "apps/web/src/app.html": (
                '<!doctype html><html><head></head><body>'
                '<script>fetch("/v1/me")</script>%sveltekit.body%</body></html>'
            ),
            "apps/web/static/raw.js": "fetch('/v1/me');",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("apps/web/src/app.html", result.stderr)
        self.assertIn("apps/web/static/raw.js", result.stderr)

    def test_scans_generated_named_directories_beneath_approved_roots(self) -> None:
        result = self.run_checker({
            "apps/web/src/lib/imported.ts": "import './vendor/node_modules/escape';",
            "apps/web/src/lib/vendor/node_modules/escape.ts": "fetch('/v1/me');",
            "apps/web/static/build/escape.js": "fetch('/v1/me');",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("apps/web/src/lib/vendor/node_modules/escape.ts", result.stderr)
        self.assertIn("apps/web/static/build/escape.js", result.stderr)

    def test_rejects_active_static_markup_and_raw_svelte_html(self) -> None:
        result = self.run_checker({
            "apps/web/static/event.svg": (
                '<svg xmlns="http://www.w3.org/2000/svg" '
                'onload="fetch(\'/v1/me\')"></svg>'
            ),
            "apps/web/static/raw.xhtml": (
                '<html xmlns="http://www.w3.org/1999/xhtml">unsafe</html>'
            ),
            "apps/web/static/raw.xml": "<?xml version=\"1.0\"?><root/>",
            "apps/web/src/routes/raw/+page.svelte": "{@html payload}",
        })
        self.assertNotEqual(result.returncode, 0)
        for path in (
            "apps/web/static/event.svg",
            "apps/web/static/raw.xhtml",
            "apps/web/static/raw.xml",
            "apps/web/src/routes/raw/+page.svelte",
        ):
            self.assertIn(path, result.stderr)

    def test_rejects_api_component_props_and_workspace_transport_escapes(self) -> None:
        result = self.run_checker({
            "apps/web/src/routes/component/+page.svelte": '<Nav target="/v1/me"/>',
            "apps/web/src/routes/component/Nav.svelte": (
                "<script>export let target;</script>"
                "<button onclick={() => window.location.assign(target)}>Go</button>"
            ),
            "packages/client-core/src/transport.ts": (
                "export const send = (url) => fetch(url);"
            ),
            "packages/i18n/src/transport.ts": (
                "export const send = (url) => fetch(url);"
            ),
        })
        self.assertNotEqual(result.returncode, 0)
        for path in (
            "apps/web/src/routes/component/+page.svelte",
            "packages/client-core/src/transport.ts",
            "packages/i18n/src/transport.ts",
        ):
            self.assertIn(path, result.stderr)

    def test_allows_non_api_navigation(self) -> None:
        result = self.run_checker({
            "apps/web/src/routes/form/+page.svelte": (
                '<form action="/search" method="get"></form>'
            ),
            "apps/web/src/routes/preload/+page.svelte": (
                '<svelte:head><link rel="preload" as="image" '
                'imagesrcset="/images/path.webp 1x"></svelte:head>'
            ),
            "apps/web/src/routes/component/+page.svelte": '<Nav target="/paths"/>',
            "apps/web/src/lib/https.ts": (
                "export const identity = 'https://identity.example/authorize';"
            ),
            "apps/web/src/routes/callback/+page.svelte": (
                '<script>window.location.replace("/");</script>'
            ),
        })
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_allows_reviewed_mobile_ui_presentation_primitives(self) -> None:
        result = self.run_checker({
            "apps/mobile/app/index.tsx": (
                "import { SafeAreaView } from 'react-native-safe-area-context'; "
                "export default function IndexRedirect() { return null; } "
                "export function HomeScreen() { return <SafeAreaView />; }"
            ),
            "apps/mobile/src/ui/native-route-presentation.tsx": (
                "import { SafeAreaView } from 'react-native-safe-area-context'; "
                "export const Route = () => <SafeAreaView />;"
            ),
            "apps/mobile/src/ui/primitives.tsx": (
                "import { SafeAreaView } from 'react-native-safe-area-context'; "
                "export const Sheet = () => <SafeAreaView />;"
            ),
            "apps/mobile/src/ui/progress-indicator.tsx": (
                "import { Pressable, StyleSheet, Text, View } from 'react-native'; "
                "const styles = StyleSheet.create({ root: { gap: 8 } }); "
                "export const Progress = () => <Pressable accessibilityRole='button'>"
                "<View style={styles.root}><Text>value</Text></View></Pressable>;"
            ),
        })
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_allows_exact_native_time_zone_settings_primitives(self) -> None:
        result = self.run_checker({
            "apps/mobile/app/settings/time-zone.tsx": (
                "import { ActivityIndicator, Alert, FlatList, InputAccessoryView, Keyboard, Pressable, StyleSheet, "
                "Text, TextInput, View } from 'react-native'; "
                "import { router } from 'expo-router'; "
                "export const Route = () => <View><InputAccessoryView nativeID='keyboard'/><TextInput/>"
                "<FlatList data={[]} "
                "renderItem={() => <Pressable><Text>Zone</Text></Pressable>}/></View>;"
            ),
        })
        self.assertEqual(result.returncode, 0, result.stderr)

        widened = self.run_checker({
            "apps/mobile/app/settings/time-zone.tsx": (
                "import { ActivityIndicator, Alert, FlatList, Linking, Pressable, StyleSheet, "
                "Text, TextInput, View } from 'react-native'; "
                "import { router } from 'expo-router'; export const Route = () => <View/>;"
            ),
        })
        self.assertNotEqual(widened.returncode, 0)

    def test_allows_exact_path_nudge_settings_router_primitives(self) -> None:
        result = self.run_checker({
            "apps/mobile/app/path/[pathID]/nudge-settings.tsx": (
                "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; "
                "export default function Route() { "
                "const { pathID = '' } = useLocalSearchParams<{ pathID: string }>(); "
                "useNavigation(); router.replace('/(tabs)/home'); "
                "return <Stack.Screen options={{ title: pathID }} />; }"
            ),
        })
        self.assertEqual(result.returncode, 0, result.stderr)

        widened = self.run_checker({
            "apps/mobile/app/path/[pathID]/nudge-settings.tsx": (
                "import { router, Stack, useLocalSearchParams, useNavigation } from 'expo-router'; "
                "export default function Route() { "
                "const { pathID = '' } = useLocalSearchParams<{ pathID: string }>(); "
                "useNavigation(); router.push('/'); "
                "return <Stack.Screen options={{ title: pathID }} />; }"
            ),
        })
        self.assertNotEqual(widened.returncode, 0)

    def test_rejects_unreviewed_presentation_capabilities(self) -> None:
        files = {
            "apps/web/src/lib/goto.ts": (
                "import { goto } from '@sveltejs/kit'; void goto('/paths');"
            ),
            "apps/web/src/lib/location.ts": "window.location.assign('/paths');",
            "apps/web/src/routes/bind/+page.svelte": (
                "<script>let form;</script><form bind:this={form}></form>"
            ),
            "apps/web/src/routes/style/+page.svelte": (
                "<script>export let target;</script><div style={target}></div>"
            ),
            "apps/web/src/routes/spread/+page.svelte": (
                "<script>export let attrs;</script><a {...attrs}>Link</a>"
            ),
            "apps/mobile/src/router.ts": (
                "import { router } from 'expo-router'; router.push('/paths');"
            ),
            "apps/mobile/src/ui/unreviewed-safe-area.tsx": (
                "import { SafeAreaView } from 'react-native-safe-area-context'; "
                "export const Screen = () => <SafeAreaView />;"
            ),
            "apps/mobile/src/ui/safe-area-alias.tsx": (
                "import { SafeAreaView as View } from 'react-native-safe-area-context'; "
                "export const Screen = () => <View />;"
            ),
            "apps/mobile/src/image.tsx": (
                "import { Image } from 'react-native'; "
                "export const Logo = () => <Image source={{ uri: '/logo.png' }} />;"
            ),
            "apps/mobile/src/linking.ts": (
                "import { Linking } from 'react-native'; "
                "export const leave = (target) => Linking.openURL(target);"
            ),
            "apps/mobile/src/linking-alias.ts": (
                "import { Linking as Transport } from 'react-native'; "
                "export const leave = (target) => Transport.openURL(target);"
            ),
            "apps/mobile/src/linking-namespace.ts": (
                "import * as Native from 'react-native'; "
                "export const leave = (target) => Native.Linking.openURL(target);"
            ),
            "apps/mobile/src/background.tsx": (
                "import { ImageBackground as Panel } from 'react-native'; "
                "export const Background = ({ source }) => <Panel source={source} />;"
            ),
        }
        result = self.run_checker(files)
        self.assertNotEqual(result.returncode, 0)
        for path in files:
            self.assertIn(path, result.stderr)

    def test_scans_api_client_and_rejects_extra_raw_transport(self) -> None:
        result = self.run_checker({
            "packages/api-client/src/raw-transport.ts": (
                "export const raw = (target) => fetch(target);"
            ),
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("packages/api-client/src/raw-transport.ts", result.stderr)

    def test_rejects_workspace_manifest_name_spoofing(self) -> None:
        result = self.run_checker({
            "packages/api-client/package.json": '{"name":"@attacker/api-client"}',
            "apps/web/src/lib/spoof.ts": "import client from '@attacker/api-client';",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("packages/api-client/package.json", result.stderr)

    def test_rejects_workspace_manifest_export_redirection(self) -> None:
        result = self.run_checker({
            "packages/api-client/package.json": (
                '{"name":"@hourpaths/api-client","exports":{".":"./src/raw.test.ts"}}'
            ),
            "packages/client-core/package.json": (
                '{"name":"@hourpaths/client-core","exports":"./src/raw.test.ts"}'
            ),
            "packages/i18n/package.json": (
                '{"name":"@hourpaths/i18n","exports":"./src/raw.test.ts"}'
            ),
            "packages/api-client/src/raw.test.ts": "export const raw = 'safe';",
            "packages/client-core/src/raw.test.ts": "export const raw = 'safe';",
            "packages/i18n/src/raw.test.ts": "export const raw = 'safe';",
        })
        self.assertNotEqual(result.returncode, 0)
        for manifest in (
            "packages/api-client/package.json",
            "packages/client-core/package.json",
            "packages/i18n/package.json",
        ):
            self.assertIn(manifest, result.stderr)

    def test_rejects_production_imports_of_ignored_api_client_tests(self) -> None:
        result = self.run_checker({
            "packages/api-client/src/bridge.ts": "export { raw } from './nested/raw.test.ts';",
            "packages/api-client/src/nested/raw.test.ts": (
                "export const raw = (target) => fetch(target);"
            ),
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("packages/api-client/src/bridge.ts", result.stderr)

    def test_rejects_cross_root_imports_of_test_and_fixture_sources(self) -> None:
        production_sources = {
            "apps/mobile/app/bridge.ts": (
                "export { raw } from '../../web/src/lib/raw.test.js';"
            ),
            "apps/mobile/src/bridge.ts": (
                "export { raw } from '../../../packages/client-core/src/raw-fixture.ts';"
            ),
            "apps/web/src/lib/bridge.ts": (
                "export { raw } from '../../../../packages/i18n/src/fixtures/raw.ts';"
            ),
            "packages/client-core/src/bridge.ts": (
                "export { raw } from '../../api-client/src/__fixtures__/raw.ts';"
            ),
            "packages/i18n/src/bridge.ts": (
                "export { raw } from '../../../apps/mobile/src/raw.test.ts';"
            ),
        }
        result = self.run_checker({
            **production_sources,
            "apps/web/src/lib/raw.test.ts": "export const raw = 'safe';",
            "packages/client-core/src/raw-fixture.ts": "export const raw = 'safe';",
            "packages/i18n/src/fixtures/raw.ts": "export const raw = 'safe';",
            "packages/api-client/src/__fixtures__/raw.ts": "export const raw = 'safe';",
            "apps/mobile/src/raw.test.ts": "export const raw = 'safe';",
        })
        self.assertNotEqual(result.returncode, 0)
        for path in production_sources:
            self.assertIn(path, result.stderr)

    def test_rejects_test_and_fixture_markers_in_auto_discovered_routes(self) -> None:
        routes = {
            "apps/mobile/app/raw.test.tsx": (
                "export default function TestRoute() { return null; }"
            ),
            "apps/web/src/routes/account.fixture/+page.svelte": "<main></main>",
        }
        result = self.run_checker(routes)
        self.assertNotEqual(result.returncode, 0)
        for path in routes:
            self.assertIn(path, result.stderr)

    def test_evaluates_marker_named_public_static_files(self) -> None:
        unsafe_static = {
            "apps/web/static/raw.test.js": "fetch('/v1/me');",
            "apps/web/static/icons/fixture.svg": "<svg></svg>",
        }
        result = self.run_checker(unsafe_static)
        self.assertNotEqual(result.returncode, 0)
        for path in unsafe_static:
            self.assertIn(path, result.stderr)

    def test_allows_provider_clients_only_in_protected_adapters(self) -> None:
        result = self.run_checker({
            "apps/mobile/src/provider-auth.ts": "import * as AuthSession from 'expo-auth-session'; AuthSession.exchangeCodeAsync(options, discovery);",
            "apps/mobile/src/provider-auth-state.ts": "export const state = 'pending';",
            "apps/web/src/lib/provider-auth.ts": "import { UserManager } from 'oidc-client-ts'; new UserManager(settings);",
            "packages/api-client/src/index.ts": "client.GET('/v1/me');",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("apps/mobile/src/provider-auth.ts", result.stderr)


if __name__ == "__main__":
    unittest.main()
