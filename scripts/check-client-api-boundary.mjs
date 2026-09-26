#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { createHash } from 'node:crypto';
import ts from '../apps/web/node_modules/typescript/lib/typescript.js';
import svelteCompiler from '../apps/web/node_modules/svelte/compiler/index.js';

const { parse: parseSvelte } = svelteCompiler;
const destinationProbe = '__hourpathsProvenDestination';

const root = path.resolve(process.argv[2] ?? '.');
const realRoot = fs.realpathSync(root);
const sourceRoots = [
  'apps/web/src',
  'apps/mobile/app',
  'apps/mobile/src',
  'packages/api-client/src',
  'packages/client-core/src',
  'packages/i18n/src',
];
const extensions = new Set(['.ts', '.tsx', '.mts', '.cts', '.js', '.jsx', '.mjs', '.cjs', '.svelte', '.html', '.css']);
const inertStaticExtensions = new Set(['.png', '.webp', '.jpg', '.jpeg', '.gif', '.avif', '.ico', '.woff', '.woff2', '.ttf', '.otf']);
const runtimeSourceSubstitutions = new Map([
  ['.js', ['.ts', '.tsx', '.js', '.jsx']],
  ['.jsx', ['.tsx', '.ts', '.jsx', '.js']],
  ['.mjs', ['.mts', '.mjs']],
  ['.cjs', ['.cts', '.cjs']],
]);
const nonProductionMarker = /(?:^|[._-])(?:fixtures?|tests?)(?:[._-]|$)/i;
const approvedSourceRoots = sourceRoots.map((sourceRoot) => path.join(root, sourceRoot));
const webLibRoot = path.join(root, 'apps/web/src/lib');
const approvedWorkspacePackages = new Map([
  ['packages/api-client/package.json', { name: '@hourpaths/api-client', exports: { '.': './src/index.ts' } }],
  ['packages/client-core/package.json', { name: '@hourpaths/client-core', exports: './src/index.ts' }],
  ['packages/i18n/package.json', { name: '@hourpaths/i18n', exports: './src/index.ts' }],
]);
const approvedExternalImports = new Set([
  '@sveltejs/kit', 'expo-auth-session', 'expo-constants', 'expo-crypto',
  'expo-image', 'expo-linking', 'expo-localization', 'expo-notifications', 'expo-router/unstable-native-tabs', 'expo-secure-store', 'expo-web-browser',
  '@expo/ui/community/segmented-control', '@expo/ui/swift-ui', '@expo/ui/swift-ui/modifiers',
  'i18next', 'node:assert/strict', 'node:test', 'oidc-client-ts', 'react', 'react-native',
  'react-native-safe-area-context',
  'svelte',
  ...[...approvedWorkspacePackages.values()].map((workspacePackage) => workspacePackage.name),
]);
const providerImports = new Map([
  ['apps/mobile/src/provider-auth.ts', new Set(['expo-auth-session', 'expo-web-browser'])],
  ['apps/mobile/src/push-notifications-native.ts', new Set(['expo-linking', 'expo-notifications'])],
  ['apps/web/src/lib/provider-auth.ts', new Set(['oidc-client-ts'])],
]);
const allProviderImports = new Set([...providerImports.values()].flatMap((values) => [...values]));
const safeGlobals = new Set([
  'Array', 'Boolean', 'Date', 'Error', 'Intl', 'JSON', 'Map', 'Math', 'Number', 'Object', 'Promise', 'Set', 'Symbol', 'URL',
  'clearTimeout', 'crypto', 'encodeURIComponent', 'setTimeout', 'WeakSet',
]);
const safeWindowMembers = new Set(['location', 'sessionStorage']);
const browserGlobalReferences = new Set([
  'content', 'document', 'frameElement', 'frames', 'globalThis',
  'navigator', 'open', 'opener', 'parent', 'self', 'top', 'window',
]);
const windowProxyMembers = new Set(['contentWindow', 'defaultView', 'view']);
const protectedProviderAdapters = new Set([
  'apps/mobile/src/provider-auth.ts',
  'apps/mobile/src/provider-auth-state.ts',
  'apps/mobile/src/provider-discovery.ts',
  'apps/mobile/src/push-notifications-native.ts',
  'apps/web/src/lib/provider-auth.ts',
]);
const protectedProviderManifest = 'scripts/protected-provider-adapters.sha256';
const protectedApiClientAdapters = new Set(['packages/api-client/src/index.ts']);
const protectedApiClientManifest = 'scripts/protected-api-client-adapter.sha256';
const protectedClientCapabilityAdapters = new Set([
  'apps/mobile/src/policy-link-native.ts',
  'apps/web/src/lib/accessibility-focus.ts',
  'apps/web/src/lib/device-locale.ts',
  'apps/web/src/lib/external-policy-link.ts',
  'apps/web/src/lib/notification-convergence-browser.ts',
]);
const protectedClientCapabilityConstructors = new Map([
  ['apps/web/src/lib/notification-convergence-browser.ts', new Set(['BroadcastChannel'])],
]);
const protectedClientCapabilityCallRoots = new Map([
  ['apps/web/src/lib/notification-convergence-browser.ts', new Set(['document'])],
]);
const protectedClientCapabilityManifest = 'scripts/protected-client-capability-adapters.sha256';
const generatedApiClientSources = new Set(['packages/api-client/src/schema.d.ts']);
const networkPrimitiveReferences = new Set([
  'EventSource', 'Request', 'RTCPeerConnection', 'SharedWorker', 'WebSocket',
  'WebTransport', 'Worker', 'XMLHttpRequest', 'fetch', 'require', 'sendBeacon',
]);
const approvedExpoRouterImports = new Map([
  ['apps/mobile/app/_layout.tsx', new Set(['Stack'])],
  ['apps/mobile/app/(tabs)/home/_layout.tsx', new Set(['Stack'])],
  ['apps/mobile/app/(tabs)/following/_layout.tsx', new Set(['Stack'])],
  ['apps/mobile/app/(tabs)/following/index.tsx', new Set(['Stack', 'router', 'useFocusEffect'])],
  ['apps/mobile/app/following/people.tsx', new Set(['Stack', 'router'])],
  ['apps/mobile/app/following/activity/[pathID]/[activityID].tsx', new Set(['router', 'Stack', 'useLocalSearchParams'])],
  ['apps/mobile/app/following/comments/[eventID].tsx', new Set(['router', 'useLocalSearchParams', 'useNavigation'])],
  ['apps/mobile/app/following/comments/[eventID]/hearts/[commentID].tsx', new Set(['router', 'useLocalSearchParams'])],
  ['apps/mobile/app/follow-requests.tsx', new Set(['router'])],
  ['apps/mobile/app/index.tsx', new Set(['router', 'useGlobalSearchParams', 'usePathname'])],
  ['apps/mobile/app/settings/index.tsx', new Set(['router'])],
  ['apps/mobile/app/settings/interactions.tsx', new Set(['router'])],
  ['apps/mobile/app/settings/time-zone.tsx', new Set(['router'])],
  ['apps/mobile/app/settings/notifications.tsx', new Set(['router'])],
  ['apps/mobile/app/notifications.tsx', new Set(['router', 'useFocusEffect'])],
  ['apps/mobile/app/invitations.tsx', new Set(['router'])],
  ['apps/mobile/app/settings/account.tsx', new Set(['router'])],
  ['apps/mobile/app/settings/blocked-accounts.tsx', new Set(['router', 'useFocusEffect'])],
  ['apps/mobile/app/path/[pathID].tsx', new Set(['router', 'Stack', 'useLocalSearchParams'])],
  ['apps/mobile/app/path/[pathID]/history/index.tsx', new Set(['router', 'Stack', 'useLocalSearchParams'])],
  ['apps/mobile/app/path/[pathID]/history/[activityID].tsx', new Set(['router', 'Stack', 'useLocalSearchParams', 'useNavigation'])],
  ['apps/mobile/app/path/[pathID]/members/index.tsx', new Set(['router', 'Stack', 'useLocalSearchParams'])],
  ['apps/mobile/app/path/[pathID]/members/[userID].tsx', new Set(['router', 'Stack', 'useLocalSearchParams', 'useNavigation'])],
  ['apps/mobile/app/path/[pathID]/nudge-settings.tsx', new Set(['router', 'Stack', 'useLocalSearchParams', 'useNavigation'])],
  ['apps/mobile/src/use-path-route-ancestry.ts', new Set(['useFocusEffect', 'useNavigation'])],
  ['apps/mobile/src/use-social-interaction-route-ancestry.ts', new Set(['useFocusEffect', 'useNavigation'])],
  ['apps/mobile/src/use-notification-journey-route-ancestry.ts', new Set(['useFocusEffect', 'useNavigation'])],
  ['apps/mobile/src/use-settings-journey-route-ancestry.ts', new Set(['useFocusEffect', 'useNavigation'])],
  ['apps/mobile/app/profile/[username].tsx', new Set(['router', 'Stack', 'useFocusEffect', 'useLocalSearchParams'])],
  ['apps/mobile/src/ui/path-header-menu.ios.tsx', new Set(['Stack'])],
  ['apps/mobile/src/ui/path-header-menu.tsx', new Set(['Stack'])],
  ['apps/mobile/src/ui/home-header-actions.ios.tsx', new Set(['Stack'])],
  ['apps/mobile/src/ui/following-header-actions.ios.tsx', new Set(['Stack'])],
]);
const approvedPlatformUIImports = new Map([
  ['apps/mobile/src/ui/platform-symbol.tsx', new Map([
    ['expo-symbols', new Set(['SymbolView', 'AndroidSymbol'])],
  ])],
  ['apps/mobile/src/ui/native-action-menu.android.tsx', new Map([
    ['@expo/ui/jetpack-compose', new Set(['DropdownMenu', 'DropdownMenuItem', 'Host', 'RNHostView', 'Text'])],
  ])],
]);
const approvedNavigationImports = new Map([
  ['apps/mobile/app/_layout.tsx', new Map([['@react-navigation/native', new Set(['ThemeProvider'])]])],
  ['apps/mobile/src/ui/navigation-theme.ts', new Map([['@react-navigation/native', new Set(['DarkTheme'])]])],
  ['apps/mobile/src/ui/native-sheet-frame.ios.tsx', new Map([
    ['@react-navigation/native', new Set(['NavigationContainer', 'NavigationIndependentTree'])],
    ['@react-navigation/native-stack', new Set(['createNativeStackNavigator'])],
  ])],
]);
const approvedNativeTabsImports = new Map([
  ['apps/mobile/app/(tabs)/_layout.tsx', new Set(['NativeTabs'])],
]);
const approvedExpoUIImports = new Map([
  ['apps/mobile/src/ui/comment-heart-icon.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Image'])],
  ])],
  ['apps/mobile/src/ui/home-arrangement-view.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button', 'HStack', 'List', 'Section', 'Spacer', 'Text'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'disabled', 'environment', 'listStyle', 'tint'])],
  ])],
  ['apps/mobile/src/ui/home-filter-chips.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button', 'HStack', 'ScrollView'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'accessibilityValue', 'buttonStyle', 'controlSize', 'environment', 'tint'])],
  ])],
  ['apps/mobile/src/ui/manual-occurrence-fields.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['DatePicker'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['datePickerStyle', 'disabled', 'environment', 'tint'])],
  ])],
  ['apps/mobile/src/ui/native-action-menu.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button', 'Image', 'Menu'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'disabled'])],
  ])],
  ['apps/mobile/src/ui/native-button.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button', 'Text'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'controlSize', 'disabled', 'frame', 'foregroundColor', 'tint'])],
  ])],
  ['apps/mobile/src/ui/native-choice-picker.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Picker', 'Text'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'disabled', 'environment', 'pickerStyle', 'tag', 'tint'])],
  ])],
  ['apps/mobile/src/ui/native-comment-send-button.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'disabled', 'tint'])],
  ])],
  ['apps/mobile/src/ui/native-content-unavailable.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['ContentUnavailableView'])],
  ])],
  ['apps/mobile/src/ui/native-header-button.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'disabled', 'labelStyle'])],
  ])],
  ['apps/mobile/src/ui/native-host.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Host'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['tint'])],
  ])],
  ['apps/mobile/src/ui/native-sheet-action.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'controlSize', 'frame', 'disabled', 'tint'])],
  ])],
  ['apps/mobile/src/ui/native-system-image.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Image'])],
  ])],
  ['apps/mobile/src/ui/native-timer-button.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'controlSize', 'disabled', 'foregroundColor', 'frame', 'tint'])],
  ])],
  ['apps/mobile/src/ui/native-tracking-button.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button', 'Image', 'VStack'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'buttonStyle', 'clipShape', 'controlSize', 'disabled', 'frame', 'tint'])],
  ])],
  ['apps/mobile/src/ui/settings-icon.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Image'])],
  ])],
  ['apps/mobile/src/ui/social-profile-avatar.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Image'])],
  ])],
  ['apps/mobile/src/ui/social-reaction-menu.ios.tsx', new Map([
    ['@expo/ui/swift-ui', new Set(['Button', 'Image', 'Menu'])],
    ['@expo/ui/swift-ui/modifiers', new Set(['accessibilityLabel', 'disabled', 'frame', 'tint'])],
  ])],
]);
const approvedSafeAreaImports = new Set([
  'apps/mobile/app/index.tsx',
  'apps/mobile/src/ui/native-route-presentation.tsx',
  'apps/mobile/src/ui/primitives.tsx',
  'apps/mobile/src/ui/native-sheet-frame.tsx',
  'apps/mobile/src/ui/practice-comments-view.tsx',
]);
const approvedSvelteComponentProps = new Map([
  ['apps/web/src/routes/+page.svelte', new Map([
    ['SocialProfileDiscovery', new Set([
      'blockBusy', 'blockError', 'blockedAccounts', 'blockedAccountsNextCursor', 'blockedAccountsOpen',
      'blockedAccountsState', 'blockReview', 'busyFollowRequestID', 'followRequests', 'followRequestsNextCursor', 'followRequestsOpen',
      'followRequestsState', 'i18n', 'loadingMore', 'nextCursor', 'onCloseFollowRequests',
      'onCancelBlock', 'onCancelUnblock', 'onCloseBlockedAccounts', 'onCloseProfile', 'onConfirmBlock',
      'onConfirmUnblock', 'onLoadMore', 'onLoadMoreBlockedAccounts', 'onLoadMoreFollowRequests',
      'onOpenBlockedAccounts', 'onOpenFollowRequests', 'onQueryChange', 'onRefreshProfile',
      'onRelationshipAction', 'onRetryBlockedAccounts', 'onRetrySearch', 'onReviewBlock',
      'onReviewFollowRequest', 'onReviewUnblock', 'onSearch',
      'onSelectProfile', 'profileState', 'query', 'relationshipBusy', 'relationshipError', 'results',
      'searchState', 'selectedProfile', 'unblockReview',
    ])],
    ['PathMemberAccess', new Set([
      'activities', 'activitiesFailed', 'activitiesLoading', 'activitiesNextCursor', 'archived', 'failed',
      'i18n', 'loading', 'loadingMore', 'members', 'nextCursor', 'onBackToMembers', 'onClose',
      'onCancelRoleChange', 'onChooseRole', 'onConfirmRoleChange', 'onLoadMore', 'onLoadMoreActivities',
      'onOpenActivity', 'onRefresh', 'onRemove', 'onRetryReview', 'onSelect', 'onUnblock', 'pendingRole',
      'removalBusy', 'removalError', 'review', 'reviewLoading', 'roleChangeBusy', 'roleChangeError',
      'selected', 'unblockBusy', 'unblockError',
    ])],
    ['PathMemberComparison', new Set([
      'failed', 'i18n', 'loading', 'loadingMore', 'members', 'nextCursor', 'onLoadMore', 'onRetry',
      'onSelect',
    ])],
  ])],
]);

