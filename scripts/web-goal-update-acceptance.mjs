import assert from 'node:assert/strict'
import { chromium } from 'playwright'

const webBaseURL = process.env.WEB_ACCEPTANCE_BASE_URL ?? 'http://localhost:5173'
const ownerToken = process.env.WEB_ACCEPTANCE_APPLICATION_TOKEN
const participantToken = process.env.WEB_ACCEPTANCE_PARTICIPANT_TOKEN
const pathID = process.env.WEB_ACCEPTANCE_PATH_ID
const pathName = process.env.WEB_ACCEPTANCE_PATH_NAME ?? 'Browser goal reconfiguration'
const alignmentWeekday = Number(process.env.WEB_ACCEPTANCE_ALIGNMENT_WEEKDAY)

if (!ownerToken) throw new Error('WEB_ACCEPTANCE_APPLICATION_TOKEN is required')
if (!participantToken) throw new Error('WEB_ACCEPTANCE_PARTICIPANT_TOKEN is required')
if (!pathID) throw new Error('WEB_ACCEPTANCE_PATH_ID is required')
if (!Number.isInteger(alignmentWeekday) || alignmentWeekday < 1 || alignmentWeekday > 7) {
  throw new Error('WEB_ACCEPTANCE_ALIGNMENT_WEEKDAY must be an ISO weekday')
}

const initialProgress = '45 of 60 seconds this interval'
const updatedProgress = '45 of 30 seconds — interval goal completed'
const accumulatedTotal = '45 seconds accumulated'
const warning = 'This change recalculates current and historical progress for every participant. Recorded activity will not change.'
const goalPathname = `/v1/paths/${pathID}/goals`

