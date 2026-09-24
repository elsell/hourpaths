import assert from 'node:assert/strict'
import test from 'node:test'

import { isDexLoginURL, navigateToDexLogin } from './web-browser-navigation.mjs'

test('Dex login URL matching accepts the actual local connector state only', () => {
  assert.equal(isDexLoginURL(new URL('http://localhost:5556/dex/auth/local/login?req=abc'), 'http://localhost:5556'), true)
  assert.equal(isDexLoginURL(new URL('http://localhost:5556/dex/auth?req=abc'), 'http://localhost:5556'), false)
  assert.equal(isDexLoginURL(new URL('http://attacker.invalid/dex/auth/local/login'), 'http://localhost:5556'), false)
})

test('the navigation waiter is installed before sign-in and the login form is ready', async () => {
  const events = []
  let time = 0
  let resolveNavigation
  const port = {
    now: () => time,
    waitForLoginURL: (predicate, timeout) => {
      events.push(['wait-url', timeout])
      return new Promise((resolve) => { resolveNavigation = (url) => {
        assert.equal(predicate(new URL(url)), true)
        resolve(new URL(url))
      } })
    },
    clickSignIn: async (timeout) => {
      assert.ok(resolveNavigation, 'navigation waiter must exist before click')
      events.push(['click', timeout])
      resolveNavigation('http://localhost:5556/dex/auth/local/login?req=abc')
    },
    waitForLoginForm: async (timeout) => { events.push(['wait-form', timeout]) },
  }
  await navigateToDexLogin(port, { dexOrigin: 'http://localhost:5556', timeoutMs: 1_000 })
  assert.deepEqual(events, [['wait-url', 1_000], ['click', 1_000], ['wait-form', 1_000]])
})

test('login-form readiness receives only the monotonic remaining navigation budget', async () => {
  let time = 0
  const events = []
  const port = {
    now: () => time,
    waitForLoginURL: async () => new URL('http://localhost:5556/dex/auth/local/login?req=abc'),
    clickSignIn: async (timeout) => { events.push(['click', timeout]); time += 400 },
    waitForLoginForm: async (timeout) => { events.push(['wait-form', timeout]); time += timeout },
  }
  await navigateToDexLogin(port, { dexOrigin: 'http://localhost:5556', timeoutMs: 1_000 })
  assert.equal(time, 1_000)
  assert.deepEqual(events, [['click', 1_000], ['wait-form', 600]])
})
