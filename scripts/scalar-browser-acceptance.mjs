import { chromium, errors } from 'playwright'
import {
  ensureTryClientOpen,
  reopenTryClient,
  waitForAuthorizedTryRequest,
  waitForScalarCredential,
} from './scalar-try-retry.mjs'

const baseURL = process.env.SCALAR_ACCEPTANCE_BASE_URL ?? 'http://localhost:8080'
const email = process.env.SCALAR_ACCEPTANCE_EMAIL ?? 'developer@example.com'
const password = process.env.SCALAR_ACCEPTANCE_PASSWORD ?? 'password'

const browser = await chromium.launch({ headless: true })
try {
  const page = await browser.newPage()
  await page.goto(`${baseURL}/docs`, { waitUntil: 'domcontentloaded' })
  await page.getByRole('button', { name: /Authorize/ }).waitFor()
  const authorizationRequestPromise = page.context().waitForEvent('request', {
    predicate: (request) => new URL(request.url()).pathname.endsWith('/dex/auth'),
  })
  const popupPromise = page.waitForEvent('popup')
  await page.getByRole('button', { name: /Authorize/ }).click()
  const authorizationURL = new URL((await authorizationRequestPromise).url())
  if (authorizationURL.searchParams.get('code_challenge_method') !== 'S256' || !authorizationURL.searchParams.get('code_challenge')) {
    throw new Error(`Scalar did not initiate S256 PKCE: ${authorizationURL}`)
  }
  const popup = await popupPromise
  await popup.locator('input[name=login]').fill(email)
  await popup.locator('input[name=password]').fill(password)
  const tokenRequestPromise = page.waitForRequest((request) => request.url() === `${baseURL}/oidc/token`)
  const tokenResponsePromise = page.waitForResponse((response) => response.url() === `${baseURL}/oidc/token`)
  await popup.getByRole('button', { name: 'Login' }).click()
  const tokenForm = new URLSearchParams((await tokenRequestPromise).postData() ?? '')
  if (tokenForm.get('grant_type') !== 'authorization_code' || !tokenForm.get('code_verifier')) {
    throw new Error(`Scalar omitted the PKCE verifier: ${tokenForm}`)
  }
  const tokenResponse = await tokenResponsePromise
  if (tokenResponse.status() !== 200) {
    throw new Error(`Scalar token exchange returned ${tokenResponse.status()}: ${await tokenResponse.text()}`)
  }
  const tokenPayload = await tokenResponse.json()
  if (typeof tokenPayload.access_token !== 'string' || tokenPayload.access_token.length === 0) {
    throw new Error('Scalar token exchange omitted the application access token')
  }
  for (let attempt = 0; attempt < 50 && !popup.isClosed(); attempt += 1) {
    await page.waitForTimeout(100)
  }
  if (!popup.isClosed()) {
    throw new Error('Scalar authorization popup did not close after token exchange')
  }
  await waitForScalarCredential({
    now: () => Date.now(),
    credentialMatches: (expectedToken) => page.locator('input.scalar-password-input').evaluateAll(
      (inputs, token) => inputs.some((input) => input.value === token),
      expectedToken,
    ),
    pause: (milliseconds) => page.waitForTimeout(milliseconds),
  }, tokenPayload.access_token, 5_000)
  const sendRequest = page.getByRole('button', { name: /Send Request/ })
  const closeClient = page.getByRole('button', { name: 'Close Client' })
  const tryUI = {
    now: () => Date.now(),
    sendVisible: () => sendRequest.isVisible(),
    closeVisible: () => closeClient.isVisible(),
    waitForSend: (timeout) => sendRequest.waitFor({ state: 'visible', timeout }).then(
      () => true,
      (cause) => {
        if (cause instanceof errors.TimeoutError) return false
        throw cause
      },
    ),
    openOperation: (buttonName, timeout) => page.getByRole('button', { name: buttonName }).click({ timeout }).then(
      () => true,
      (cause) => {
        if (cause instanceof errors.TimeoutError) return false
        throw cause
      },
    ),
    closeClient: (timeout) => closeClient.click({ timeout }).then(
      () => true,
      (cause) => {
        if (cause instanceof errors.TimeoutError) return false
        throw cause
      },
    ),
    waitForClosed: (timeout) => Promise.all([
      sendRequest.waitFor({ state: 'hidden', timeout }),
      closeClient.waitFor({ state: 'hidden', timeout }),
    ]).then(
      () => true,
      (cause) => {
        if (cause instanceof errors.TimeoutError) return false
        throw cause
      },
    ),
  }
  const tryPort = {
    now: () => Date.now(),
    ensureOpen: (buttonName, timeout) => ensureTryClientOpen(tryUI, buttonName, timeout),
    reopen: (buttonName, timeout) => reopenTryClient(tryUI, buttonName, timeout),
    waitForResponse: (pathname, timeout) => page.waitForResponse(
      (response) => response.url().startsWith(`${baseURL}${pathname}`) && response.request().method() === 'GET',
      { timeout },
    ).catch((cause) => {
      if (cause instanceof errors.TimeoutError) return undefined
      throw cause
    }),
    send: (timeout) => page.getByRole('button', { name: /Send Request/ }).click({ timeout }),
    authorization: (response) => response.request().headerValue('authorization'),
    status: (response) => response.status(),
    bodyText: (response) => response.text(),
    parse: (response) => response.json(),
    pause: (milliseconds) => page.waitForTimeout(milliseconds),
  }

  const onboarding = await waitForAuthorizedTryRequest(tryPort, {
    buttonName: /Test Request.*get \/v1\/onboarding\)/i,
    pathname: '/v1/onboarding',
  })
  if (onboarding?.data?.email !== email || onboarding?.data?.displayName !== 'developer' || onboarding?.data?.usernameSuggestion !== 'developer') {
    throw new Error(`Scalar /v1/onboarding returned the wrong private seeds: ${JSON.stringify(onboarding)}`)
  }
  console.log('Scalar browser Dex OIDC and provisional onboarding Try It acceptance passed')
} finally {
  await browser.close()
}
