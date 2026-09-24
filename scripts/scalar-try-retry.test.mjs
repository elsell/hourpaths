import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

import {
  ensureTryClientOpen,
  reopenTryClient,
  waitForAuthorizedTryRequest,
  waitForScalarCredential,
} from './scalar-try-retry.mjs'

class ControlledTryPort {
  constructor(outcomes) {
    this.outcomes = [...outcomes]
    this.events = []
    this.time = 0
    this.pendingWaiter = undefined
    this.clientOpen = false
  }

  now = () => this.time
  ensureOpen = async (_buttonName, timeout) => {
    this.events.push(['ensure-open', timeout])
    if (!this.clientOpen) {
      this.events.push(['open-client'])
      this.clientOpen = true
    }
    return true
  }
  waitForResponse = (_pathname, timeout) => {
    assert.equal(this.pendingWaiter, undefined, 'only one controlled response waiter may be installed')
    this.events.push(['wait', timeout])
    return new Promise((resolve) => { this.pendingWaiter = { resolve, timeout } })
  }
  send = async (timeout) => {
    assert.equal(this.clientOpen, true, 'the Scalar client must remain open before send')
    assert.ok(this.pendingWaiter, 'response waiter must be installed before send')
    this.events.push(['send', timeout])
    const waiter = this.pendingWaiter
    this.pendingWaiter = undefined
    const outcome = this.outcomes.shift()
    if (outcome === undefined) this.time += waiter.timeout
    waiter.resolve(outcome)
  }
  authorization = async (response) => response.authorization
  status = (response) => response.status
  bodyText = async (response) => response.body ?? ''
  parse = async (response) => { this.events.push(['parse']); return response.json }
  pause = async (milliseconds) => { this.events.push(['pause', milliseconds]); this.time += milliseconds }
  reopen = async () => {
    this.events.push(['reopen'])
    this.clientOpen = false
    return true
  }
}

const options = {
  buttonName: /Test Request/,
  pathname: '/v1/me',
  totalTimeoutMs: 10_000,
  responseTimeoutMs: 100,
  actionTimeoutMs: 200,
  retryPauseMs: 25,
  maxAttempts: 5,
}

test('credential readiness requires the exact exchanged token to remain stable before returning', async () => {
  const checks = [false, true, true]
  const events = []
  let time = 0
  const port = {
    now: () => time,
    credentialMatches: async (token) => {
      events.push(['credential', token])
      return checks.shift() ?? false
    },
    pause: async (milliseconds) => { events.push(['pause', milliseconds]); time += milliseconds },
  }

  await waitForScalarCredential(port, 'application-session', 100, 25, 25)
  assert.deepEqual(events, [
    ['credential', 'application-session'], ['pause', 25],
    ['credential', 'application-session'], ['pause', 25],
    ['credential', 'application-session'],
  ])
  assert.equal(time, 50)
})

test('a transient credential mismatch resets the bounded stability interval', async () => {
  const checks = [true, false, true, true, true]
  const events = []
  let time = 0
  const port = {
    now: () => time,
    credentialMatches: async () => {
      events.push(['credential', time])
      return checks.shift() ?? false
    },
    pause: async (milliseconds) => { events.push(['pause', milliseconds]); time += milliseconds },
  }

  await waitForScalarCredential(port, 'application-session', 125, 25, 50)
  assert.deepEqual(events, [
    ['credential', 0], ['pause', 25],
    ['credential', 25], ['pause', 25],
    ['credential', 50], ['pause', 25],
    ['credential', 75], ['pause', 25],
    ['credential', 100],
  ])
  assert.equal(time, 100)
})

test('first equality never bypasses the production Scalar settling interval', async () => {
  const pauses = []
  let time = 0
  const port = {
    now: () => time,
    credentialMatches: async () => true,
    pause: async (milliseconds) => { pauses.push(milliseconds); time += milliseconds },
  }

  await waitForScalarCredential(port, 'application-session', 1_000, 100)
  assert.deepEqual(pauses, [100, 100, 100, 100, 100])
  assert.equal(time, 500)
})

test('credential readiness fails closed without overspending its bound', async () => {
  const pauses = []
  let time = 0
  const port = {
    now: () => time,
    credentialMatches: async () => false,
    pause: async (milliseconds) => { pauses.push(milliseconds); time += milliseconds },
  }

  await assert.rejects(
    waitForScalarCredential(port, 'application-session', 60, 25),
    /did not apply the exchanged OIDC credential/,
  )
  assert.deepEqual(pauses, [25, 25, 10])
  assert.equal(time, 60)
})

class ControlledTryUI {
  constructor(states) {
    this.states = [...states]
    this.events = []
  }