function workspaceManifestViolations() {
  const violations = [];
  for (const [manifestPath, expected] of approvedWorkspacePackages) {
    const absolute = path.join(root, manifestPath);
    try {
      const metadata = fs.lstatSync(absolute);
      const manifest = JSON.parse(fs.readFileSync(absolute, 'utf8'));
      if (metadata.isSymbolicLink() || !metadata.isFile() || manifest.name !== expected.name ||
        JSON.stringify(manifest.exports) !== JSON.stringify(expected.exports)) violations.push(manifestPath);
    } catch { violations.push(manifestPath); }
  }
  return violations;
}

function filesBelow(directory, traversalViolations, includeAll = false) {
  if (!fs.existsSync(directory)) return [];
  try {
    const metadata = fs.lstatSync(directory);
    const realDirectory = fs.realpathSync(directory);
    if (metadata.isSymbolicLink() || !metadata.isDirectory() || !belowRoot(realDirectory, realRoot)) {
      traversalViolations.push(directory);
      return [];
    }
  } catch {
    traversalViolations.push(directory);
    return [];
  }
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isSymbolicLink()) {
      traversalViolations.push(target);
      return [];
    }
    if (entry.isDirectory()) {
      return filesBelow(target, traversalViolations, includeAll);
    }
    return includeAll || extensions.has(path.extname(entry.name)) ? [target] : [];
  });
}

