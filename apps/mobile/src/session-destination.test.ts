import assert from 'node:assert/strict';
import test from 'node:test';
import { activatePersistedMobileSession } from './session-exchange';
import { admitMobileSessionPaths, canCompleteMobileOnboarding, createMobileOnboardingDraft, expoCalendarWeekdayToISO, loadMobileHomeProfile, loadMobileOnboardingProfile, refreshMobileOnboardingPolicy, replaceMobileOnboardingUsername, validProfileDisplayName, validProfileUsername } from './session-destination';

const newYorkTimeZone = ['America', 'New_York'].join('/');
const kolkataTimeZone = ['Asia', 'Kolkata'].join('/');
const pathCapabilities = (overrides: Partial<Record<'inviteMembers' | 'leavePath' | 'manageGoals' | 'manageLifecycle' | 'manageMembers' | 'manageVisibility' | 'renamePath' | 'trackTime' | 'transferOwnership', boolean>> = {}) => ({
  inviteMembers: false,
  leavePath: false,
  manageGoals: false,
  manageLifecycle: false,
  manageMembers: false,
  manageVisibility: false,
  renamePath: false,
  trackTime: false,
  transferOwnership: false,
  ...overrides,
});
const homeOrganization = (classification: 'solo' | 'shared' | 'supporting' = 'solo') => ({ classification, pinned: false });
const homePreferences = { manualPathIds: ['path-1', 'path-supporter', 'path-2'], orderMethod: 'recent', pinnedPathIds: [], revision: 0 };