  now = () => 0
  current() { return this.states[0] ?? { send: false, close: false } }
  sendVisible = async () => this.current().send
  closeVisible = async () => this.current().close
  waitForSend = async (timeout) => {
    this.events.push(['wait-for-send', timeout])
    if (this.states.length > 1) this.states.shift()
    return this.current().send
  }
  openOperation = async (_buttonName, timeout) => {
    this.events.push(['open-operation', timeout])
    if (this.states.length > 1) this.states.shift()
    return true
  }
  closeClient = async (timeout) => {
    this.events.push(['close-client', timeout])
    if (this.states.length > 1) this.states.shift()
    return true
  }
  waitForClosed = async (timeout) => {
    this.events.push(['wait-for-closed', timeout])
    return !this.current().send && !this.current().close
  }
}

test('UI convergence recomputes one monotonic elapsed budget after every await', async () => {
  const events = []
  let time = 0
  const ui = {
    now: () => time,
    sendVisible: async () => { time += 10; return false },
    closeVisible: async () => { time += 10; return false },
    waitForSend: async (timeout) => {
      events.push(['wait-for-send', timeout])
      time += timeout
      return false
    },
    openOperation: async (_buttonName, timeout) => {
      events.push(['open-operation', timeout])
      time += 40
      return true
    },
  }
  assert.equal(await ensureTryClientOpen(ui, /Test Request/, 200), false)
  assert.equal(time, 200)
  assert.deepEqual(events, [
    ['wait-for-send', 100],
    ['open-operation', 60],
    ['wait-for-send', 20],
  ])
})

test('an open but settling client is never toggled and may retry convergence', async () => {
  const ui = new ControlledTryUI([{ send: false, close: true }])
  assert.equal(await ensureTryClientOpen(ui, /Test Request/, 200), false)
  assert.deepEqual(ui.events, [['wait-for-send', 200]])
})

test('a transient both-hidden snapshot converges without toggling the operation', async () => {
  const ui = new ControlledTryUI([
    { send: false, close: false },
    { send: true, close: true },
  ])
  assert.equal(await ensureTryClientOpen(ui, /Test Request/, 200), true)
  assert.deepEqual(ui.events, [['wait-for-send', 100]])
})

test('a stable closed client opens once and waits for Send Request', async () => {
  const ui = new ControlledTryUI([
    { send: false, close: false },
    { send: false, close: false },
    { send: true, close: true },
  ])
  assert.equal(await ensureTryClientOpen(ui, /Test Request/, 200), true)
  assert.deepEqual(ui.events, [
    ['wait-for-send', 100],
    ['open-operation', 200],
    ['wait-for-send', 200],
  ])
})

test('credential refresh explicitly closes and reopens a sticky Try client within one deadline', async () => {
  let time = 0
  const events = []
  const ui = {
    now: () => time,
    sendVisible: async () => true,
    closeVisible: async () => true,
    closeClient: async (timeout) => { events.push(['close-client', timeout]); time += 25; return true },
    waitForClosed: async (timeout) => { events.push(['wait-for-closed', timeout]); time += 25; return true },
    waitForSend: async (timeout) => { events.push(['wait-for-send', timeout]); time += 25; return true },
    openOperation: async (_buttonName, timeout) => { events.push(['open-operation', timeout]); time += 25; return true },
  }
  assert.equal(await reopenTryClient(ui, /Test Request/, 200), true)
  assert.deepEqual(events, [
    ['close-client', 200],
    ['wait-for-closed', 175],
    ['open-operation', 150],
    ['wait-for-send', 125],
  ])
  assert.equal(time, 100)
})

test('the controlled response waiter remains pending until send', async () => {
  const port = new ControlledTryPort([{ authorization: 'Bearer session', status: 200, json: {} }])
  let settled = false
  await port.ensureOpen(/Test Request/, 100)
  const waiter = port.waitForResponse('/v1/me', 100).then((response) => { settled = true; return response })
  await Promise.resolve()
  assert.equal(settled, false)
  await port.send(100)
  assert.equal((await waiter).status, 200)
})

test('mutation awaiting the response waiter before send deterministically stalls', async () => {
  const source = await readFile(new URL('./scalar-try-retry.mjs', import.meta.url), 'utf8')
  const mutantSource = source.replace(
    'const responsePromise = port.waitForResponse(',
    'const responsePromise = await port.waitForResponse(',
  )
  assert.notEqual(mutantSource, source, 'mutation target must remain present')
  const mutant = await import(`data:text/javascript;base64,${Buffer.from(mutantSource).toString('base64')}`)
  const port = new ControlledTryPort([{ authorization: 'Bearer session', status: 200, json: {} }])
  let settled = false
  void mutant.waitForAuthorizedTryRequest(port, options).then(() => { settled = true }, () => { settled = true })
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
  assert.deepEqual(port.events.map(([event]) => event), ['ensure-open', 'open-client', 'wait'])
  assert.equal(settled, false)
})