function protectedAdapterViolations(adapters, manifestPath) {
  let manifest;
  try { manifest = fs.readFileSync(path.join(root, manifestPath), 'utf8'); }
  catch { return [manifestPath]; }
  const entries = new Map();
  for (const line of manifest.trim().split(/\r?\n/)) {
    const match = /^([0-9a-f]{64})  (.+)$/.exec(line);
    if (!match || entries.has(match[2])) return [manifestPath];
    entries.set(match[2], match[1]);
  }
  if (entries.size !== adapters.size ||
    [...adapters].some((adapter) => !entries.has(adapter))) return [manifestPath];
  const violations = [];
  for (const adapter of adapters) {
    try {
      const digest = createHash('sha256').update(fs.readFileSync(path.join(root, adapter))).digest('hex');
      if (digest !== entries.get(adapter)) violations.push(adapter);
    } catch { violations.push(adapter); }
  }
  return violations;
}

function canonicalDestinationText(text) {
  const codePoint = (encoded, radix, original) => {
    const value = Number.parseInt(encoded, radix);
    return Number.isSafeInteger(value) && value <= 0x10ffff ? String.fromCodePoint(value) : original;
  };
  return text
    .replace(/&sol;/gi, '/')
    .replace(/&#(?:x([0-9a-f]+)|([0-9]+));/gi, (match, hex, decimal) =>
      codePoint(hex ?? decimal, hex ? 16 : 10, match))
    .replace(/\\x([0-9a-f]{2})|\\u([0-9a-f]{4})|\\u\{([0-9a-f]+)\}/gi, (match, byte, word, point) =>
      codePoint(byte ?? word ?? point, 16, match))
    .replace(/%([0-9a-f]{2})/gi, (match, byte) => codePoint(byte, 16, match))
    .replace(/\\/g, '/');
}

