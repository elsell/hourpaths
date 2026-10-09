import { chromium } from 'playwright'
import { readFileSync } from 'node:fs'

const englishCatalog = JSON.parse(readFileSync(new URL('../packages/i18n/src/locales/en.json', import.meta.url), 'utf8'))
const deleteConfirmation = englishCatalog['pathDetails.deleteConfirmation']
if (typeof deleteConfirmation !== 'string' || deleteConfirmation.length === 0) {
  throw new Error('English activity deletion confirmation is missing')
}

const webBaseURL = process.env.WEB_ACCEPTANCE_BASE_URL ?? 'http://localhost:5173'
const apiBaseURL = process.env.WEB_ACCEPTANCE_API_URL ?? 'http://localhost:8080'
const token = process.env.WEB_ACCEPTANCE_APPLICATION_TOKEN
const pathName = process.env.WEB_ACCEPTANCE_PATH_NAME ?? 'Piano practice'
if (!token) throw new Error('WEB_ACCEPTANCE_APPLICATION_TOKEN is required')

function participantLocalFields(instant, timeZone) {
  const parts = new Intl.DateTimeFormat('en', {
    timeZone,
    calendar: 'iso8601',
    numberingSystem: 'latn',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(instant)
  const value = (type) => parts.find((part) => part.type === type)?.value
  const fields = {
    localDate: `${value('year')}-${value('month')}-${value('day')}`,
    localTime: `${value('hour')}:${value('minute')}:${value('second')}`,
  }
  if (Object.values(fields).some((field) => field.includes('undefined'))) {
    throw new Error('browser edit start could not be represented in the participant time zone')
  }
  return fields
}

const browser = await chromium.launch({ headless: true })
try {
  const context = await browser.newContext({ locale: 'en-US' })
  // Shared acceptance accounts may cross a minute-bound rate-limit window.
  // Allow the real client retry to finish without bypassing that boundary.
  context.setDefaultTimeout(75_000)
  await context.addInitScript(({ applicationToken }) => {
    window.sessionStorage.setItem('hourpaths_application_session', JSON.stringify({
      token: applicationToken,
      expiresAt: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      nextAction: 'home',
    }))
  }, { applicationToken: token })
  const page = await context.newPage()
  await page.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await page.getByRole('heading', { name: 'Paths', exact: true }).waitFor()
  if (await page.getByRole('link', { name: 'Add activity', exact: true }).isVisible().catch(() => false)) {
    throw new Error('Home exposed manual activity entry outside the Path menu')
  }
  const card = () => page.locator('li').filter({ has: page.getByRole('button', { name: pathName, exact: true }) })
  async function home() {
    await page.getByRole('link', { name: 'Paths', exact: true }).click()
    await page.getByRole('heading', { name: 'Paths', exact: true }).waitFor()
    await card().waitFor()
  }
  async function addActivity(seconds, note) {
    await home()
    await card().locator('summary').click()
    await card().getByRole('link', { name: 'Add activity', exact: true }).click()
    await page.getByRole('spinbutton', { name: 'Seconds', exact: true }).fill(String(seconds))
    await page.locator('details > summary').click()
    await page.getByRole('textbox', { name: 'Private note (optional)', exact: true }).fill(note)
  }
  const historyRow = id => page.locator(`.studio-history a[href$="/activities/${id}"]`)
  async function progress(label) {
    await home()
    await card().getByRole('progressbar', { name: label, exact: true }).waitFor()
  }
  const noGoalCard = page.locator('li').filter({ has: page.getByRole('button', { name: 'Piano practice', exact: true }) })
  await noGoalCard.waitFor()
  if (await noGoalCard.count() !== 1) throw new Error('no-goal Path was missing from Home')
  if (await noGoalCard.getByRole('progressbar').count() !== 0) throw new Error('no-goal Path rendered overall progress')
  await noGoalCard.getByText('Total time', { exact: true }).waitFor()
  await card().getByRole('progressbar', { name: '0 of 120 seconds toward overall target', exact: true }).waitFor()
  await card().getByRole('progressbar', { name: '0 of 60 seconds this interval', exact: true }).waitFor()
  await addActivity(60, 'browser private note')
  const createdResponse = page.waitForResponse((response) =>
    response.url().startsWith(`${apiBaseURL}/v1/`) && new URL(response.url()).pathname.endsWith('/offline-activity') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save activity', exact: true }).click()
  const created = await createdResponse
  if (created.status() !== 200) throw new Error(`browser manual create failed: ${created.status()} ${JSON.stringify(await created.json())}`)
  const createdBody = await created.json()
  const activityID = createdBody?.data?.activity?.id
  if (!activityID || createdBody?.data?.outcome !== 'accepted' || createdBody?.data?.activity?.note !== 'browser private note') {
    throw new Error('browser manual create response was incomplete')
  }
  if (createdBody?.data?.activity?.durationSeconds !== 60) throw new Error('browser manual create did not persist 60 seconds')
  await page.getByText('Activity saved. Current revision: 1.', { exact: true }).waitFor()
  await progress('60 of 120 seconds toward overall target')
  await card().getByRole('progressbar', { name: '60 of 60 seconds — interval goal completed', exact: true }).waitFor()
  const createdRow = historyRow(activityID)
  await createdRow.waitFor({ state: 'visible' })
  if (await createdRow.count() !== 1) throw new Error('browser-created activity did not have exactly one stable history control')
  await createdRow.click()
  await page.getByRole('heading', { name: 'Activity details', exact: true }).waitFor()
  const detailResponsePromise = page.waitForResponse((response) =>
    response.url().startsWith(`${apiBaseURL}/v1/`) && new URL(response.url()).pathname.endsWith(`/activities/${activityID}`) && response.request().method() === 'GET')
  await page.waitForLoadState('networkidle')
  await page.reload({ waitUntil: 'domcontentloaded' })
  const detailResponse = await detailResponsePromise
  if (detailResponse.status() !== 200) throw new Error(`browser activity detail failed: ${detailResponse.status()}`)
  const detailBody = await detailResponse.json()
  if (detailBody?.data?.activity?.id !== activityID || detailBody?.data?.version !== 1 || detailBody?.data?.activity?.note !== 'browser private note') {
    throw new Error('browser activity detail did not retain the created private note')
  }
  await page.getByRole('heading', { name: 'Activity details', exact: true }).waitFor()
  await page.getByText('browser private note', { exact: false }).waitFor()

  await page.getByRole('link', { name: 'Edit activity', exact: true }).click()
  const editStartInstant = new Date(Date.parse(createdBody.data.activity.startedAt) - 120_000)
  const editStart = participantLocalFields(editStartInstant, createdBody.data.activity.occurrenceTimeZone)
  await page.getByLabel('Date', { exact: true }).fill(editStart.localDate)
  await page.getByLabel('Start time', { exact: true }).fill(editStart.localTime)
  await page.getByRole('spinbutton', { name: 'Minutes', exact: true }).fill('2')
  await page.getByRole('spinbutton', { name: 'Seconds', exact: true }).fill('13')
  await page.getByRole('textbox', { name: 'Private note (optional)', exact: true }).fill('browser edited note')
  const updatedResponse = page.waitForResponse((response) =>
    response.url().startsWith(`${apiBaseURL}/v1/`) && new URL(response.url()).pathname.endsWith('/offline-activity') && response.request().method() === 'POST' && response.request().postDataJSON()?.activityId === activityID)
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  const updated = await updatedResponse
  if (updated.status() !== 200) throw new Error(`browser manual update failed: ${updated.status()}`)
  const updatedBody = await updated.json()
  if (updatedBody?.data?.outcome !== 'accepted' || updatedBody?.data?.activity?.durationSeconds !== 133 || updatedBody?.data?.activity?.note !== 'browser edited note') {
    throw new Error('browser manual update did not become the current revision')
  }
  await page.waitForLoadState('networkidle')
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.getByRole('heading', { name: 'Revision 1', exact: true }).waitFor()
  await page.getByText('Activity saved. Current revision: 2.', { exact: true }).waitFor()
  await progress('133 of 120 seconds — overall target completed')
  await addActivity(17, 'browser unrelated note')
  const unrelatedResponsePromise = page.waitForResponse((response) =>
    response.url().startsWith(`${apiBaseURL}/v1/`) && new URL(response.url()).pathname.endsWith('/offline-activity') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save activity', exact: true }).click()
  const unrelatedResponse = await unrelatedResponsePromise
  if (unrelatedResponse.status() !== 200) throw new Error(`browser unrelated manual create failed: ${unrelatedResponse.status()}`)
  const unrelatedBody = await unrelatedResponse.json()
  const unrelatedActivityID = unrelatedBody?.data?.activity?.id
  const totalBeforeDeletion = 150
  if (!unrelatedActivityID || unrelatedActivityID === activityID || unrelatedBody?.data?.activity?.note !== 'browser unrelated note' || unrelatedBody?.data?.outcome !== 'accepted' || unrelatedBody?.data?.activity?.durationSeconds !== 17) {
    throw new Error('browser unrelated manual create response was incomplete')
  }
  await page.getByText('Activity saved. Current revision: 1.', { exact: true }).waitFor()
  await progress('150 of 120 seconds — overall target completed')
  const unrelatedRow = historyRow(unrelatedActivityID)
  await unrelatedRow.waitFor()
  if (await createdRow.count() !== 1 || await unrelatedRow.count() !== 1) {
    throw new Error('browser history did not retain both activities before deletion')
  }
  await createdRow.click()
  await page.getByText('browser edited note', { exact: true }).waitFor()
  const deletionRequests = []
  page.on('request', request => {
    if (request.method() === 'DELETE' && new URL(request.url()).pathname.endsWith(`/activities/${activityID}`)) deletionRequests.push(request)
  })
  await page.getByRole('button', { name: 'Delete activity', exact: true }).click()
  await page.getByText(deleteConfirmation, { exact: true }).waitFor()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  if (deletionRequests.length) throw new Error('cancelled deletion issued a mutation')
  await progress('150 of 120 seconds — overall target completed')
  if (await createdRow.count() !== 1 || await unrelatedRow.count() !== 1) {
    throw new Error('cancelled deletion changed the authoritative total or removed an activity')
  }
  await createdRow.click()
  await page.getByText('browser edited note', { exact: true }).waitFor()
  await page.getByRole('button', { name: 'Delete activity', exact: true }).click()
  const deletedResponsePromise = page.waitForResponse((response) =>
    response.url().startsWith(`${apiBaseURL}/v1/`) && new URL(response.url()).pathname.endsWith(`/activities/${activityID}`) && response.request().method() === 'DELETE')
  await page.getByRole('dialog').getByRole('button', { name: 'Delete activity', exact: true }).click()
  const deletedResponse = await deletedResponsePromise
  if (deletedResponse.status() !== 200) throw new Error(`browser activity deletion failed: ${deletedResponse.status()}`)
  const deletionIdempotencyKey = deletedResponse.request().headers()['idempotency-key']
  if (!deletionIdempotencyKey || deletionIdempotencyKey.length < 16) {
    throw new Error('browser activity deletion did not send an Idempotency-Key')
  }
  const deletedBody = await deletedResponse.json()
  const expectedTotalAfterDeletion = totalBeforeDeletion - 133
  if (deletedBody?.data?.accumulatedSeconds !== expectedTotalAfterDeletion) {
    throw new Error('browser deletion did not return the authoritative total')
  }
  await page.getByRole('heading', { name: 'Paths', exact: true }).waitFor()
  await unrelatedRow.waitFor()
  await createdRow.waitFor({ state: 'detached' })
  if (await unrelatedRow.count() !== 1) throw new Error('browser deletion removed the unrelated activity')
  if (await page.getByRole('heading', { name: 'Activity details', exact: true }).count() !== 0) {
    throw new Error('browser deletion left the deleted activity details open')
  }
  if (await page.getByText('browser edited note', { exact: false }).count() !== 0) {
    throw new Error('browser deletion retained the deleted private note')
  }
  await card().getByRole('progressbar', { name: '17 of 120 seconds toward overall target', exact: true }).waitFor()
  await page.waitForLoadState('networkidle')
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.getByRole('heading', { name: 'Paths', exact: true }).waitFor()
  await unrelatedRow.waitFor()
  if (await createdRow.count() !== 0) throw new Error('deleted activity survived the browser reload')
  if (await unrelatedRow.count() !== 1) throw new Error('unrelated activity did not survive the browser reload')
  await card().getByRole('progressbar', { name: '17 of 120 seconds toward overall target', exact: true }).waitFor()
  console.log('browser Path manual create, reopen, edit, revision, cancel-delete, delete, and reload acceptance passed')
} finally {
  await browser.close()
}
