import { chromium } from 'playwright'
import { navigateToDexLogin } from './web-browser-navigation.mjs'

const webBaseURL = process.env.WEB_ACCEPTANCE_BASE_URL ?? 'http://localhost:5173'
const apiBaseURL = process.env.WEB_ACCEPTANCE_API_URL ?? 'http://localhost:8080'
const dexOrigin = process.env.WEB_ACCEPTANCE_DEX_ORIGIN ?? 'http://localhost:5556'
const email = process.env.WEB_ACCEPTANCE_EMAIL ?? 'developer@example.com'
const password = process.env.WEB_ACCEPTANCE_PASSWORD ?? 'password'
const invitationID = process.env.WEB_ACCEPTANCE_INVITATION_ID
const invitationPathName = process.env.WEB_ACCEPTANCE_INVITATION_PATH_NAME

if (Boolean(invitationID) !== Boolean(invitationPathName)) {
  throw new Error('WEB_ACCEPTANCE_INVITATION_ID and WEB_ACCEPTANCE_INVITATION_PATH_NAME must be provided together')
}

async function openLocalizedDocument(page) {
  const documentResponse = await page.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  if (!documentResponse?.ok()) {
    throw new Error(`localized web document returned ${documentResponse?.status() ?? 'no response'}`)
  }

  const language = await page.locator('html').getAttribute('lang')
  if (language !== 'es') {
    throw new Error(`localized web document used lang=${JSON.stringify(language)} instead of es`)
  }
  const vary = documentResponse.headers().vary?.split(',').map((value) => value.trim().toLowerCase()) ?? []
  if (!vary.includes('accept-language')) {
    throw new Error(`localized web document omitted Vary: Accept-Language: ${JSON.stringify(vary)}`)
  }

  await page.getByText('Tu aplicación generada está lista.', { exact: true }).waitFor()
}

async function completeDexLogin(page, expectedResponse) {
  const signIn = page.getByRole('button', { name: 'Iniciar sesión', exact: true })
  await navigateToDexLogin({
    now: () => Date.now(),
    waitForLoginURL: async (predicate, timeout) => {
      await page.waitForURL(predicate, { timeout, waitUntil: 'domcontentloaded' })
      return new URL(page.url())
    },
    clickSignIn: (timeout) => signIn.click({ timeout }),
    waitForLoginForm: (timeout) => Promise.all([
      page.locator('input[name=login]').waitFor({ state: 'visible', timeout }),
      page.locator('input[name=password]').waitFor({ state: 'visible', timeout }),
    ]),
  }, {
    dexOrigin,
    timeoutMs: 15_000,
  })
  await page.locator('input[name=login]').fill(email)
  await page.locator('input[name=password]').fill(password)

  const responsePromise = page.waitForResponse(expectedResponse)
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  return responsePromise
}

async function expectFocused(locator, message) {
  await locator.waitFor({ state: 'visible' })
  if (!await locator.evaluate((element) => element === document.activeElement)) {
    throw new Error(message)
  }
}