function applicationApiDestinationText(text) {
  const compact = canonicalDestinationText(text).replace(/[\s'"`+${}()]/g, '');
  return /\/v1(?:\/|$|[?#])/.test(compact);
}

function provenNonApiDestination(text) {
  const destination = canonicalDestinationText(text).trim();
  if (!destination || destination.includes('\u0000') || /[\u0001-\u001f\u007f]/.test(destination)) return false;
  if (applicationApiDestinationText(destination)) return false;
  if (destination.startsWith('/') && !destination.startsWith('//')) return true;
  return /^https?:\/\/[^\s/$.?#].*$/i.test(destination);
}

const svelteTemplateNodeTypes = new Set([
  'AnimateDirective', 'AttachTag', 'Attribute', 'AwaitBlock', 'BindDirective', 'ClassDirective',
  'Comment', 'Component', 'ConstTag', 'DebugTag', 'DeclarationTag', 'EachBlock', 'ExpressionTag',
  'Fragment', 'HtmlTag', 'IfBlock', 'KeyBlock', 'LetDirective', 'OnDirective', 'RegularElement',
  'RenderTag', 'SlotElement', 'SnippetBlock', 'SpreadAttribute', 'StyleDirective', 'SvelteBody',
  'SvelteBoundary', 'SvelteComponent', 'SvelteDocument', 'SvelteElement', 'SvelteFragment', 'SvelteHead',
  'SvelteOptions', 'SvelteSelf', 'SvelteWindow', 'Text', 'TitleElement', 'TransitionDirective', 'UseDirective',
]);
const executableURLAttributes = new Set([
  'action', 'data', 'formaction', 'href', 'imagesrcset', 'ping', 'poster', 'src', 'srcdoc', 'srcset',
]);

function inspectSvelteMarkup(source, relative) {
  let parsed;
  try { parsed = parseSvelte(source, { modern: true }); }
  catch { return { malformed: true, apiDestination: false, expressions: [], destinations: [], instanceSource: '', scripts: [] }; }
  const expressions = [];
  const destinations = [];
  let apiDestination = false;
  function attributeParts(attribute) {
    if (attribute?.type !== 'Attribute' || attribute.value === true) return undefined;
    return Array.isArray(attribute.value) ? attribute.value : [attribute.value];
  }
  function staticAttribute(attribute) {
    const parts = attributeParts(attribute);
    return parts?.map((part) => part.type === 'Text' ? part.data : '\u0000').join('');
  }
  function visit(value) {
    if (!value || typeof value !== 'object') return;
    if (Array.isArray(value)) {
      for (const item of value) visit(item);
      return;
    }
    if (!svelteTemplateNodeTypes.has(value.type) && Number.isInteger(value.start) && Number.isInteger(value.end)) {
      expressions.push(source.slice(value.start, value.end));
      return;
    }
    if (value.type === 'HtmlTag' || value.type === 'SpreadAttribute' || value.type === 'StyleDirective' ||
      (value.type === 'BindDirective' && value.name === 'this')) apiDestination = true;
    if (value.type === 'Attribute' && attributeParts(value)) {
      const parts = attributeParts(value);
      const staticValue = staticAttribute(value);
      const name = value.name.toLowerCase();
      const cssRequest = name === 'style' && /\b(?:url|image-set)\s*\(/i.test(staticValue);
      const urlRequest = executableURLAttributes.has(name) || name.endsWith(':href');
      if (name === 'style' && staticValue.includes('\u0000')) apiDestination = true;
      if (cssRequest && applicationApiDestinationText(staticValue)) apiDestination = true;
      if (urlRequest) {
        const dynamic = parts.filter((part) => part.type !== 'Text');
        const literal = parts.filter((part) => part.type === 'Text').map((part) => part.data).join('');
        if (dynamic.length === 0) {
          if (!provenNonApiDestination(staticValue)) apiDestination = true;
        } else if (dynamic.length === 1 && literal.length === 0 && dynamic[0].expression) {
          destinations.push(source.slice(dynamic[0].expression.start, dynamic[0].expression.end));
        } else if (!provenNonApiDestination(staticValue)) apiDestination = true;
      }
      if (/^on/i.test(name) && !staticValue.includes('\u0000') && staticValue.trim()) expressions.push(staticValue);
    }
    if (value.type === 'Component' || value.type === 'SvelteComponent') {
      for (const attribute of value.attributes ?? []) {
        const staticValue = staticAttribute(attribute);
        if (staticValue === undefined) continue;
        if (approvedSvelteComponentProps.get(relative)?.get(value.name)?.has(attribute.name)) continue;
        const parts = attributeParts(attribute);
        const dynamic = parts.filter((part) => part.type !== 'Text');
        const literal = parts.filter((part) => part.type === 'Text').map((part) => part.data).join('');
        if (dynamic.length === 1 && literal.length === 0 && dynamic[0].expression) {
          destinations.push(source.slice(dynamic[0].expression.start, dynamic[0].expression.end));
        } else if (!provenNonApiDestination(staticValue)) apiDestination = true;
      }
    }
    if (value.type === 'RegularElement' && value.name.toLowerCase() === 'meta') {
      const attributes = new Map(value.attributes
        .filter((attribute) => attribute.type === 'Attribute')
        .map((attribute) => [attribute.name.toLowerCase(), staticAttribute(attribute)]));
      if (attributes.get('http-equiv')?.toLowerCase() === 'refresh') {
        apiDestination = true;
      }
    }
    if (value.type === 'RegularElement' && value.name.toLowerCase() === 'script') {
      const nodes = value.fragment?.nodes ?? [];
      if (nodes.length > 0) expressions.push(source.slice(nodes[0].start, nodes.at(-1).end));
    }
    if (value.type === 'RegularElement' && value.name.toLowerCase() === 'style') {
      const nodes = value.fragment?.nodes ?? [];
      const stylesheet = nodes.length > 0 ? source.slice(nodes[0].start, nodes.at(-1).end) : '';
      if (/\b(?:url|image-set)\s*\(/i.test(stylesheet) && applicationApiDestinationText(stylesheet)) apiDestination = true;
    }
    for (const [key, child] of Object.entries(value)) {
      if (key !== 'loc' && key !== 'name_loc') visit(child);
    }
  }
  function visitCSS(value) {
    if (!value || typeof value !== 'object') return;
    if (Array.isArray(value)) {
      for (const item of value) visitCSS(item);
      return;
    }
    if (value.type === 'Declaration' && /\b(?:url|image-set)\s*\(/i.test(value.value) &&
      applicationApiDestinationText(value.value)) apiDestination = true;
    if (value.type === 'Atrule' && value.name?.toLowerCase() === 'import' &&
      applicationApiDestinationText(value.prelude ?? '')) apiDestination = true;
    for (const child of Object.values(value)) visitCSS(child);
  }
  visit(parsed.fragment);
  visitCSS(parsed.css);
  const scriptSource = (script) => script?.content ? source.slice(script.content.start, script.content.end) : '';
  const instanceSource = scriptSource(parsed.instance);
  const scripts = [scriptSource(parsed.module), instanceSource].filter(Boolean);
  return { malformed: false, apiDestination, expressions, destinations, instanceSource, scripts };
}

function lexicalBindings(sourceFile) {
  const bindings = new Map();
  function hasModifier(node, kind) {
    return ts.canHaveModifiers(node) && ts.getModifiers(node)?.some((modifier) => modifier.kind === kind);
  }
  function isAmbient(node) {
    if (sourceFile.isDeclarationFile) return true;
    for (let current = node; current; current = current.parent) {
      if (hasModifier(current, ts.SyntaxKind.DeclareKeyword)) return true;
    }
    return false;
  }
  function runtimeEnum(node) {
    return !isAmbient(node) && !hasModifier(node, ts.SyntaxKind.ConstKeyword);
  }
  function addBindingTo(scope, name) {
    const names = bindings.get(scope);
    if (ts.isIdentifier(name)) names.add(name.text);
    else if (ts.isObjectBindingPattern(name) || ts.isArrayBindingPattern(name)) {
      for (const element of name.elements) if (ts.isBindingElement(element)) addBindingTo(scope, element.name);
    }
  }
  function addBinding(name) {
    addBindingTo(scopeStack.at(-1), name);
  }
  function isLexicalScope(node) {
    const classLike = ts.isClassDeclaration(node) || ts.isClassExpression(node);
    return ts.isSourceFile(node) || ts.isFunctionLike(node) || ts.isBlock(node) ||
      ts.isModuleDeclaration(node) || ts.isModuleBlock(node) ||
      ts.isCatchClause(node) || ts.isForStatement(node) || ts.isForInStatement(node) ||
      ts.isForOfStatement(node) || ts.isCaseBlock(node) || classLike;
  }
  let scopeStack = [];
  function visit(node) {
    const outerScope = scopeStack.at(-1);
    const runtimeFunction = ts.isFunctionDeclaration(node) && Boolean(node.body) && !isAmbient(node);
    const runtimeClass = ts.isClassDeclaration(node) && !isAmbient(node);
    const runtimeNamespace = ts.isModuleDeclaration(node) && ts.isIdentifier(node.name) && !isAmbient(node);
    if ((runtimeFunction || runtimeClass || (ts.isEnumDeclaration(node) && runtimeEnum(node)) || runtimeNamespace) && node.name && outerScope) {
      addBindingTo(outerScope, node.name);
    }
    const opensScope = isLexicalScope(node);
    if (opensScope) {
      if (!bindings.has(node)) bindings.set(node, new Set());
      scopeStack.push(node);
      if ((ts.isFunctionLike(node) || ts.isClassDeclaration(node) || ts.isClassExpression(node)) && node.name) addBinding(node.name);
    }
    if (ts.isImportClause(node)) {
      if (!node.isTypeOnly && node.name) addBinding(node.name);
      if (node.namedBindings) {
        if (!node.isTypeOnly && ts.isNamespaceImport(node.namedBindings)) addBinding(node.namedBindings.name);
        else if (ts.isNamedImports(node.namedBindings)) {
          for (const element of node.namedBindings.elements) if (!node.isTypeOnly && !element.isTypeOnly) addBinding(element.name);
        }
      }
    }
    if (ts.isImportEqualsDeclaration(node) && !node.isTypeOnly && !isAmbient(node)) addBinding(node.name);
    if (ts.isParameter(node)) addBinding(node.name);
    if (ts.isVariableDeclaration(node) && !isAmbient(node)) {
      const declarationList = ts.isVariableDeclarationList(node.parent) ? node.parent : undefined;
      const blockScoped = declarationList && (declarationList.flags & ts.NodeFlags.BlockScoped) !== 0;
      const target = blockScoped || ts.isCatchClause(node.parent)
        ? scopeStack.at(-1)
        : [...scopeStack].reverse().find((scope) => ts.isSourceFile(scope) || ts.isFunctionLike(scope) || ts.isModuleBlock(scope));
      if (target) addBindingTo(target, node.name);
    }
    ts.forEachChild(node, visit);
    if (opensScope) scopeStack.pop();
  }
  visit(sourceFile);
  return bindings;
}

function rootIdentifierNode(expression) {
  let current = expression;
  while (ts.isPropertyAccessExpression(current) || ts.isElementAccessExpression(current)) current = current.expression;
  while (ts.isParenthesizedExpression(current) || ts.isAsExpression(current) || ts.isNonNullExpression(current)) current = current.expression;
  return ts.isIdentifier(current) ? current : undefined;
}

function firstMember(expression, globalName) {
  let current = expression;
  while (ts.isPropertyAccessExpression(current) || ts.isElementAccessExpression(current)) {
    const member = ts.isPropertyAccessExpression(current)
      ? current.name.text
      : ts.isStringLiteral(current.argumentExpression) ? current.argumentExpression.text : undefined;
    if (ts.isIdentifier(current.expression) && current.expression.text === globalName) return member;
    current = current.expression;
  }
  return undefined;
}

function isNonReferenceName(node) {
  const parent = node.parent;
  const declarationName = (
    ts.isMethodDeclaration(parent) || ts.isPropertyDeclaration(parent) ||
    ts.isPropertySignature(parent) || ts.isMethodSignature(parent) ||
    ts.isGetAccessorDeclaration(parent) || ts.isSetAccessorDeclaration(parent) ||
    ts.isEnumMember(parent) || ts.isJsxAttribute(parent)
  ) && parent.name === node;
  const importExportName = (ts.isImportSpecifier(parent) || ts.isExportSpecifier(parent)) && parent.propertyName === node;
  return (ts.isPropertyAccessExpression(parent) && parent.name === node) ||
    (ts.isPropertyAssignment(parent) && parent.name === node) ||
    (ts.isBindingElement(parent) && parent.propertyName === node) ||
    declarationName || importExportName;
}

function isTypeOnlyReference(node) {
  for (let current = node; current; current = current.parent) {
    if (ts.isTypeNode(current) || ts.isInterfaceDeclaration(current) ||
      ts.isTypeAliasDeclaration(current) || ts.isTypeParameterDeclaration(current)) return true;
    if (ts.isImportDeclaration(current)) {
      const clause = current.importClause;
      if (clause?.isTypeOnly) return true;
      if (ts.isImportSpecifier(node.parent) && node.parent.isTypeOnly) return true;
    }
    if (ts.isExportDeclaration(current) && current.isTypeOnly) return true;
    if (ts.isExportSpecifier(node.parent) && node.parent.isTypeOnly) return true;
    if (ts.isStatement(current) || ts.isSourceFile(current)) return false;
  }
  return false;
}

function belowApprovedSourceRoot(file) {
  return approvedSourceRoots.some((sourceRoot) => file === sourceRoot || file.startsWith(`${sourceRoot}${path.sep}`));
}

function belowRoot(file, sourceRoot) {
  return file === sourceRoot || file.startsWith(`${sourceRoot}${path.sep}`);
}

function realPathBelow(file, sourceRoot) {
  try {
    const metadata = fs.lstatSync(file);
    if (metadata.isSymbolicLink() || !metadata.isFile()) return false;
    const realFile = fs.realpathSync(file);
    const realSourceRoot = fs.realpathSync(sourceRoot);
    return belowRoot(realSourceRoot, realRoot) && belowRoot(realFile, realSourceRoot);
  } catch { return false; }
}

function belowApprovedRealSourceRoot(file) {
  return approvedSourceRoots.some((sourceRoot) => realPathBelow(file, sourceRoot));
}

function resolveImportBase(base) {
  const explicitExtension = path.extname(base);
  if (explicitExtension && !extensions.has(explicitExtension)) return undefined;
  const substitutions = runtimeSourceSubstitutions.get(explicitExtension);
  const candidates = explicitExtension
    ? substitutions?.map((extension) => `${base.slice(0, -explicitExtension.length)}${extension}`) ?? [base]
    : [base, `${base}.d.ts`, ...[...extensions].flatMap((extension) => [`${base}${extension}`, path.join(base, `index${extension}`)])];
  return candidates.find((candidate) => {
    try { return fs.existsSync(candidate) && fs.statSync(candidate).isFile(); }
    catch { return false; }
  });
}

function resolveRelativeImport(importer, specifier) {
  return resolveImportBase(path.resolve(path.dirname(importer), specifier));
}

function nonProductionSource(relative) {
  return relative.split('/').some((part) => nonProductionMarker.test(part));
}

function autoDiscoveredRouteSource(relative) {
  return relative.startsWith('apps/mobile/app/') || relative.startsWith('apps/web/src/routes/');
}

function approvedResolvedImport(imported, importerRelative) {
  if (!imported || !belowApprovedSourceRoot(imported) || !belowApprovedRealSourceRoot(imported)) return false;
  const importedRelative = path.relative(root, imported).split(path.sep).join('/');
  return nonProductionSource(importerRelative) || !nonProductionSource(importedRelative);
}

function importAllowed(specifier, relative, file) {
  if (specifier === '@react-navigation/native' || specifier === '@react-navigation/native-stack') return approvedNavigationImports.get(relative)?.has(specifier) ?? false;
  if (specifier === 'expo-symbols' || specifier === '@expo/ui/jetpack-compose') return approvedPlatformUIImports.get(relative)?.has(specifier) ?? false;
  if (specifier === '$env/dynamic/private') return relative === 'apps/web/src/lib/server/config.ts';
  if (specifier === '@expo/ui/swift-ui' || specifier === '@expo/ui/swift-ui/modifiers') {
    return approvedExpoUIImports.get(relative)?.has(specifier) ?? false;
  }
  if (specifier === 'expo-router') {
    return approvedExpoRouterImports.has(relative);
  }
  if (specifier === 'expo-router/unstable-native-tabs') {
    return approvedNativeTabsImports.has(relative);
  }
  if (specifier === 'react-native-safe-area-context') {
    return approvedSafeAreaImports.has(relative);
  }
  if (specifier === '$lib' || specifier.startsWith('$lib/')) {
    if (!relative.startsWith('apps/web/src/')) return false;
    const suffix = specifier === '$lib' ? '' : specifier.slice('$lib/'.length);
    const imported = resolveImportBase(path.join(webLibRoot, suffix));
    return Boolean(imported && belowRoot(imported, webLibRoot) && realPathBelow(imported, webLibRoot) &&
      (nonProductionSource(relative) || !nonProductionSource(path.relative(root, imported).split(path.sep).join('/'))));
  }
  if (specifier.startsWith('.')) {
    if (specifier === './$types') return true;
    if (relative === 'packages/i18n/src/index.ts' && specifier.startsWith('./locales/') && specifier.endsWith('.json')) {
      const imported = path.resolve(path.dirname(file), specifier);
      const localesRoot = path.join(root, 'packages/i18n/src/locales');
      return belowRoot(imported, localesRoot) && realPathBelow(imported, localesRoot);
    }
    const imported = resolveRelativeImport(file, specifier);
    return approvedResolvedImport(imported, relative);
  }
  if (!approvedExternalImports.has(specifier)) return false;
  if (!allProviderImports.has(specifier)) return true;
  return providerImports.get(relative)?.has(specifier) ?? false;
}

function inspectSource(relative, file, source, index) {
  const kind = relative.endsWith('.tsx') || relative.endsWith('.jsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const scriptName = relative.endsWith('.svelte') ? `${relative}.script-${index}.ts` : relative;
  const sourceFile = ts.createSourceFile(scriptName, source, ts.ScriptTarget.Latest, true, kind);
  if (sourceFile.parseDiagnostics.length > 0) return true;
  const bindings = lexicalBindings(sourceFile);
  function isLexicallyBound(identifier) {
    return bindingScope(identifier) !== undefined;
  }
  function bindingScope(identifier) {
    for (let current = identifier.parent; current; current = current.parent) {
      if (bindings.get(current)?.has(identifier.text)) return current;
    }
    return undefined;
  }
  const scopeKey = (scope, name) => `${scope.kind}:${scope.pos}:${scope.end}:${name}`;
  const bindingKey = (identifier) => {
    const scope = bindingScope(identifier);
    return scope ? scopeKey(scope, identifier.text) : undefined;
  };
  const initializers = new Map();
  const functionReturns = new Map();
  const topLevelFunctionReturns = new Map();
  function collectInitializers(node) {
    const declarationList = ts.isVariableDeclaration(node) && ts.isVariableDeclarationList(node.parent) ? node.parent : undefined;
    if (declarationList && (declarationList.flags & ts.NodeFlags.Const) !== 0 &&
      ts.isIdentifier(node.name) && node.initializer) {
      const key = bindingKey(node.name);
      if (key) initializers.set(key, node.initializer);
    }
    if (ts.isFunctionDeclaration(node) && node.name && node.parameters.length === 0 && node.body?.statements.length === 1 &&
      ts.isReturnStatement(node.body.statements[0]) && node.body.statements[0].expression) {
      const key = bindingKey(node.name);
      if (key) functionReturns.set(key, node.body.statements[0].expression);
      if (node.parent === sourceFile) topLevelFunctionReturns.set(node.name.text, node.body.statements[0].expression);
    }
    ts.forEachChild(node, collectInitializers);
  }
  collectInitializers(sourceFile);
  function staticArray(node, resolving = new Set()) {
    if (ts.isArrayLiteralExpression(node)) {
      return node.elements.map((element) => {
        if (ts.isOmittedExpression(element)) return '';
        if (ts.isSpreadElement(element)) return '\u0000';
        return staticText(element, resolving) ?? '\u0000';
      });
    }
    if (ts.isIdentifier(node)) {
      const key = bindingKey(node);
      if (!key || resolving.has(key) || !initializers.has(key)) return undefined;
      const next = new Set(resolving);
      next.add(key);
      return staticArray(initializers.get(key), next);
    }
    return undefined;
  }
  function staticText(node, resolving = new Set()) {
    if (ts.isStringLiteralLike(node)) return node.text;
    if (ts.isNumericLiteral(node)) return node.text;
    if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isNonNullExpression(node)) {
      return staticText(node.expression, resolving);
    }
    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
      const left = staticText(node.left, resolving);
      const right = staticText(node.right, resolving);
      return left === undefined || right === undefined ? undefined : left + right;
    }
    if (ts.isTemplateExpression(node)) {
      let text = node.head.text;
      for (const span of node.templateSpans) {
        const expression = staticText(span.expression, resolving);
        text += (expression ?? '\u0000') + span.literal.text;
      }
      return text;
    }
    if (ts.isCallExpression(node) &&
      (ts.isPropertyAccessExpression(node.expression) || ts.isElementAccessExpression(node.expression))) {
      const member = ts.isPropertyAccessExpression(node.expression)
        ? node.expression.name.text
        : ts.isStringLiteral(node.expression.argumentExpression) ? node.expression.argumentExpression.text : undefined;
      if (member === 'join') {
        const values = staticArray(node.expression.expression, resolving);
        const separator = node.arguments.length === 0 ? ',' : staticText(node.arguments[0], resolving);
        if (values && separator !== undefined) return values.join(separator);
      }
      if (member === 'concat') {
        const base = staticText(node.expression.expression, resolving);
        const additions = node.arguments.map((argument) => staticText(argument, resolving));
        if (base !== undefined && additions.every((value) => value !== undefined)) return base + additions.join('');
      }
    }
    if (ts.isCallExpression(node) && node.arguments.length === 0 && ts.isIdentifier(node.expression)) {
      const key = bindingKey(node.expression);
      const returned = key && functionReturns.has(key)
        ? functionReturns.get(key)
        : topLevelFunctionReturns.get(node.expression.text);
      if (returned && (!key || !resolving.has(key))) {
        const next = new Set(resolving);
        if (key) next.add(key);
        return staticText(returned, next);
      }
    }
    if (ts.isIdentifier(node)) {
      const key = bindingKey(node);
      if (!key || resolving.has(key) || !initializers.has(key)) return undefined;
      const next = new Set(resolving);
      next.add(key);
      return staticText(initializers.get(key), next);
    }
    return undefined;
  }
  function isModuleSpecifier(node) {
    const parent = node.parent;
    if (!parent) return false;
    return (ts.isImportDeclaration(parent) || ts.isExportDeclaration(parent)) && parent.moduleSpecifier === node ||
      (ts.isExternalModuleReference(parent) && parent.expression === node);
  }
  function sourceWideApiDestination(node) {
    if (protectedProviderAdapters.has(relative) || isModuleSpecifier(node)) return false;
    if (!ts.isStringLiteralLike(node) && !ts.isIdentifier(node) && !ts.isBinaryExpression(node) &&
      !ts.isTemplateExpression(node) && !ts.isCallExpression(node)) return false;
    const value = staticText(node);
    return value !== undefined && applicationApiDestinationText(value);
  }
  function destinationProven(node) {
    const value = node ? staticText(node) : undefined;
    return value !== undefined && provenNonApiDestination(value);
  }
  function locationReceiver(node) {
    if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isNonNullExpression(node)) {
      return locationReceiver(node.expression);
    }
    if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
      const member = ts.isPropertyAccessExpression(node)
        ? node.name.text
        : ts.isStringLiteral(node.argumentExpression) ? node.argumentExpression.text : undefined;
      if (member === 'location') return true;
    }
    if (ts.isIdentifier(node) && node.text === 'location') return true;
    if (ts.isIdentifier(node)) {
      const key = bindingKey(node);
      return Boolean(key && initializers.has(key) && locationReceiver(initializers.get(key)));
    }
    if (ts.isCallExpression(node) && node.arguments.length === 0 && ts.isIdentifier(node.expression)) {
      const key = bindingKey(node.expression);
      return Boolean(key && functionReturns.has(key) && locationReceiver(functionReturns.get(key)));
    }
    return false;
  }
  function navigationDestination(node) {
    const expression = node.expression;
    if (ts.isIdentifier(expression) && ['goto', 'navigate'].includes(expression.text)) return node.arguments[0];
    if (!ts.isPropertyAccessExpression(expression) && !ts.isElementAccessExpression(expression)) return undefined;
    const member = ts.isPropertyAccessExpression(expression)
      ? expression.name.text
      : ts.isStringLiteral(expression.argumentExpression) ? expression.argumentExpression.text : undefined;
    const receiver = expression.expression;
    if (member === 'goto' || member === 'assign' || (member === 'replace' && locationReceiver(receiver))) return node.arguments[0];
    if (member === undefined && locationReceiver(receiver)) return node.arguments[0];
    return undefined;
  }
  function reviewedCallbackLocationUse(node) {
    if (relative !== 'apps/web/src/routes/callback/+page.svelte') return false;
    let current = node;
    while ((ts.isPropertyAccessExpression(current.parent) || ts.isElementAccessExpression(current.parent)) &&
      current.parent.expression === current) current = current.parent;
    const call = current.parent;
    if (!ts.isCallExpression(call) || call.expression !== current || !ts.isPropertyAccessExpression(current)) return false;
    return current.name.text === 'replace' && call.arguments.length === 1 && staticText(call.arguments[0]) === '/';
  }
  function reviewedPathPeopleRouterReference(node) {
    if (![
      'apps/mobile/app/path/[pathID]/members/index.tsx',
      'apps/mobile/app/path/[pathID]/members/[userID].tsx',
      'apps/mobile/app/path/[pathID]/nudge-settings.tsx',
    ].includes(relative)) return true;
    if (!ts.isIdentifier(node) || node.text !== 'router') return true;
    if (ts.isImportSpecifier(node.parent)) return true;
    const member = node.parent;
    if (!ts.isPropertyAccessExpression(member) || member.expression !== node || member.name.text !== 'replace') return false;
    const call = member.parent;
    return ts.isCallExpression(call) && call.expression === member && call.arguments.length === 1 &&
      staticText(call.arguments[0]) === '/(tabs)/home';
  }
  function protectedBrowserCapability(relativeMember, node) {
    if (protectedClientCapabilityAdapters.has(relative)) return true;
    if (protectedProviderAdapters.has(relative)) return true;
    if (relativeMember === 'sessionStorage') return relative === 'apps/web/src/lib/auth.ts';
    if (relativeMember === 'location') return reviewedCallbackLocationUse(node);
    return false;
  }
  let violation = false;
  if (relative === 'apps/mobile/app/index.tsx') {
    const exported = sourceFile.statements.filter((statement) =>
      (ts.canHaveModifiers(statement) && ts.getModifiers(statement)?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword)) ||
      ts.isExportDeclaration(statement) || ts.isExportAssignment(statement));
    const indexRedirect = exported.find((statement) => ts.isFunctionDeclaration(statement) &&
      ts.getModifiers(statement)?.some((modifier) => modifier.kind === ts.SyntaxKind.DefaultKeyword));
    const home = exported.find((statement) => ts.isFunctionDeclaration(statement) && statement.name?.text === 'HomeScreen');
    const validFunction = (statement) => ts.isFunctionDeclaration(statement) && statement.parameters.length === 0 &&
      !statement.asteriskToken &&
      !ts.getModifiers(statement)?.some((modifier) => modifier.kind === ts.SyntaxKind.AsyncKeyword);
    if (exported.length !== 2 || !validFunction(indexRedirect) || indexRedirect.name?.text !== 'IndexRedirect' ||
      !validFunction(home) || ts.getModifiers(home)?.some((modifier) => modifier.kind === ts.SyntaxKind.DefaultKeyword)) {
      violation = true;
    }
  }
  function visit(node) {
    if (!reviewedPathPeopleRouterReference(node)) violation = true;
    if (sourceWideApiDestination(node)) violation = true;
    if (ts.isIdentifier(node) && browserGlobalReferences.has(node.text) &&
      !isNonReferenceName(node) && !isLexicallyBound(node)) {
      if (protectedClientCapabilityAdapters.has(relative)) { /* Reviewed browser capability adapter. */ }
      else if (node.text !== 'window') violation = true;
      else {
        const parent = node.parent;
        const directMember = (ts.isPropertyAccessExpression(parent) || ts.isElementAccessExpression(parent)) && parent.expression === node
          ? firstMember(parent, 'window')
          : undefined;
        if (!directMember || !safeWindowMembers.has(directMember) || !protectedBrowserCapability(directMember, node)) violation = true;
      }
    }
    if (ts.isIdentifier(node) && networkPrimitiveReferences.has(node.text) &&
      !isNonReferenceName(node) && !isTypeOnlyReference(node) && !isLexicallyBound(node)) violation = true;
    if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
      const rootNode = rootIdentifierNode(node);
      if (rootNode?.text === 'window' && !isLexicallyBound(rootNode)) {
        const member = firstMember(node, 'window');
        if (!protectedClientCapabilityAdapters.has(relative) &&
          (!member || !safeWindowMembers.has(member) || !protectedBrowserCapability(member, node))) violation = true;
      }
    }
    if ((ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
      ts.isIdentifier(node.tagName) && node.tagName.text === 'Image') violation = true;
    if (ts.isJsxAttribute(node) && executableURLAttributes.has(node.name.text.toLowerCase())) {
      const initializer = node.initializer;
      const destination = initializer && ts.isJsxExpression(initializer) ? initializer.expression : initializer;
      const element = node.parent.parent;
      const nativeListData = node.name.text.toLowerCase() === 'data' &&
        (ts.isJsxOpeningElement(element) || ts.isJsxSelfClosingElement(element)) &&
        ts.isIdentifier(element.tagName) && element.tagName.text === 'FlatList';
      if (!nativeListData && !destinationProven(destination)) violation = true;
    }
    if (ts.isJsxSpreadAttribute(node)) violation = true;
    if (ts.isTaggedTemplateExpression(node)) {
      const rootNode = rootIdentifierNode(node.tag);
      if (rootNode?.text === 'String' && !isLexicallyBound(rootNode)) violation = true;
    }
    if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
      const member = ts.isPropertyAccessExpression(node)
        ? node.name.text
        : ts.isStringLiteral(node.argumentExpression) ? node.argumentExpression.text : undefined;
      if (member && windowProxyMembers.has(member) && !protectedClientCapabilityAdapters.has(relative)) violation = true;
    }
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier) && !importAllowed(node.moduleSpecifier.text, relative, file)) {
      violation = true;
    }
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      const specifier = node.moduleSpecifier.text;
      const clause = node.importClause;
      if (specifier === '@sveltejs/kit' && clause && !clause.isTypeOnly) {
        if (clause.namedBindings && (ts.isNamespaceImport(clause.namedBindings) || ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.some((element) => !element.isTypeOnly && (element.propertyName ?? element.name).text === 'goto'))) violation = true;
      }
      if (specifier === '@expo/ui/swift-ui' || specifier === '@expo/ui/swift-ui/modifiers') {
        const reviewedImports = approvedExpoUIImports.get(relative)?.get(specifier);
        const exactNativeUIImport = reviewedImports && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === reviewedImports.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && reviewedImports.has((element.propertyName ?? element.name).text));
        if (!exactNativeUIImport) violation = true;
      }
      if (specifier === '@react-navigation/native' || specifier === '@react-navigation/native-stack') {
        const reviewedImports = approvedNavigationImports.get(relative)?.get(specifier);
        const exactNavigationImport = reviewedImports && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === reviewedImports.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && reviewedImports.has(element.name.text));
        if (!exactNavigationImport) violation = true;
      }
      if (specifier === 'expo-symbols' || specifier === '@expo/ui/jetpack-compose') {
        const reviewedImports = approvedPlatformUIImports.get(relative)?.get(specifier);
        const exactPlatformImport = reviewedImports && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === reviewedImports.size &&
          clause.namedBindings.elements.every((element) => !element.propertyName && reviewedImports.has(element.name.text));
        if (!exactPlatformImport) violation = true;
      }
      if (specifier === 'react-native') {
        const allowed = new Set(['AccessibilityInfo', 'Button', 'SafeAreaView', 'ScrollView', 'Switch', 'Text', 'TextInput', 'View']);
        const uiAllowed = new Set(['ActivityIndicator', 'Alert', 'Button', 'FlatList', 'KeyboardAvoidingView', 'Modal', 'Platform', 'Pressable', 'RefreshControl', 'SafeAreaView', 'ScrollView', 'StyleSheet', 'Switch', 'Text', 'TextInput', 'View', 'useWindowDimensions']);
        const onboardingPresentationAllowed = new Set(['StyleSheet', 'Switch', 'View', 'useWindowDimensions']);
        const pathCreatePresentationAllowed = new Set([
          'AccessibilityInfo', 'InputAccessoryView', 'Keyboard', 'Pressable', 'StyleSheet', 'Switch', 'View',
        ]);
        const manualActivityPresentationAllowed = new Set(['AccessibilityInfo', 'StyleSheet', 'View']);
        const pathSharePresentationAllowed = new Set(['AccessibilityInfo', 'StyleSheet', 'View']);
        const pathArchivePresentationAllowed = new Set(['AccessibilityInfo']);
        const nudgeComposerPresentationAllowed = new Set(['AccessibilityInfo', 'Pressable', 'StyleSheet', 'View']);
        const commentEditPresentationAllowed = new Set([
          'AccessibilityInfo', 'Alert', 'InputAccessoryView', 'Keyboard', 'StyleSheet', 'View',
        ]);
        const practiceCommentsPresentationAllowed = new Set([
          'ActivityIndicator', 'Alert', 'FlatList', 'InputAccessoryView', 'Keyboard', 'KeyboardAvoidingView',
          'Platform', 'Pressable', 'StyleSheet', 'View', 'useWindowDimensions',
        ]);
        const reactionFallbackPresentationAllowed = new Set([
          'AccessibilityInfo', 'Modal', 'Pressable', 'ScrollView', 'StyleSheet', 'View',
        ]);
        const signedOutPresentationAllowed = new Set(['ActivityIndicator', 'ScrollView', 'StyleSheet', 'View', 'useWindowDimensions']);
        const trackingPresentationAllowed = new Set([
          'StyleSheet', 'View', 'useWindowDimensions',
        ]);
        const trackingFallbackAllowed = new Set([
          'Pressable', 'StyleSheet', 'Text', 'View', 'useWindowDimensions',
        ]);
        const exactPresentationImport = relative === 'apps/mobile/app/index.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.every((element) => element.isTypeOnly ||
            !element.propertyName && allowed.has(element.name.text));
        const exactAccountPresentationImport = relative === 'apps/mobile/app/settings/account.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 1 &&
          !clause.namedBindings.elements[0].isTypeOnly && !clause.namedBindings.elements[0].propertyName &&
          clause.namedBindings.elements[0].name.text === 'Alert';
        const exactSocialProfilePresentationImport = relative === 'apps/mobile/app/profile/[username].tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 2 &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && ['AccessibilityInfo', 'Alert'].includes(element.name.text));
        const exactBlockedAccountsPresentationImport = relative === 'apps/mobile/app/settings/blocked-accounts.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 2 &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && ['AccessibilityInfo', 'Alert'].includes(element.name.text));
        const exactNotificationPresentationImport = relative === 'apps/mobile/app/notifications.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 1 &&
          !clause.namedBindings.elements[0].isTypeOnly && !clause.namedBindings.elements[0].propertyName &&
          clause.namedBindings.elements[0].name.text === 'AppState';
        const exactFollowingPresentationImport = relative === 'apps/mobile/app/(tabs)/following/index.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 1 &&
          !clause.namedBindings.elements[0].isTypeOnly && !clause.namedBindings.elements[0].propertyName &&
          clause.namedBindings.elements[0].name.text === 'AppState';
        const exactInteractionSettingsPresentationImport = relative === 'apps/mobile/app/settings/interactions.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 2 &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && ['ActivityIndicator', 'View'].includes(element.name.text));
        const timeZoneSettingsPresentationAllowed = new Set([
          'ActivityIndicator', 'Alert', 'FlatList', 'InputAccessoryView', 'Keyboard', 'Pressable', 'StyleSheet', 'Text', 'TextInput', 'View',
        ]);
        const exactTimeZoneSettingsPresentationImport = relative === 'apps/mobile/app/settings/time-zone.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === timeZoneSettingsPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && timeZoneSettingsPresentationAllowed.has(element.name.text));
        const exactPathPresentationImport = relative === 'apps/mobile/src/ui/native-action-menu.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 2 &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && ['Alert', 'Button'].includes(element.name.text));
        const exactHeaderPresentationImport = relative === 'apps/mobile/src/ui/native-header-button.tsx' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 1 &&
          !clause.namedBindings.elements[0].isTypeOnly && !clause.namedBindings.elements[0].propertyName &&
          clause.namedBindings.elements[0].name.text === 'Button';
        const exactOnboardingPresentationImport = relative === 'apps/mobile/src/ui/onboarding-form.tsx' &&
          clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === onboardingPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && onboardingPresentationAllowed.has(element.name.text));
        const exactPathCreatePresentationImport = relative === 'apps/mobile/src/ui/path-create-form.tsx' &&
          clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === pathCreatePresentationAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && pathCreatePresentationAllowed.has(element.name.text));
        const exactManualActivityPresentationImport = relative === 'apps/mobile/src/ui/manual-activity-form.tsx' &&
          clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === manualActivityPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && manualActivityPresentationAllowed.has(element.name.text));
        const exactPathSharePresentationImport = relative === 'apps/mobile/src/ui/path-share-sheet.tsx' && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === pathSharePresentationAllowed.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && pathSharePresentationAllowed.has(element.name.text));
        const exactPathArchivePresentationImport = relative === 'apps/mobile/src/ui/path-archive-confirmation-sheet.tsx' && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === pathArchivePresentationAllowed.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && pathArchivePresentationAllowed.has(element.name.text));
        const exactNudgeComposerPresentationImport = relative === 'apps/mobile/src/ui/nudge-composer-sheet.tsx' && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === nudgeComposerPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && nudgeComposerPresentationAllowed.has(element.name.text));
        const exactCommentEditPresentationImport = relative === 'apps/mobile/src/ui/comment-edit-sheet.tsx' && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === commentEditPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && commentEditPresentationAllowed.has(element.name.text));
        const exactPracticeCommentsPresentationImport = relative === 'apps/mobile/src/ui/practice-comments-view.tsx' && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === practiceCommentsPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && practiceCommentsPresentationAllowed.has(element.name.text));
        const exactReactionFallbackPresentationImport = relative === 'apps/mobile/src/ui/social-reaction-menu.tsx' && clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === reactionFallbackPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) => !element.isTypeOnly && !element.propertyName && reactionFallbackPresentationAllowed.has(element.name.text));
        const exactSignedOutPresentationImport = relative === 'apps/mobile/src/ui/signed-out-screen.tsx' &&
          clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === signedOutPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && signedOutPresentationAllowed.has(element.name.text));
        const exactTrackingPresentationImport = relative === 'apps/mobile/src/ui/native-tracking-button.ios.tsx' &&
          clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === trackingPresentationAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && trackingPresentationAllowed.has(element.name.text));
        const exactTrackingFallbackImport = relative === 'apps/mobile/src/ui/native-tracking-button.tsx' &&
          clause && !clause.name && !clause.isTypeOnly &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === trackingFallbackAllowed.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && trackingFallbackAllowed.has(element.name.text));
        const exactUIPresentationImport = relative !== 'apps/mobile/src/ui/onboarding-form.tsx' &&
          relative !== 'apps/mobile/src/ui/path-create-form.tsx' &&
          relative !== 'apps/mobile/src/ui/manual-activity-form.tsx' &&
          relative !== 'apps/mobile/src/ui/path-share-sheet.tsx' &&
          relative !== 'apps/mobile/src/ui/path-archive-confirmation-sheet.tsx' &&
          relative !== 'apps/mobile/src/ui/nudge-composer-sheet.tsx' &&
          relative !== 'apps/mobile/src/ui/comment-edit-sheet.tsx' &&
          relative !== 'apps/mobile/src/ui/practice-comments-view.tsx' &&
          relative !== 'apps/mobile/src/ui/social-reaction-menu.tsx' &&
          relative !== 'apps/mobile/src/ui/signed-out-screen.tsx' &&
          relative !== 'apps/mobile/src/ui/native-tracking-button.ios.tsx' &&
          relative !== 'apps/mobile/src/ui/native-tracking-button.tsx' &&
          relative.startsWith('apps/mobile/src/ui/') && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.every((element) => element.isTypeOnly ||
            !element.propertyName && uiAllowed.has(element.name.text));
        const exactPolicyLinkImport = relative === 'apps/mobile/src/policy-link-native.ts' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) && clause.namedBindings.elements.length === 1 &&
          !clause.namedBindings.elements[0].isTypeOnly && !clause.namedBindings.elements[0].propertyName &&
          clause.namedBindings.elements[0].name.text === 'Linking';
        const exactPushAdapterImport = relative === 'apps/mobile/src/push-notifications-native.ts' && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 2 &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && ['AppState', 'Platform'].includes(element.name.text));
        if (!exactPresentationImport && !exactAccountPresentationImport && !exactSocialProfilePresentationImport && !exactBlockedAccountsPresentationImport && !exactNotificationPresentationImport &&
          !exactFollowingPresentationImport &&
          !exactInteractionSettingsPresentationImport &&
          !exactTimeZoneSettingsPresentationImport &&
          !exactPathPresentationImport && !exactHeaderPresentationImport &&
          !exactOnboardingPresentationImport &&
          !exactPathCreatePresentationImport &&
          !exactManualActivityPresentationImport &&
          !exactPathSharePresentationImport &&
          !exactPathArchivePresentationImport &&
          !exactNudgeComposerPresentationImport &&
          !exactCommentEditPresentationImport &&
          !exactPracticeCommentsPresentationImport &&
          !exactReactionFallbackPresentationImport &&
          !exactSignedOutPresentationImport &&
          !exactTrackingPresentationImport &&
          !exactTrackingFallbackImport &&
          !exactUIPresentationImport && !exactPolicyLinkImport && !exactPushAdapterImport) violation = true;
      }
      if (specifier === 'expo-router') {
        const reviewedImports = approvedExpoRouterImports.get(relative);
        const exactNativeStackImport = reviewedImports && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === reviewedImports.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && reviewedImports.has(element.name.text));
        if (!exactNativeStackImport) violation = true;
      }
      if (specifier === 'expo-router/unstable-native-tabs') {
        const reviewedImports = approvedNativeTabsImports.get(relative);
        const exactNativeTabsImport = reviewedImports && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === reviewedImports.size &&
          clause.namedBindings.elements.every((element) =>
            !element.isTypeOnly && !element.propertyName && reviewedImports.has(element.name.text));
        if (!exactNativeTabsImport) violation = true;
      }
      if (specifier === 'react-native-safe-area-context') {
        const exactSafeAreaImport = approvedSafeAreaImports.has(relative) && clause && !clause.name &&
          clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length === 1 &&
          !clause.namedBindings.elements[0].isTypeOnly && !clause.namedBindings.elements[0].propertyName &&
          clause.namedBindings.elements[0].name.text === (relative === 'apps/mobile/src/ui/practice-comments-view.tsx' ? 'useSafeAreaInsets' : 'SafeAreaView');
        if (!exactSafeAreaImport) violation = true;
      }
    }
    if (ts.isExportDeclaration(node)) {
      if (node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
        const specifier = node.moduleSpecifier.text;
        if (specifier === '@expo/ui/swift-ui' || specifier === '@expo/ui/swift-ui/modifiers' ||
          specifier === '@react-navigation/native' || specifier === '@react-navigation/native-stack' ||
          specifier === 'expo-symbols' || specifier === '@expo/ui/jetpack-compose' ||
          (relative === 'apps/mobile/src/ui/onboarding-form.tsx' && specifier === 'react-native') ||
          (relative === 'apps/mobile/src/ui/signed-out-screen.tsx' && specifier === 'react-native') ||
          allProviderImports.has(specifier) || !importAllowed(specifier, relative, file)) violation = true;
      }
    }
    if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference)) {
      const expression = node.moduleReference.expression;
      const specifier = expression && ts.isStringLiteral(expression) ? expression.text : undefined;
      if (!specifier || specifier === '@expo/ui/swift-ui' || specifier === '@expo/ui/swift-ui/modifiers' ||
        specifier === '@react-navigation/native' || specifier === '@react-navigation/native-stack' ||
          specifier === 'expo-symbols' || specifier === '@expo/ui/jetpack-compose' ||
        (relative === 'apps/mobile/src/ui/onboarding-form.tsx' && specifier === 'react-native') ||
        (relative === 'apps/mobile/src/ui/signed-out-screen.tsx' && specifier === 'react-native') ||
        !importAllowed(specifier, relative, file)) violation = true;
    }
    if (ts.isCallExpression(node)) {
      if (ts.isIdentifier(node.expression) && node.expression.text === destinationProbe) {
        if (node.arguments.length !== 1 || !destinationProven(node.arguments[0])) violation = true;
        for (const argument of node.arguments) visit(argument);
        return;
      }
      const destination = navigationDestination(node);
      if (destination !== undefined && !destinationProven(destination) &&
        !protectedClientCapabilityAdapters.has(relative)) violation = true;
      if (node.expression.kind === ts.SyntaxKind.ImportKeyword) violation = true;
      const rootNode = rootIdentifierNode(node.expression);
      const rootName = rootNode?.text;
      const reviewedCallRoot = rootName && protectedClientCapabilityCallRoots.get(relative)?.has(rootName);
      if (rootNode && ['globalThis', 'navigator', 'self'].includes(rootName) && !isLexicallyBound(rootNode)) violation = true;
      else if (rootNode && rootName !== 'window' && !isLexicallyBound(rootNode) && !safeGlobals.has(rootName) &&
        !(rootName === 'String' && ts.isIdentifier(node.expression)) && !reviewedCallRoot) violation = true;
    }
    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken) {
      const left = node.left;
      if (ts.isPropertyAccessExpression(left) || ts.isElementAccessExpression(left)) {
        const member = ts.isPropertyAccessExpression(left)
          ? left.name.text
          : ts.isStringLiteral(left.argumentExpression) ? left.argumentExpression.text : undefined;
        if ((member === 'href' && locationReceiver(left.expression)) || member === 'location') {
          if (!destinationProven(node.right)) violation = true;
        }
      }
    }
    if (ts.isNewExpression(node)) {
      const rootNode = rootIdentifierNode(node.expression);
      const rootName = rootNode?.text;
      const reviewedConstructor = rootName && protectedClientCapabilityConstructors.get(relative)?.has(rootName);
      if (rootNode && !isLexicallyBound(rootNode) && !safeGlobals.has(rootName) && !reviewedConstructor) violation = true;
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return violation;
}