function isGoalUpdate(request) {
  return request.method() === 'PUT' && new URL(request.url()).pathname === goalPathname
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

async function waitForHome(page) {
  await page.getByRole('heading', { name: 'Home', exact: true }).waitFor()
}

function homePathCard(page) {
  return page.locator('li').filter({
    has: page.getByRole('button', { name: pathName, exact: true }),
  })
}

async function assertHomeInterval(page, label) {
  await waitForHome(page)
  const card = homePathCard(page)
  assert.equal(await card.count(), 1, 'goal-update Path was missing from Home')
  await card.getByRole('progressbar', { name: label, exact: true }).waitFor()
  await card.getByText(accumulatedTotal, { exact: true }).waitFor()
}

async function openPathDetail(page) {
  const card = homePathCard(page)
  await card.getByRole('button', { name: pathName, exact: true }).click()
  await page.getByRole('heading', { name: pathName, exact: true }).waitFor()
}

async function backToHome(page) {
  await page.getByRole('button', { name: 'Back to Home', exact: true }).click()
  await waitForHome(page)
}

async function setReviewedIntervalTarget(page, seconds) {
  await page.getByRole('button', { name: 'Manage Path', exact: true }).click()
  await page.getByRole('heading', { name: 'Manage Path goals', exact: true }).waitFor()
  await page.locator('#manage-interval-hours').fill('0')
  await page.locator('#manage-interval-minutes').fill('0')
  await page.locator('#manage-interval-seconds').fill(String(seconds))
  await page.getByRole('button', { name: 'Review goal changes', exact: true }).click()
  await page.getByRole('heading', { name: 'Current goals', exact: true }).waitFor()
  await page.getByRole('heading', { name: 'Proposed goals', exact: true }).waitFor()
  await page.getByText(warning, { exact: true }).waitFor()
}

async function confirmGoalUpdate(page) {
  const responsePromise = page.waitForResponse((response) => isGoalUpdate(response.request()))
  await page.getByRole('button', { name: 'Confirm goal changes', exact: true }).click()
  return responsePromise
}

const browser = await chromium.launch({ headless: true })
try {
  const ownerContext = await applicationContext(browser, ownerToken)
  const ownerPage = await ownerContext.newPage()
  const ownerGoalRequests = []
  ownerPage.on('request', (request) => {
    if (isGoalUpdate(request)) ownerGoalRequests.push(request)
  })

  await ownerPage.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await assertHomeInterval(ownerPage, initialProgress)
  await openPathDetail(ownerPage)
  await ownerPage.getByRole('progressbar', { name: initialProgress, exact: true }).waitFor()

  await setReviewedIntervalTarget(ownerPage, 30)
  await ownerPage.getByRole('button', { name: 'Cancel', exact: true }).click()
  assert.equal(ownerGoalRequests.length, 0, 'cancelled goal review issued a PUT')
  await ownerPage.getByRole('progressbar', { name: initialProgress, exact: true }).waitFor()
  await backToHome(ownerPage)
  await assertHomeInterval(ownerPage, initialProgress)
  await openPathDetail(ownerPage)

  await setReviewedIntervalTarget(ownerPage, 30)
  const updatedResponse = await confirmGoalUpdate(ownerPage)
  assert.equal(updatedResponse.status(), 200, 'confirmed owner goal update failed')
  assert.equal(ownerGoalRequests.length, 1, 'confirmed goal update did not issue exactly one PUT')
  const updatedRequest = updatedResponse.request()
  const updatedKey = updatedRequest.headers()['idempotency-key']
  assert.ok(updatedKey && updatedKey.length >= 16, 'confirmed goal update omitted its Idempotency-Key')
  assert.deepEqual(updatedRequest.postDataJSON(), {
    confirmed: true,
    expectedGoals: {
      intervalGoal: {
        targetSeconds: 60,
        recurrence: 'weekly',
        alignment: { isoWeekday: alignmentWeekday },
      },
    },
    intervalGoal: {
      targetSeconds: 30,
      recurrence: 'weekly',
      alignment: { isoWeekday: alignmentWeekday },
    },
  })
  const updatedBody = await updatedResponse.json()
  assert.equal(updatedBody?.data?.accumulatedSeconds, 45)
  assert.deepEqual(updatedBody?.data?.intervalProgress && {
    accumulatedSeconds: updatedBody.data.intervalProgress.accumulatedSeconds,
    targetSeconds: updatedBody.data.intervalProgress.targetSeconds,
  }, { accumulatedSeconds: 45, targetSeconds: 30 })
  await ownerPage.getByRole('progressbar', { name: updatedProgress, exact: true }).waitFor()
  await backToHome(ownerPage)
  await assertHomeInterval(ownerPage, updatedProgress)

  const participantContext = await applicationContext(browser, participantToken)
  const participantPage = await participantContext.newPage()
  const participantGoalRequests = []
  participantPage.on('request', (request) => {
    if (isGoalUpdate(request)) participantGoalRequests.push(request)
  })
  await participantPage.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await waitForHome(participantPage)
  await openPathDetail(participantPage)
  assert.equal(
    await participantPage.getByRole('button', { name: 'Manage Path', exact: true }).count(),
    0,
    'participant was shown Manage Path despite manageGoals=false',
  )
  assert.equal(participantGoalRequests.length, 0, 'participant capability-gated view issued a goal PUT')
  await participantContext.close()

  await ownerPage.reload({ waitUntil: 'domcontentloaded' })
  await assertHomeInterval(ownerPage, updatedProgress)
  await openPathDetail(ownerPage)
  await ownerPage.getByRole('progressbar', { name: updatedProgress, exact: true }).waitFor()
  assert.equal(ownerGoalRequests.length, 1, 'participant view changed the owner goal projection')

  await ownerPage.getByRole('button', { name: 'Manage Path', exact: true }).click()
  await ownerPage.getByRole('heading', { name: 'Manage Path goals', exact: true }).waitFor()
  await ownerPage.getByRole('checkbox', { name: 'Add an interval goal', exact: true }).uncheck()
  await ownerPage.getByRole('button', { name: 'Review goal changes', exact: true }).click()
  await ownerPage.getByText(warning, { exact: true }).waitFor()
  const removedResponse = await confirmGoalUpdate(ownerPage)
  assert.equal(removedResponse.status(), 200, 'confirmed goal removal failed')
  assert.equal(ownerGoalRequests.length, 2, 'goal removal did not issue exactly one additional PUT')
  assert.deepEqual(removedResponse.request().postDataJSON(), {
    confirmed: true,
    expectedGoals: {
      intervalGoal: {
        targetSeconds: 30,
        recurrence: 'weekly',
        alignment: { isoWeekday: alignmentWeekday },
      },
    },
  })
  const removedBody = await removedResponse.json()
  assert.equal(removedBody?.data?.accumulatedSeconds, 45)
  assert.equal(removedBody?.data?.intervalProgress, undefined, 'goal removal retained interval progress')
  assert.equal(removedBody?.data?.path?.intervalGoal, undefined, 'goal removal retained the interval goal')
  await ownerPage.getByText(accumulatedTotal, { exact: true }).waitFor()
  assert.equal(await ownerPage.getByRole('progressbar').count(), 0, 'goal removal retained interval progress')
  await backToHome(ownerPage)
  const removedCard = homePathCard(ownerPage)
  await removedCard.getByText(accumulatedTotal, { exact: true }).waitFor()
  assert.equal(await removedCard.getByRole('progressbar').count(), 0, 'Home retained interval progress after removal')

  await ownerContext.close()
  console.log('browser goal update confirmation, cancellation, denial, removal, and projection acceptance passed')
} finally {
  await browser.close()
}
