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
  await context.addInitScript(({ applicationToken }) => {
    window.sessionStorage.setItem('hourpaths_application_session', JSON.stringify({
      token: applicationToken,
      expiresAt: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      nextAction: 'home',
    }))
  }, { applicationToken: token })
  const page = await context.newPage()
  await page.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await page.getByRole('heading', { name: 'Home', exact: true }).waitFor()
  if (await page.getByRole('button', { name: 'Add activity', exact: true }).count() !== 0) {
    throw new Error('Home exposed manual activity entry')
  }

  const noGoalCard = page.locator('li').filter({ has: page.getByRole('button', { name: 'Piano practice', exact: true }) })
  if (await noGoalCard.count() !== 1) throw new Error('no-goal Path was missing from Home')
  if (await noGoalCard.getByRole('progressbar').count() !== 0) throw new Error('no-goal Path rendered overall progress')
  if (await noGoalCard.getByText(/seconds accumulated$/).count() !== 1) throw new Error('no-goal Path omitted accumulated time')
  const initialProgress = '0 of 120 seconds toward overall target'
  const initialIntervalProgress = '0 of 60 seconds this interval'
  await page.getByRole('progressbar', { name: initialProgress, exact: true }).waitFor()
  await page.getByRole('progressbar', { name: initialIntervalProgress, exact: true }).waitFor()

  await page.getByRole('button', { name: pathName, exact: true }).click()
  await page.getByRole('heading', { name: pathName, exact: true }).waitFor()
  await page.getByRole('progressbar', { name: initialProgress, exact: true }).waitFor()
  await page.getByRole('progressbar', { name: initialIntervalProgress, exact: true }).waitFor()
  await page.getByRole('button', { name: 'Add activity', exact: true }).click()
  await page.getByRole('textbox', { name: 'Duration in seconds', exact: true }).fill('60')
  await page.getByRole('textbox', { name: 'Private note (optional)', exact: true }).fill('browser private note')
  const createdResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith('/activities') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save activity', exact: true }).click()
  const created = await createdResponse
  if (created.status() !== 201) throw new Error(`browser manual create failed: ${created.status()}`)
  const createdBody = await created.json()
  const activityID = createdBody?.data?.activity?.id
  if (!activityID || createdBody?.data?.version !== 1 || createdBody?.data?.activity?.note !== 'browser private note') {
    throw new Error('browser manual create response was incomplete')
  }
  if (createdBody?.data?.accumulatedSeconds !== 60) throw new Error('browser manual create did not return 60 seconds accumulated')
  if (createdBody?.data?.intervalProgress?.targetSeconds !== 60 || createdBody?.data?.intervalProgress?.accumulatedSeconds !== 60) {
    throw new Error('browser manual create did not return authoritative current interval progress')
  }
  await page.getByRole('progressbar', { name: '60 of 120 seconds toward overall target', exact: true }).waitFor()
  await page.getByRole('progressbar', { name: '60 of 60 seconds — interval goal completed', exact: true }).waitFor()
  await page.getByText('Activity saved. Current revision: 1.', { exact: true }).waitFor()

  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await page.getByRole('button', { name: 'Back to Home', exact: true }).click()
  await page.getByRole('button', { name: pathName, exact: true }).click()
  const historyResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith('/activities') && response.request().method() === 'GET')
  await page.getByRole('button', { name: 'View activity history', exact: true }).click()
  const historyResponse = await historyResponsePromise
  if (historyResponse.status() !== 200) throw new Error(`browser Path history failed: ${historyResponse.status()}`)
  const createdRow = page.locator(`[data-activity-id="${activityID}"]`)
  if (await createdRow.count() !== 1) throw new Error('browser-created activity did not have exactly one stable history control')
  const detailResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(`/activities/${activityID}`) && response.request().method() === 'GET')
  await createdRow.click()
  const detailResponse = await detailResponsePromise
  if (detailResponse.status() !== 200) throw new Error(`browser activity detail failed: ${detailResponse.status()}`)
  const detailBody = await detailResponse.json()
  if (detailBody?.data?.activity?.id !== activityID || detailBody?.data?.activity?.note !== 'browser private note') {
    throw new Error('browser activity detail did not retain the created private note')
  }
  await page.getByRole('heading', { name: 'Activity details', exact: true }).waitFor()
  await page.getByText('browser private note', { exact: false }).waitFor()

  await page.getByRole('button', { name: 'Edit activity', exact: true }).click()
  const editStartInstant = new Date(Date.parse(createdBody.data.activity.startedAt) - 120_000)
  const editStart = participantLocalFields(editStartInstant, createdBody.data.activity.occurrenceTimeZone)
  await page.getByLabel('Date', { exact: true }).fill(editStart.localDate)
  await page.getByLabel('Start time', { exact: true }).fill(editStart.localTime)
  await page.getByRole('textbox', { name: 'Duration in seconds', exact: true }).fill('133')
  await page.getByRole('textbox', { name: 'Private note (optional)', exact: true }).fill('browser edited note')
  const updatedResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(`/activities/${activityID}`) && response.request().method() === 'PUT')
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  const updated = await updatedResponse
  if (updated.status() !== 200) throw new Error(`browser manual update failed: ${updated.status()}`)
  const updatedBody = await updated.json()
  if (updatedBody?.data?.version !== 2 || updatedBody?.data?.activity?.durationSeconds !== 133 || updatedBody?.data?.accumulatedSeconds !== 133) {
    throw new Error('browser manual update did not become the current revision')
  }
  await page.getByRole('progressbar', { name: '133 of 120 seconds — overall target completed', exact: true }).waitFor()
  await page.getByRole('heading', { name: 'Revision 1', exact: true }).waitFor()

  await page.getByRole('button', { name: 'Add activity', exact: true }).click()
  await page.getByRole('textbox', { name: 'Duration in seconds', exact: true }).fill('17')
  await page.getByRole('textbox', { name: 'Private note (optional)', exact: true }).fill('browser unrelated note')
  const unrelatedResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith('/activities') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save activity', exact: true }).click()
  const unrelatedResponse = await unrelatedResponsePromise
  if (unrelatedResponse.status() !== 201) throw new Error(`browser unrelated manual create failed: ${unrelatedResponse.status()}`)
  const unrelatedBody = await unrelatedResponse.json()
  const unrelatedActivityID = unrelatedBody?.data?.activity?.id
  const totalBeforeDeletion = unrelatedBody?.data?.accumulatedSeconds
  if (!unrelatedActivityID || unrelatedActivityID === activityID || unrelatedBody?.data?.activity?.note !== 'browser unrelated note' || totalBeforeDeletion !== 150) {
    throw new Error('browser unrelated manual create response was incomplete')
  }
  await page.getByRole('progressbar', { name: '150 of 120 seconds — overall target completed', exact: true }).waitFor()
  await page.getByText('Activity saved. Current revision: 1.', { exact: true }).waitFor()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()

  const unrelatedRow = page.locator(`[data-activity-id="${unrelatedActivityID}"]`)
  if (await createdRow.count() !== 1 || await unrelatedRow.count() !== 1) {
    throw new Error('browser history did not retain both activities before deletion')
  }
  await createdRow.click()
  await page.getByText('browser edited note', { exact: false }).waitFor()
  const totalBeforeDeletionText = `${new Intl.NumberFormat('en-US').format(totalBeforeDeletion)} seconds accumulated`
  await page.getByText(totalBeforeDeletionText, { exact: true }).waitFor()

  await page.getByRole('button', { name: 'Delete activity', exact: true }).click()
  await page.getByText(deleteConfirmation, { exact: true }).waitFor()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  if (await createdRow.count() !== 1 || await unrelatedRow.count() !== 1) {
    throw new Error('cancelled deletion removed an activity')
  }
  if (await page.getByText(totalBeforeDeletionText, { exact: true }).count() !== 1) {
    throw new Error('cancelled deletion changed the authoritative total')
  }
  await page.getByText('browser edited note', { exact: false }).waitFor()

  await page.getByRole('button', { name: 'Delete activity', exact: true }).click()
  const deletedResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(`/activities/${activityID}`) && response.request().method() === 'DELETE')
  await page.getByRole('button', { name: 'Delete activity', exact: true }).click()
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
  await createdRow.waitFor({ state: 'detached' })
  if (await unrelatedRow.count() !== 1) throw new Error('browser deletion removed the unrelated activity')
  if (await page.getByRole('heading', { name: 'Activity details', exact: true }).count() !== 0) {
    throw new Error('browser deletion left the deleted activity details open')
  }
  if (await page.getByText('browser edited note', { exact: false }).count() !== 0) {
    throw new Error('browser deletion retained the deleted private note')
  }
  const totalAfterDeletionText = `${new Intl.NumberFormat('en-US').format(expectedTotalAfterDeletion)} seconds accumulated`
  await page.getByText(totalAfterDeletionText, { exact: true }).waitFor()
  await page.getByRole('progressbar', { name: '17 of 120 seconds toward overall target', exact: true }).waitFor()

  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.getByRole('heading', { name: 'Home', exact: true }).waitFor()
  await page.getByRole('progressbar', { name: '17 of 120 seconds toward overall target', exact: true }).waitFor()
  await page.getByRole('button', { name: pathName, exact: true }).click()
  const reloadedHistoryResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith('/activities') && response.request().method() === 'GET')
  await page.getByRole('button', { name: 'View activity history', exact: true }).click()
  const reloadedHistoryResponse = await reloadedHistoryResponsePromise
  if (reloadedHistoryResponse.status() !== 200) throw new Error(`browser reloaded Path history failed: ${reloadedHistoryResponse.status()}`)
  if (await page.locator(`[data-activity-id="${activityID}"]`).count() !== 0) {
    throw new Error('deleted activity survived the browser reload')
  }
  if (await page.locator(`[data-activity-id="${unrelatedActivityID}"]`).count() !== 1) {
    throw new Error('unrelated activity did not survive the browser reload')
  }
  await page.getByText(totalAfterDeletionText, { exact: true }).waitFor()
  await page.getByRole('progressbar', { name: '17 of 120 seconds toward overall target', exact: true }).waitFor()
  console.log('browser Path manual create, reopen, edit, revision, cancel-delete, delete, and reload acceptance passed')
} finally {
  await browser.close()
}