const violations = [
  ...workspaceManifestViolations(),
  ...protectedAdapterViolations(protectedProviderAdapters, protectedProviderManifest),
  ...protectedAdapterViolations(protectedApiClientAdapters, protectedApiClientManifest),
  ...protectedAdapterViolations(protectedClientCapabilityAdapters, protectedClientCapabilityManifest),
];
const traversalViolations = [];
const sourceFiles = [
  ...sourceRoots.flatMap((sourceRoot) => filesBelow(path.join(root, sourceRoot), traversalViolations)),
  ...filesBelow(path.join(root, 'apps/web/static'), traversalViolations, true),
];
violations.push(...traversalViolations.map((file) => path.relative(root, file).split(path.sep).join('/')));
for (const file of sourceFiles) {
  const relative = path.relative(root, file).split(path.sep).join('/');
  if (relative.startsWith('apps/web/static/')) {
    if (!inertStaticExtensions.has(path.extname(file).toLowerCase())) violations.push(relative);
    continue;
  }
  if (nonProductionSource(relative)) {
    if (autoDiscoveredRouteSource(relative)) violations.push(relative);
    continue;
  }
  if (generatedApiClientSources.has(relative) || protectedApiClientAdapters.has(relative)) continue;
  const source = fs.readFileSync(file, 'utf8');
  const markup = file.endsWith('.svelte') || relative === 'apps/web/src/app.html' ? inspectSvelteMarkup(source, relative) : undefined;
  const scripts = markup ? markup.scripts : [source];
  const inlineViolation = markup?.expressions.some((expression, index) =>
    inspectSource(relative, file, `${markup.instanceSource}\n${expression};`, scripts.length + index));
  const destinationViolation = markup?.destinations.some((expression, index) =>
    inspectSource(relative, file, `${markup.instanceSource}\n${destinationProbe}(${expression});`, scripts.length + markup.expressions.length + index));
  if (markup?.malformed || markup?.apiDestination || inlineViolation || destinationViolation ||
    scripts.some((script, index) => inspectSource(relative, file, script, index))) violations.push(relative);
}
for (const relative of [...new Set(violations)].sort()) {
  console.error(`client API boundary: ${relative} uses client transport outside an approved generated or provider adapter`);
}
process.exitCode = violations.length > 0 ? 1 : 0;
