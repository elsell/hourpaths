import assert from 'node:assert/strict'
import { chromium } from 'playwright'

const webBaseURL = process.env.WEB_ACCEPTANCE_BASE_URL ?? 'http://localhost:5173'
const ownerToken = process.env.WEB_ACCEPTANCE_APPLICATION_TOKEN
const participantToken = process.env.WEB_ACCEPTANCE_PARTICIPANT_TOKEN
const pathID = process.env.WEB_ACCEPTANCE_PATH_ID
const pathName = process.env.WEB_ACCEPTANCE_PATH_NAME ?? 'Guitar practice'

if (!ownerToken) throw new Error('WEB_ACCEPTANCE_APPLICATION_TOKEN is required')
if (!participantToken) throw new Error('WEB_ACCEPTANCE_PARTICIPANT_TOKEN is required')
if (!pathID) throw new Error('WEB_ACCEPTANCE_PATH_ID is required')

const visibilityPathname = `/v1/paths/${pathID}/visibility`

function isVisibilityUpdate(request) {
  return request.method() === 'PUT' && new URL(request.url()).pathname === visibilityPathname
}

async function applicationContext(browser, token) {
  const context = await browser.newContext({ locale: 'en-US' })
  await context.addInitScript(({ applicationToken }) => {
    window.sessionStorage.setItem('hourpaths_application_session', JSON.stringify({
      token: applicationToken,
      expiresAt: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      nextAction: 'home',
    }))
  }, { applicationToken: token })
  return context
}

async function openPath(page) {
  await page.getByRole('heading', { name: 'Home', exact: true }).waitFor()
  const card = page.locator('li').filter({
    has: page.getByRole('button', { name: pathName, exact: true }),
  })
  assert.equal(await card.count(), 1, 'visibility acceptance Path was missing from Home')
  await card.getByRole('button', { name: pathName, exact: true }).click()
  await page.getByRole('heading', { name: pathName, exact: true }).waitFor()
}

async function openVisibilityManagement(page) {
  await page.getByRole('button', { name: 'Manage Path', exact: true }).click()
  await page.getByRole('heading', { name: 'Path visibility', exact: true }).waitFor()
}

async function submitVisibility(page, proposed) {
  await page.getByRole('radio', { name: proposed, exact: true }).check()
  await page.getByRole('button', { name: 'Save visibility', exact: true }).click()
}

const browser = await chromium.launch({ headless: true })
try {
  const ownerContext = await applicationContext(browser, ownerToken)
  const ownerPage = await ownerContext.newPage()
  const requests = []
  ownerPage.on('request', (request) => {
    if (isVisibilityUpdate(request)) requests.push(request)
  })
  await ownerPage.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await openPath(ownerPage)
  await openVisibilityManagement(ownerPage)
  await ownerPage.getByText('Current visibility: Private', { exact: true }).waitFor()

  await submitVisibility(ownerPage, 'Followers')
  const confirmation = ownerPage.getByRole('alertdialog')
  await confirmation.getByRole('heading', { name: 'Share this Path more broadly?', exact: true }).waitFor()
  await confirmation.getByText(`Change ${pathName} from Private to Followers?`, { exact: true }).waitFor()
  await confirmation.getByText('Eligible historical identity, progress, and activity from this Path will become visible to the broader audience.', { exact: true }).waitFor()
  await confirmation.getByText('Membership, recorded activity, progress, your profile, and unrelated Paths will not change.', { exact: true }).waitFor()
  await confirmation.getByRole('button', { name: 'Cancel', exact: true }).click()
  assert.equal(requests.length, 0, 'cancelled visibility confirmation issued a PUT')
  await ownerPage.getByText('Current visibility: Private', { exact: true }).waitFor()

  await submitVisibility(ownerPage, 'Followers')
  const [expandedResponse] = await Promise.all([
    ownerPage.waitForResponse((response) => isVisibilityUpdate(response.request())),
    ownerPage.getByRole('button', { name: 'Confirm visibility change', exact: true }).click(),
  ])
  assert.equal(expandedResponse.status(), 200, 'confirmed visibility expansion failed')
  assert.deepEqual(expandedResponse.request().postDataJSON(), {
    confirmed: true,
    expectedVisibility: 'private',
    visibility: 'followers',
  })
  const expansionKey = expandedResponse.request().headers()['idempotency-key']
  assert.ok(expansionKey && expansionKey.length >= 16, 'visibility expansion omitted its Idempotency-Key')
  const expandedBody = await expandedResponse.json()
  assert.equal(expandedBody?.data?.id, pathID)
  assert.equal(expandedBody?.data?.visibility, 'followers')
  assert.equal(expandedBody?.data?.capabilities?.manageVisibility, true)
  await ownerPage.getByText('Current visibility: Followers', { exact: true }).waitFor()
  await ownerPage.getByText('Path visibility updated.', { exact: true }).waitFor()

  const participantContext = await applicationContext(browser, participantToken)
  const participantPage = await participantContext.newPage()
  await participantPage.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await openPath(participantPage)
  assert.equal(
    await participantPage.getByRole('button', { name: 'Manage Path', exact: true }).count(),
    0,
    'participant was shown creator-only visibility management',
  )
  await participantContext.close()

  const narrowingResponsePromise = ownerPage.waitForResponse((response) => isVisibilityUpdate(response.request()))
  await submitVisibility(ownerPage, 'Private')
  const narrowedResponse = await narrowingResponsePromise
  assert.equal(narrowedResponse.status(), 200, 'explicit visibility contraction failed')
  assert.deepEqual(narrowedResponse.request().postDataJSON(), {
    confirmed: true,
    expectedVisibility: 'followers',
    visibility: 'private',
  })
  assert.notEqual(
    narrowedResponse.request().headers()['idempotency-key'],
    expansionKey,
    'different visibility transition reused its Idempotency-Key',
  )
  await ownerPage.getByText('Current visibility: Private', { exact: true }).waitFor()
  await ownerPage.reload({ waitUntil: 'domcontentloaded' })
  await openPath(ownerPage)
  await openVisibilityManagement(ownerPage)
  await ownerPage.getByText('Current visibility: Private', { exact: true }).waitFor()
  assert.equal(requests.length, 2, 'visibility workflow issued an unexpected number of PUTs')

  await ownerContext.close()
  console.log('browser creator-only visibility confirmation, cancellation, contraction, and reload acceptance passed')
} finally {
  await browser.close()
}