test('registers the response waiter before send and recovers from UI settling', async () => {
  const port = new ControlledTryPort([
    undefined,
    { authorization: undefined, status: 401 },
    { authorization: 'Bearer application-session', status: 200, json: { data: { email: 'developer@example.com' } } },
  ])
  const result = await waitForAuthorizedTryRequest(port, options)
  assert.deepEqual(result, { data: { email: 'developer@example.com' } })
  assert.deepEqual(port.events.map(([event]) => event), [
    'ensure-open', 'open-client', 'wait', 'send', 'pause',
    'ensure-open', 'wait', 'send', 'pause', 'reopen',
    'ensure-open', 'open-client', 'wait', 'send', 'parse',
  ])
})

test('a sticky unauthenticated Try client becomes authorized only after explicit reopen', async () => {
  const port = new ControlledTryPort([
    { authorization: undefined, status: 401 },
    { authorization: 'Bearer application-session', status: 200, json: { data: { email: 'developer@example.com' } } },
  ])
  port.reopen = async () => {
    port.events.push(['reopen'])
    port.clientOpen = false
    return true
  }
  const result = await waitForAuthorizedTryRequest(port, options)
  assert.deepEqual(result, { data: { email: 'developer@example.com' } })
  assert.deepEqual(port.events.map(([event]) => event), [
    'ensure-open', 'open-client', 'wait', 'send', 'pause', 'reopen',
    'ensure-open', 'open-client', 'wait', 'send', 'parse',
  ])
})

test('the production retry budget is deadline-driven beyond twenty rapid unauthorized responses', async () => {
  const unauthorized = Array.from({ length: 20 }, () => ({ authorization: undefined, status: 401 }))
  const port = new ControlledTryPort([
    ...unauthorized,
    { authorization: 'Bearer application-session', status: 200, json: { data: { email: 'developer@example.com' } } },
  ])
  const result = await waitForAuthorizedTryRequest(port, {
    buttonName: /Test Request/,
    pathname: '/v1/me',
    totalTimeoutMs: 45_000,
    responseTimeoutMs: 100,
    actionTimeoutMs: 200,
    retryPauseMs: 250,
  })
  assert.deepEqual(result, { data: { email: 'developer@example.com' } })
  assert.equal(port.events.filter(([event]) => event === 'send').length, 21)
})

test('UI convergence failures retry without installing a response waiter', async () => {
  const port = new ControlledTryPort([
    { authorization: 'Bearer application-session', status: 200, json: { data: {} } },
  ])
  const originalEnsureOpen = port.ensureOpen
  let convergenceAttempt = 0
  port.ensureOpen = async (...arguments_) => {
    convergenceAttempt += 1
    if (convergenceAttempt === 1) {
      port.events.push(['ensure-open-unsettled'])
      return false
    }
    return originalEnsureOpen(...arguments_)
  }
  await waitForAuthorizedTryRequest(port, options)
  assert.deepEqual(port.events.map(([event]) => event), [
    'ensure-open-unsettled', 'pause',
    'ensure-open', 'open-client', 'wait', 'send', 'parse',
  ])
})

test('an exhausted UI convergence attempt cannot overspend the outer deadline', async () => {
  const port = new ControlledTryPort([])
  port.ensureOpen = async (_buttonName, timeout) => {
    port.events.push(['ensure-open-unsettled', timeout])
    port.time += timeout
    return false
  }
  await assert.rejects(
    waitForAuthorizedTryRequest(port, { ...options, totalTimeoutMs: 150, actionTimeoutMs: 200 }),
    /omitted the OIDC bearer token/,
  )
  assert.equal(port.time, 150)
  assert.deepEqual(port.events.map(([event]) => event), ['ensure-open-unsettled'])
})

test('permanent response timeouts exhaust the bound and fail closed', async () => {
  const port = new ControlledTryPort([undefined, undefined, undefined])
  await assert.rejects(
    waitForAuthorizedTryRequest(port, { ...options, maxAttempts: 3 }),
    /omitted the OIDC bearer token for \/v1\/me after the bounded credential-application wait/,
  )
  assert.equal(port.events.filter(([event]) => event === 'send').length, 3)
  assert.equal(port.events.filter(([event]) => event === 'open-client').length, 1)
  assert.equal(port.time, 375)
})

test('an authorized non-200 response fails immediately without cleanup retry', async () => {
  const port = new ControlledTryPort([
    { authorization: 'Bearer application-session', status: 503, body: 'unavailable' },
    { authorization: 'Bearer application-session', status: 200, json: {} },
  ])
  await assert.rejects(waitForAuthorizedTryRequest(port, options), /returned 503: unavailable/)
  assert.deepEqual(port.events.map(([event]) => event), ['ensure-open', 'open-client', 'wait', 'send'])
})