test('the active mobile Home loads its server-backed Path collection with the profile', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    const pathname = new URL(request.url).pathname;
    if (pathname === '/v1/me') {
      return Response.json({ data: { id: 'user-1', email: 'person@example.test', displayName: 'seed', profileVisibility: 'private' } });
    }
    if (pathname === '/v1/paths' && new URL(request.url).searchParams.get('archived') === 'true') {
      return Response.json({ data: [{ id: 'path-2', name: 'archived-path', visibility: 'private', archivedAt: '2026-07-21T11:00:00Z', capabilities: pathCapabilities({ manageLifecycle: true }), home: homeOrganization() }], meta: { homePreferences } });
    }
    if (pathname === '/v1/paths') return Response.json({ data: [
      { id: 'path-1', name: 'path-name-1', visibility: 'private', capabilities: pathCapabilities({ trackTime: true }), home: homeOrganization() },
      { id: 'path-supporter', name: 'supporter-path', visibility: 'private', capabilities: pathCapabilities(), home: homeOrganization('supporting') },
    ], meta: { homePreferences } });
    if (pathname === '/v1/path-invitations') return Response.json({
      data: [{
        invitation: {
          id: 'invitation-1',
          pathId: 'invited-path',
          inviterUserId: 'creator',
          recipientUserId: 'user-1',
          offeredRole: 'supporter',
          createdAt: '2026-07-21T10:00:00Z',
        },
        pathName: 'Shared reading',
        inviter: { userId: 'creator', username: 'reader.one', displayName: 'reader-display-name' },
      }],
      meta: { nextCursor: 'signed-next' },
    });
    if (pathname === '/v1/paths/path-1/timer') return Response.json({ data: { running: false, accumulatedSeconds: 90 } });
    return Response.json({ code: 'not_found' }, { status: 404 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const home = await loadMobileHomeProfile('https://api.example.test', {
    token: 'active-token',
    expiresAt: '2026-07-21T12:00:00Z',
    nextAction: 'home',
  });

  assert.deepEqual(home, {
    id: 'user-1',
    email: 'person@example.test',
    displayName: 'seed',
    profileVisibility: 'private',
    homePreferences,
    paths: [
      { id: 'path-1', name: 'path-name-1', visibility: 'private', capabilities: pathCapabilities({ trackTime: true }), home: homeOrganization() },
      { id: 'path-supporter', name: 'supporter-path', visibility: 'private', capabilities: pathCapabilities(), home: homeOrganization('supporting') },
    ],
    archivedPaths: [{ id: 'path-2', name: 'archived-path', visibility: 'private', archivedAt: '2026-07-21T11:00:00Z', capabilities: pathCapabilities({ manageLifecycle: true }), home: homeOrganization() }],
    pendingInvitations: {
      items: [{
        invitation: {
          id: 'invitation-1',
          pathId: 'invited-path',
          inviterUserId: 'creator',
          recipientUserId: 'user-1',
          offeredRole: 'supporter',
          createdAt: '2026-07-21T10:00:00Z',
        },
        pathName: 'Shared reading',
        inviter: { userId: 'creator', username: 'reader.one', displayName: 'reader-display-name' },
      }],
      nextCursor: 'signed-next',
    },
    timers: { 'path-1': { running: false, accumulatedSeconds: 90 } },
  });
  assert.deepEqual(requests.map((request) => [
    request.method,
    `${new URL(request.url).pathname}${new URL(request.url).search}`,
    request.headers.get('authorization'),
  ]), [
    ['GET', '/v1/me', 'Bearer active-token'],
    ['GET', '/v1/paths?limit=25', 'Bearer active-token'],
    ['GET', '/v1/paths?archived=true&limit=25', 'Bearer active-token'],
    ['GET', '/v1/path-invitations?limit=25', 'Bearer active-token'],
    ['GET', '/v1/paths/path-1/timer', 'Bearer active-token'],
  ]);
});

test('mobile Home loads every active and archived Path page before enabling exact preference updates', async (context) => {
  const originalFetch = globalThis.fetch;
  const path = (id: string, archivedAt?: string) => ({
    id,
    name: id,
    visibility: 'private',
    ...(archivedAt ? { archivedAt } : {}),
    capabilities: pathCapabilities({ trackTime: !archivedAt }),
    home: homeOrganization(),
  });
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    const url = new URL(request.url);
    if (url.pathname === '/v1/me') return Response.json({ data: { id: 'user-1', email: 'person@example.test', displayName: 'seed', profileVisibility: 'public' } });
    if (url.pathname === '/v1/path-invitations') return Response.json({ data: [], meta: {} });
    if (url.pathname.endsWith('/timer')) return Response.json({ data: { running: false, accumulatedSeconds: 0 } });
    if (url.pathname === '/v1/paths') {
      const archived = url.searchParams.get('archived') === 'true';
      const cursor = url.searchParams.get('cursor');
      if (archived) return Response.json(cursor
        ? { data: [path('archived-2', '2026-08-01T12:00:00Z')], meta: { homePreferences } }
        : { data: [path('archived-1', '2026-08-01T12:00:00Z')], meta: { nextCursor: 'archived-next', homePreferences } });
      return Response.json(cursor
        ? { data: [path('active-2')], meta: { homePreferences } }
        : { data: [path('active-1')], meta: { nextCursor: 'active-next', homePreferences } });
    }
    return Response.json({ code: 'not_found' }, { status: 404 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const home = await loadMobileHomeProfile('https://api.example.test', {
    token: 'active-token',
    expiresAt: '2026-07-21T12:00:00Z',
    nextAction: 'home',
  });

  assert.deepEqual(home.paths.map(({ id }) => id), ['active-1', 'active-2']);
  assert.deepEqual(home.archivedPaths.map(({ id }) => id), ['archived-1', 'archived-2']);
});

test('mobile Home fails closed when a Path projection has no usable name', async (context) => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    const url = new URL(request.url);
    if (url.pathname === '/v1/me') {
      return Response.json({ data: { id: 'user-1', email: 'person@example.test', displayName: 'seed', profileVisibility: 'private' } });
    }
    if (url.pathname === '/v1/paths' && url.searchParams.get('archived') === 'true') {
      return Response.json({ data: [], meta: {} });
    }
    if (url.pathname === '/v1/paths') {
      return Response.json({ data: [{
        id: 'path-blank',
        name: '   ',
        visibility: 'private',
        capabilities: pathCapabilities({ trackTime: true }),
      }], meta: {} });
    }
    if (url.pathname === '/v1/path-invitations') {
      return Response.json({ data: [], meta: {} });
    }
    return Response.json({ code: 'not_found' }, { status: 404 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  await assert.rejects(
    () => loadMobileHomeProfile('https://api.example.test', {
      token: 'active-token',
      expiresAt: '2026-07-21T12:00:00Z',
      nextAction: 'home',
    }),
    (failure: unknown) => {
      assert.deepEqual(failure, { kind: 'http', status: 502 });
      return true;
    },
  );
});

test('mobile Home retains only an authoritative public or private profile visibility', async (context) => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    const url = new URL(request.url);
    if (url.pathname === '/v1/me') {
      return Response.json({ data: { id: 'user-1', email: 'person@example.test', displayName: 'seed', profileVisibility: 'followers' } });
    }
    if (url.pathname === '/v1/paths') return Response.json({ data: [], meta: { homePreferences } });
    if (url.pathname === '/v1/path-invitations') return Response.json({ data: [], meta: {} });
    return Response.json({ code: 'not_found' }, { status: 404 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  await assert.rejects(
    () => loadMobileHomeProfile('https://api.example.test', {
      token: 'active-token',
      expiresAt: '2026-07-21T12:00:00Z',
      nextAction: 'home',
    }),
    (failure: unknown) => {
      assert.deepEqual(failure, { kind: 'http', status: 502 });
      return true;
    },
  );
});

test('mobile Path admission requires complete capabilities and unique identified rows', () => {
  const complete = {
    id: 'path-1',
    name: 'Reading',
    visibility: 'private',
    capabilities: {
      inviteMembers: false,
      leavePath: false,
      manageGoals: false,
      manageLifecycle: false,
      manageMembers: false,
      manageVisibility: false,
      renamePath: false,
      trackTime: true,
      transferOwnership: false,
    },
    home: homeOrganization(),
  };
  assert.deepEqual(admitMobileSessionPaths([complete], []), { active: [complete], archived: [] });
  assert.throws(
    () => admitMobileSessionPaths([{ ...complete, capabilities: { trackTime: true } }], []),
    (failure: unknown) => { assert.deepEqual(failure, { kind: 'http', status: 502 }); return true; },
  );
  assert.throws(
    () => admitMobileSessionPaths([complete], [{ ...complete, archivedAt: '2026-08-02T12:00:00Z' }]),
    (failure: unknown) => { assert.deepEqual(failure, { kind: 'http', status: 502 }); return true; },
  );
  for (const malformed of [
    { ...complete, intervalGoal: { recurrence: 'weekly', targetSeconds: 60, alignment: { isoWeekday: 0 } } },
    { ...complete, overallTarget: { targetSeconds: 0 } },
    { ...complete, archivedAt: '2026-02-30T12:00:00Z' },
  ]) {
    assert.throws(
      () => admitMobileSessionPaths([malformed], []),
      (failure: unknown) => { assert.deepEqual(failure, { kind: 'http', status: 502 }); return true; },
    );
  }
});

test('restricted mobile destinations call onboarding and never the Home profile endpoint', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    if (new URL(request.url).pathname === '/v1/onboarding') {
      return Response.json({ data: {
        email: 'private@example.test',
        displayName: 'seed',
        usernameSuggestion: 'seed.user',
        policyReviewToken: 'review-token',
        policies: {
          termsOfService: { version: 'terms-v1', url: 'https://example.test/terms' },
          privacyPolicy: { version: 'privacy-v1', url: 'https://example.test/privacy' },
          communityGuidelines: { version: 'guidelines-v1', url: 'https://example.test/guidelines' },
          supportUrl: 'https://example.test/support',
        },
      } });
    }
    return Response.json({ code: 'forbidden' }, { status: 403 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const destinations: unknown[] = [];
  for (const nextAction of ['onboarding', 'duplicate_email_recovery'] as const) {
    const credential = {
      token: `${nextAction}-token`,
      expiresAt: '2026-07-21T12:00:00Z',
      nextAction,
    };
    await activatePersistedMobileSession({
      credential,
      renewable: true,
      current: () => true,
      adopt: () => {},
      loadHome: (current) => loadMobileHomeProfile('https://api.example.test', current),
      loadOnboarding: (current) => loadMobileOnboardingProfile('https://api.example.test', current),
      online: (destination) => {
        assert.equal(destination.kind, nextAction);
        destinations.push(destination);
      },
    });
  }

  assert.deepEqual(requests.map((request) => [
    request.method,
    new URL(request.url).pathname,
    request.headers.get('authorization'),
  ]), [
    ['GET', '/v1/onboarding', 'Bearer onboarding-token'],
    ['GET', '/v1/onboarding', 'Bearer duplicate_email_recovery-token'],
  ]);
  assert.deepEqual(destinations, [
    { kind: 'onboarding', profile: {
      email: 'private@example.test', displayName: 'seed', usernameSuggestion: 'seed.user', policyReviewToken: 'review-token',
      policies: {
        termsOfService: { version: 'terms-v1', url: 'https://example.test/terms' },
        privacyPolicy: { version: 'privacy-v1', url: 'https://example.test/privacy' },
        communityGuidelines: { version: 'guidelines-v1', url: 'https://example.test/guidelines' },
        supportUrl: 'https://example.test/support',
      },
    } },
    { kind: 'duplicate_email_recovery', profile: {
      email: 'private@example.test', displayName: 'seed', usernameSuggestion: 'seed.user', policyReviewToken: 'review-token',
      policies: {
        termsOfService: { version: 'terms-v1', url: 'https://example.test/terms' },
        privacyPolicy: { version: 'privacy-v1', url: 'https://example.test/privacy' },
        communityGuidelines: { version: 'guidelines-v1', url: 'https://example.test/guidelines' },
        supportUrl: 'https://example.test/support',
      },
    } },
  ]);
});

test('the native onboarding draft combines server policy evidence with editable device defaults', () => {
  const draft = createMobileOnboardingDraft({
    email: 'private@example.test',
    displayName: 'Provider Seed',
    usernameSuggestion: 'provider.seed',
    policyReviewToken: 'review-token',
    policies: {
      termsOfService: { version: 'terms-v3', url: 'https://example.test/terms' },
      privacyPolicy: { version: 'privacy-v2', url: 'https://example.test/privacy' },
      communityGuidelines: { version: 'guidelines-v4', url: 'https://example.test/guidelines' },
      supportUrl: 'https://example.test/support',
    },
  }, { timeZone: newYorkTimeZone, firstDayOfWeek: 1 });

  assert.deepEqual(draft, {
    email: 'private@example.test', displayName: 'Provider Seed', usernameSuggestion: 'provider.seed',
    policyReviewToken: 'review-token', policies: {
      termsOfService: { version: 'terms-v3', url: 'https://example.test/terms' },
      privacyPolicy: { version: 'privacy-v2', url: 'https://example.test/privacy' },
      communityGuidelines: { version: 'guidelines-v4', url: 'https://example.test/guidelines' },
      supportUrl: 'https://example.test/support',
    },
    profileVisibility: null, timeZone: newYorkTimeZone, firstDayOfWeek: 1,
    usernameReviewed: false,
    atLeast16: false, termsAccepted: false, privacyAcknowledged: false,
    communityGuidelinesAccepted: false,
  });
});

test('native calendar weekdays are converted to the API ISO weekday convention', () => {
  assert.equal(expoCalendarWeekdayToISO(1), 7, 'Expo Sunday becomes ISO Sunday');
  assert.equal(expoCalendarWeekdayToISO(2), 1, 'Expo Monday becomes ISO Monday');
  assert.equal(expoCalendarWeekdayToISO(7), 6, 'Expo Saturday becomes ISO Saturday');
  assert.equal(expoCalendarWeekdayToISO(undefined), null, 'an unavailable device convention cannot be invented');
});

test('native onboarding captures device time zone without adding an onboarding question', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  const onboardingForm = await readFile('src/ui/onboarding-form.tsx', 'utf8');
  assert.match(app, /timeZone: deviceCalendar\?\.timeZone \?\? null/);
  assert.doesNotMatch(`${app}\n${onboardingForm}`, /i18n\.t\('onboarding\.timeZoneLabel'\)/);
  assert.match(onboardingForm, /onboarding\.timeZoneUnavailable/);
  assert.match(app, /timeZone: draft\.timeZone/);
});

test('unavailable regional calendar preferences remain blocking instead of receiving invented defaults', () => {
  const draft = createMobileOnboardingDraft({
    email: 'private@example.test', displayName: 'Seed', usernameSuggestion: 'seed.user',
    policyReviewToken: 'review-token', policies: {
      termsOfService: { version: 'terms-v1', url: 'https://example.test/terms' },
      privacyPolicy: { version: 'privacy-v1', url: 'https://example.test/privacy' },
      communityGuidelines: { version: 'guidelines-v1', url: 'https://example.test/guidelines' },
      supportUrl: 'https://example.test/support',
    },
  }, { timeZone: null, firstDayOfWeek: null });
  assert.equal(draft.timeZone, null);
  assert.equal(draft.firstDayOfWeek, null);
  const affirmed = {
    ...draft,
    profileVisibility: 'public' as const,
    usernameReviewed: true,
    atLeast16: true,
    termsAccepted: true,
    privacyAcknowledged: true,
    communityGuidelinesAccepted: true,
  };
  assert.equal(canCompleteMobileOnboarding(affirmed), false);
  assert.equal(canCompleteMobileOnboarding({ ...affirmed, timeZone: kolkataTimeZone }), false);
  assert.equal(canCompleteMobileOnboarding({ ...affirmed, timeZone: kolkataTimeZone, firstDayOfWeek: 6 }), true);
});

test('a policy refresh replaces server evidence while preserving the user-edited profile draft', () => {
  const policies = {
    termsOfService: { version: 'terms-v1', url: 'https://example.test/terms' },
    privacyPolicy: { version: 'privacy-v1', url: 'https://example.test/privacy' },
    communityGuidelines: { version: 'guidelines-v1', url: 'https://example.test/guidelines' },
    supportUrl: 'https://example.test/support',
  };
  const draft = {
    email: 'old@example.test', displayName: 'Edited Name', usernameSuggestion: 'edited.username',
    policyReviewToken: 'old-token', policies, profileVisibility: 'private' as const,
    timeZone: 'America/Chicago', firstDayOfWeek: 7, atLeast16: true,
    usernameReviewed: true,
    termsAccepted: true, privacyAcknowledged: true, communityGuidelinesAccepted: true,
  };
  const refreshed = refreshMobileOnboardingPolicy(draft, {
    email: 'current@example.test', displayName: 'Provider Name', usernameSuggestion: 'provider.username',
    policyReviewToken: 'new-token',
    policies: {
      ...policies,
      termsOfService: { version: 'terms-v2', url: 'https://example.test/terms-v2' },
    },
  });

  assert.deepEqual(refreshed, {
    ...draft,
    email: 'current@example.test',
    policyReviewToken: 'new-token',
    policies: {
      ...policies,
      termsOfService: { version: 'terms-v2', url: 'https://example.test/terms-v2' },
    },
    termsAccepted: false,
    privacyAcknowledged: false,
    communityGuidelinesAccepted: false,
  });
  assert.equal(refreshed.usernameReviewed, true);
});

test('native onboarding applies the activation field rules before completion', () => {
  for (const [displayName, valid] of [
    ['', false], ['   ', false], [' Name ', true], ['x'.repeat(100), true], ['x'.repeat(101), true],
    ['😀'.repeat(100), true], ['😀'.repeat(101), true],
  ] as const) assert.equal(validProfileDisplayName(displayName), valid, displayName);

  for (const [username, valid] of [
    ['ab', false], ['abc', true], ['a.b', true], ['a_b', true], ['1.a_2', true],
    ['.ab', true], ['abc.', true], ['_abc', true], ['abc_', true],
    ['a-b', false], ['a b', false], ['ábč', false], ['a'.repeat(64), true], ['a'.repeat(65), false],
  ] as const) assert.equal(validProfileUsername(username), valid, username);

  const complete = {
    ...createMobileOnboardingDraft({
      email: 'private@example.test', displayName: 'Valid Name', usernameSuggestion: 'valid.user',
      policyReviewToken: 'review-token', policies: {
        termsOfService: { version: 'terms-v1', url: 'https://example.test/terms' },
        privacyPolicy: { version: 'privacy-v1', url: 'https://example.test/privacy' },
        communityGuidelines: { version: 'guidelines-v1', url: 'https://example.test/guidelines' },
        supportUrl: 'https://example.test/support',
      },
    }, { timeZone: newYorkTimeZone, firstDayOfWeek: 1 }),
    profileVisibility: 'private' as const,
    usernameReviewed: true,
    atLeast16: true,
    termsAccepted: true,
    privacyAcknowledged: true,
    communityGuidelinesAccepted: true,
  };
  assert.equal(canCompleteMobileOnboarding(complete), true);
  assert.equal(canCompleteMobileOnboarding({ ...complete, displayName: ' '.repeat(3) }), false);
  assert.equal(canCompleteMobileOnboarding({ ...complete, usernameSuggestion: 'in-valid' }), false);
  assert.equal(canCompleteMobileOnboarding({ ...complete, usernameReviewed: false }), false);
});

test('the native onboarding surface lets the user review and replace the app-specific username suggestion', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  const onboardingForm = await readFile('src/ui/onboarding-form.tsx', 'utf8');
  assert.match(app, /updateOnboardingUsername/);
  assert.match(app, /replaceMobileOnboardingUsername/);
  assert.match(app, /onChangeUsername=\{updateOnboardingUsername\}/);
  assert.match(onboardingForm, /onChangeText=\{onChangeUsername\}/);
  assert.match(onboardingForm, /autoComplete="username"/);
  assert.match(onboardingForm, /value=\{profile\.usernameSuggestion\}/);
  assert.match(app, /destination\?\.kind === 'onboarding' \? <OnboardingForm/);
  assert.doesNotMatch(onboardingForm, /autoComplete="name"\s+maxLength/);
  assert.match(onboardingForm, /onboarding\.visibilityChoose/);
  assert.match(onboardingForm, /!profile\.profileVisibility/);
  assert.doesNotMatch(`${app}\n${onboardingForm}`, /onboarding\.weekStart(?:Label|Sunday|Monday)/);
  assert.doesNotMatch(app, /firstDayOfWeek: [17]/);

  const edited = replaceMobileOnboardingUsername(
    { email: 'private@example.test', displayName: 'seed', usernameSuggestion: 'provider.seed', usernameReviewed: true },
    'my.reviewed_username',
  );
  assert.deepEqual(edited, {
    email: 'private@example.test',
    displayName: 'seed',
    usernameSuggestion: 'my.reviewed_username',
    usernameReviewed: false,
  });
});

test('native onboarding revalidates its owner-scoped reviewed draft at submission', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  assert.match(app, /current && current\.kind === 'onboarding'[\s\S]*replaceMobileOnboardingUsername/);
  assert.match(app, /if \(!canCompleteMobileOnboarding\(draft\)\) return;/);
  assert.match(app, /canSubmit=\{canCompleteMobileOnboarding\(destination\.profile\) && errorKey !== 'onboarding\.usernameUnavailable'\}/);
  assert.match(app, /errorKey === 'onboarding\.usernameUnavailable'\) return;/);
  assert.match(app, /usernameReviewed=\{destination\.profile\.usernameReviewed\}/);
  assert.match(app, /reviewOnboardingUsername[\s\S]*usernameReviewed: reviewed/);
  assert.match(app, /onReviewUsername=\{reviewOnboardingUsername\}/);
  assert.match(app, /const attempt = onboardingActivations\.begin\(session\.token, ticket\)/);
  assert.match(app, /updateOnboardingUsername[\s\S]*if \(onboardingHomeRecovery \|\| onboardingActivations\.blocked\(\)\) return/);
  assert.match(app, /onboardingActivations\.release\(attempt\)/);
  assert.match(app, /sessionOperations\.invalidate\(\);\s*invalidateOnboardingActivation\(\);\s*await deregisterPushSession/);
  assert.match(app, /adopt: \(credential, canRenew\) => \{[\s\S]*?if \(onboardingActivations\.ownedBy\(ticket\) \|\| onboardingHomeRecovery\?\.sessionToken === credential\.token\)/);
  const unavailableStart = app.indexOf("outcome.kind === 'username_unavailable'");
  const unavailableEnd = app.indexOf('    } catch (cause) {', unavailableStart);
  assert.ok(unavailableStart > 0 && unavailableEnd > unavailableStart);
  const unavailableBranch = app.slice(unavailableStart, unavailableEnd);
  assert.match(unavailableBranch, /setErrorKey\('onboarding\.usernameUnavailable'\)/);
  assert.doesNotMatch(unavailableBranch, /setDestination/);
});

test('the native active Home presents the no-Path explanation and Create Path action', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  const homeView = await readFile('src/ui/home-view.tsx', 'utf8');
  assert.match(app, /totalCount:[\s\S]*ownedHomeDestination\.profile\.paths\.length/);
  assert.match(app, /<HomeView[\s\S]*onCreate=\{beginPathCreation\}/);
  assert.match(homeView, /presentation\.kind === 'empty'/);
  assert.match(homeView, /home\.empty\.explanation/);
  assert.match(homeView, /home\.createPath/);
});

test('native Path creation is controlled, retryable, and immediately trackable', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  const form = await readFile('src/ui/path-create-form.tsx', 'utf8');
  assert.match(app, /createPathSubmissionOwner/);
  assert.match(app, /\.createPath\(body, idempotencyKey\)/);
  assert.match(app, /buildPathCreateDraft\(pathName, pathGoalForm, pathVisibility\)/);
  assert.match(app, /pathCreationTarget\.current = \{[\s\S]*ownerID,[\s\S]*requestSessionToken: currentSession\.token,[\s\S]*sessionTokens: \[currentSession\.token\]/);
  assert.match(app, /discardCredential &&[\s\S]*pathCreationTarget\.current\?\.sessionTokens\.at\(-1\) === currentSession\.token/);
  assert.match(app, /retainsOwnedHome[\s\S]*pathCreationTarget\.current = rotatePathCreationTarget\(pathCreationTarget\.current, credential\.token\)/);
  assert.match(app, /setPathVisibility\(defaultPathCreationVisibility\(profileVisibility\)\)/);
  assert.match(app, /if \(!session \|\| destination\?\.kind !== 'home' \|\| pathCreationBusy\.current\) return/);
  assert.match(app, /pathCreationBusy\.current = true/);
  assert.match(app, /ownsPathCreationTarget\([\s\S]*active\.session\?\.token,[\s\S]*activeOwnerID/);
  assert.match(app, /onNameChange=\{updatePathName\}/);
  assert.match(app, /onVisibilityChange=\{updatePathVisibility\}/);
  assert.match(app, /destination\?\.kind === 'home'[\s\S]*<HomeView/);
  assert.match(app, /<PathCreateForm/);
  assert.match(form, /pathRecurrences\.map/);
  assert.match(form, /pathCreate\.intervalEnabled/);
  assert.match(form, /pathCreate\.overallEnabled/);
  assert.match(form, /keyboardType="number-pad"/);
  assert.match(app, /createdPath\.intervalGoal/);
  assert.match(app, /path\.overallTarget/);
  assert.match(app, /paths: \[\.\.\.current\.profile\.paths, createdPath\]/);
  assert.match(form, /pathCreate\.submitting/);
  assert.match(form, /pathCreate\.retry/);
  assert.match(app, /timers: \{[\s\S]*?\.\.\.current\.profile\.timers,[\s\S]*?\[createdPath\.id\]:/);
  assert.match(app, /pathCreation\.cancel\(\)/);
  assert.doesNotMatch(app, /fetch\(/);
});

test('native Home restores and controls each per-Path timer through the generated client', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  assert.match(app, /createTimerOperationOwner/);
  assert.match(app, /\.startTimer\(pathID, idempotencyKey\)/);
  assert.match(app, /\.stopTimer\(pathID, timerID, idempotencyKey\)/);
  assert.match(app, /activeTimerSeconds\(state\.timer\?\.startedAt, now\)/);
  assert.match(app, /const presentation = timerMutationPresentation\(result\.state\)/);
  assert.match(app, /applyOwnedTimerState\(ownerID, currentSession, pathID, presentation\.state\)/);
  assert.match(app, /timerMutationPresentation\(state\)\.controlMessage/);
  assert.match(app, /if \(presentation\.notice === 'subsecond'\) setTimerNoticeKey\('timer\.subsecondNotice'\)/);
  assert.match(app, /timerNoticeKey \? <Text accessibilityLiveRegion="polite">\{i18n\.t\(timerNoticeKey\)\}<\/Text>/);
  assert.doesNotMatch(app, /pathCreate\.trackingUnavailable/);
});

test('native manual private state is session-owned and rejects late responses', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  assert.match(app, /const manualOperations = createSessionOperationOwner\(\)/);
  assert.match(app, /function resetManualActivity\(\)/);
  assert.match(app, /setManualPathID\(null\);[\s\S]*setManualForm\(null\);[\s\S]*setManualNote\(''\);[\s\S]*setManualBusy\(false\);[\s\S]*setManualActivity\(null\);[\s\S]*setManualIdempotencyKey\(''\);/);
  assert.match(app, /const ticket = manualOperations\.issue\(\);[\s\S]*if \(!ticket\.current\(\) \|\| manualOwnerID\.current !== ownerID\) return;/);
  assert.match(app, /async function clearSession[\s\S]*resetManualActivity\(\);/);
  assert.match(app, /if \(manualOwnerID\.current && manualOwnerID\.current !== nextOwnerID\) resetManualActivity\(\);/);
  assert.match(app, /function closeManualActivity\(\) \{[\s\S]*if \(manualBusy\) return;[\s\S]*if \(!manualDraftDirty\) \{[\s\S]*resetManualActivity\(\);[\s\S]*presentNativeDestructiveConfirmation\(\{/);
  const failureHandlerStart = app.indexOf('async function handleSessionFailure(');
  const failureHandler = app.slice(failureHandlerStart, app.indexOf('useEffect(() =>', failureHandlerStart));
  assert.ok(failureHandler.indexOf('resetManualActivity()') >= 0);
  assert.ok(failureHandler.indexOf('resetManualActivity()') < failureHandler.indexOf('await applyMobileSessionFailure'));
});

test('session disposal clears timer presentation state before a later identity can reuse the shell', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  assert.match(app, /function resetTimerPresentation\(\)[\s\S]*timerOperations\.cancel\(\);[\s\S]*setTimerBusy\(\{\}\);[\s\S]*setTimerErrorKeys\(\{\}\);[\s\S]*setTimerNoticeKey\(null\);/);
  assert.match(app, /async function clearSession[\s\S]*resetTimerPresentation\(\);/);
  assert.match(app, /if \(classifySessionFailure\(failure\)\.discardCredential\) \{[\s\S]*sessionOperations\.invalidate\(\);[\s\S]*resetTimerPresentation\(\);/);
});

test('native Path details own activity history, inspection, and participant editing', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  const historyView = await readFile('src/ui/activity-history-view.tsx', 'utf8');
  const detailView = await readFile('src/ui/activity-detail-view.tsx', 'utf8');
  assert.match(app, /name=\{path\.name\}[\s\S]*onOpen=\{\(\) => void openPathDetail\(path\.id\)\}/);
  assert.match(app, /label:\s*i18n\.t\('pathDetails\.openHistory'\)[\s\S]*onPress:\s*\(\) => void openActivityHistory\(selectedPath\.id\)/);
  assert.match(app, /pathname: '\/path\/\[pathID\]\/history'/);
  assert.match(historyView, /groupActivitiesByOccurrenceDay\(activities\)\.map/);
  assert.match(
    historyView,
    /i18n\.date\(retainedCalendarDayDate\(day\.localDate\), \{[\s\S]*timeZone: retainedCalendarDayTimeZone/,
  );
  assert.match(app, /\.activities\(pathID, cursor, participantID\)/);
  assert.match(app, /\.activity\(pathID, activityID\)/);
  assert.match(app, /\.activityRevisions\(pathID, activityID, cursor\)/);
  assert.match(app, /setActivityHistory\(newestActivitiesFirst\(history\.items\)\)/);
  assert.match(app, /activityBelongsToProfile\(selectedActivity, destination\.profile\.id\)/);
  assert.match(historyView, /activityWasEdited\(detail\)/);
  assert.match(historyView, /pathDetails\.participant/);
  assert.match(historyView, /pathDetails\.edited/);
  assert.match(detailView, /activityInstant\(revision\.startedAt, revision\.occurrenceTimeZone\)/);
  assert.match(detailView, /priorNoteForProfile\(revision, profileID\)/);
  const pathLanding = app.slice(app.indexOf('function openPathDetail('), app.indexOf('async function openActivityHistory('));
  assert.doesNotMatch(pathLanding, /\.activities\(/);
  assert.match(app, /const seed = activityEditSeed\(selectedActivity, defaults\)/);
  assert.match(app, /await refreshPathDetail\(pathID, result\.activity\.id, currentSession, ownerID\)/);
  assert.match(app, /function resetPathDetail\(\)[\s\S]*setActivityHistory\(\[\]\);[\s\S]*setSelectedActivity\(null\);[\s\S]*setActivityRevisions\(\[\]\);/);
  const homeCards = app.slice(app.indexOf('const renderHomePath'), app.indexOf('{selectedPath ? <NativeRouteSource'));
  assert.doesNotMatch(homeCards, /activity\.add/);
});

test('native activity and revision histories paginate without replacing retained rows', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  const historyView = await readFile('src/ui/activity-history-view.tsx', 'utf8');
  const detailView = await readFile('src/ui/activity-detail-view.tsx', 'utf8');
  assert.match(app, /cursor \? appendUniqueActivities\(current, page\.items\) : newestActivitiesFirst\(page\.items\)/);
  assert.match(app, /setActivityRevisions\(\(current\) => appendUniqueRevisions\(current, page\.items\)\)/);
  assert.match(app, /setActivityHistoryCursor\(page\.nextCursor\)/);
  assert.match(app, /setActivityRevisionCursor\(page\.nextCursor\)/);
  assert.match(app, /nextCursor: result\.data\.meta\.nextCursor \?\? null/);
  assert.match(historyView, /common\.loadMore/);
  assert.match(historyView, /common\.retry/);
  assert.match(detailView, /common\.loadMore/);
  assert.match(detailView, /common\.retry/);
  assert.match(app, /setActivityHistoryErrorKey\(localizedFailure/);
  assert.match(app, /setActivityRevisionErrorKey\(localizedFailure/);
  assert.match(app, /ownsActivePathDetail\(pathID, activityID\)/);
  assert.match(app, /ownsPathDetailTarget\(/);
  const historyFailure = app.slice(app.indexOf('async function openActivityHistory('), app.indexOf('async function inspectActivity('));
  assert.doesNotMatch(historyFailure.slice(historyFailure.indexOf('catch (cause)')), /setActivityHistory\(/);
  const revisionFailure = app.slice(app.indexOf('async function loadMoreActivityRevisions('), app.indexOf('async function refreshPathDetail('));
  assert.doesNotMatch(revisionFailure.slice(revisionFailure.indexOf('catch (cause)')), /setActivityRevisions\(/);
});