async function acceptInvitationWithVisibilityWarning(page) {
  const invitationPath = `/v1/path-invitations/${invitationID}/accept`
  const acceptanceRequests = []
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === invitationPath && request.method() === 'POST') {
      acceptanceRequests.push(request)
    }
  })

  const invitationsResponse = await completeDexLogin(
    page,
    (response) =>
      new URL(response.url()).pathname === '/v1/path-invitations' &&
      response.request().method() === 'GET',
  )
  if (invitationsResponse.status() !== 200) {
    throw new Error(`authenticated invitation Home request returned ${invitationsResponse.status()}`)
  }
  await page.getByRole('heading', { name: 'Inicio', exact: true }).waitFor()
  const invitation = page.locator(`[id="${invitationID}"]`)
  await invitation.waitFor({ state: 'visible' })
  await invitation.getByText(invitationPathName, { exact: false }).waitFor()

  const acceptInvitation = invitation.getByRole('button', { name: 'Aceptar invitación', exact: true })
  await acceptInvitation.click()
  const warning = invitation.getByRole('alertdialog')
  await warning.getByRole('heading', { name: 'Revisa la visibilidad de esta ruta', exact: true }).waitFor()
  await warning.getByText('Visibilidad actual de la ruta: seguidores', { exact: true }).waitFor()
  const confirmAcceptance = warning.getByRole('button', { name: 'Confirmar aceptación', exact: true })
  await expectFocused(confirmAcceptance, 'visibility warning did not move focus to confirmation')
  if (acceptanceRequests.length !== 0) {
    throw new Error('opening the visibility warning issued an acceptance mutation')
  }

  await warning.getByRole('button', { name: 'Cancelar', exact: true }).click()
  await expectFocused(acceptInvitation, 'cancelling the visibility warning did not restore focus to acceptance')
  if (await invitation.count() !== 1) {
    throw new Error('invitation disappeared after cancelling its visibility warning')
  }
  if (acceptanceRequests.length !== 0) {
    throw new Error('cancelling the visibility warning issued an acceptance mutation')
  }

  await acceptInvitation.click()
  await warning.waitFor({ state: 'visible' })
  await expectFocused(confirmAcceptance, 'reopened visibility warning did not focus confirmation')
  const acceptedResponsePromise = page.waitForResponse(
    (response) => new URL(response.url()).pathname === invitationPath &&
      response.request().method() === 'POST',
  )
  await confirmAcceptance.click()
  const acceptedResponse = await acceptedResponsePromise
  if (acceptedResponse.status() !== 200) {
    throw new Error(`confirmed browser invitation acceptance returned ${acceptedResponse.status()}`)
  }
  const acknowledgement = acceptedResponse.request().postDataJSON()
  if (acknowledgement?.visibilityWarningAcknowledgement?.pathVisibility !== 'followers') {
    throw new Error(`browser acceptance omitted the reviewed visibility acknowledgement: ${JSON.stringify(acknowledgement)}`)
  }
  if (acceptanceRequests.length !== 1) {
    throw new Error(`browser issued ${acceptanceRequests.length} invitation acceptance mutations instead of one`)
  }

  await invitation.waitFor({ state: 'detached' })
  if (await invitation.count() !== 0) throw new Error('accepted invitation remained on Home')
  await page.getByRole('button', { name: invitationPathName, exact: true }).waitFor()
  console.log('browser Dex invitation visibility warning, cancel, confirmation, and Path projection acceptance passed')
}

async function reviewOnboarding(page) {
  const response = await completeDexLogin(
    page,
    (response) => response.url() === `${apiBaseURL}/v1/onboarding` &&
      response.request().method() === 'GET',
  )
  if (response.status() !== 200) {
    throw new Error(`authenticated web onboarding returned ${response.status()}: ${await response.text()}`)
  }
  await page.getByRole('heading', { name: 'Configura tu perfil', exact: true }).waitFor()
  const privateEmail = page.getByRole('textbox', { name: 'Correo del proveedor', exact: true })
  if (await privateEmail.inputValue() !== email) {
    throw new Error('web onboarding did not preserve the private provider email seed')
  }
  const displayName = page.getByRole('textbox', { name: 'Nombre visible', exact: true })
  if (await displayName.inputValue() !== 'developer') {
    throw new Error('web onboarding did not preserve the provider display-name seed')
  }
  const username = page.getByRole('textbox', { name: 'Nombre de usuario', exact: true })
  if (await username.inputValue() !== 'developer') {
    throw new Error('web onboarding did not present the available app-specific username suggestion')
  }
  await username.fill('reviewed.user')
  if (await username.inputValue() !== 'reviewed.user') {
    throw new Error('web onboarding did not retain the reviewed username replacement')
  }
  if (await page.locator('html').getAttribute('lang') !== 'es') {
    throw new Error('onboarding web document lost the negotiated Spanish language')
  }

  console.log('localized Dex web sign-in and onboarding acceptance passed')
}

const browser = await chromium.launch({ headless: true })
try {
  const context = await browser.newContext({ locale: 'es-ES' })
  const page = await context.newPage()
  await openLocalizedDocument(page)
  if (invitationID) await acceptInvitationWithVisibilityWarning(page)
  else await reviewOnboarding(page)
} finally {
  await browser.close()
}
