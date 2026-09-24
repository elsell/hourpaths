import assert from 'node:assert/strict'
import { chromium } from 'playwright'

const webBaseURL = process.env.WEB_ACCEPTANCE_BASE_URL ?? 'http://localhost:5173'
const apiBaseURL = process.env.WEB_ACCEPTANCE_API_URL ?? 'http://localhost:8080'
const token = process.env.WEB_ACCEPTANCE_APPLICATION_TOKEN
const notificationID = process.env.WEB_ACCEPTANCE_NOTIFICATION_ID
const invitationID = process.env.WEB_ACCEPTANCE_INVITATION_ID
if (!token) throw new Error('WEB_ACCEPTANCE_APPLICATION_TOKEN is required')
if (!notificationID) throw new Error('WEB_ACCEPTANCE_NOTIFICATION_ID is required')
if (!invitationID) throw new Error('WEB_ACCEPTANCE_INVITATION_ID is required')

async function openApplicationPage(context) {
  const page = await context.newPage()
  await page.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  await page.getByRole('heading', { name: 'Home', exact: true }).waitFor()
  await page.getByRole('button', { name: /^Notifications/ }).click()
  const row = page.locator('#notification-history li').filter({ hasText: 'Invitation acceptance' })
  await row.waitFor()
  return { page, row }
}

async function notificationFromServer(page) {
  return page.evaluate(async ({ apiURL, applicationToken, expectedID }) => {
    const response = await fetch(`${apiURL}/v1/notifications?limit=25`, {
      headers: { Authorization: `Bearer ${applicationToken}` },
    })
    if (!response.ok) throw new Error(`notification history request returned ${response.status}`)
    const envelope = await response.json()
    return envelope.data.find((item) => item.id === expectedID) ?? null
  }, { apiURL: apiBaseURL, applicationToken: token, expectedID: notificationID })
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

  const first = await openApplicationPage(context)
  const second = await openApplicationPage(context)
  await second.row.getByText('Unread', { exact: true }).waitFor()
  await first.page.bringToFront()
  await first.row.getByText('Unread', { exact: true }).waitFor()

  await first.row.locator('button.notification-link').click()
  await second.row.getByText('Unread', { exact: true }).waitFor({ state: 'detached' })
  assert.equal((await notificationFromServer(second.page))?.read, true)

  await first.page.getByRole('button', { name: /^Notifications/ }).click()
  const refreshedFirstRow = first.page.locator('#notification-history li').filter({ hasText: 'Invitation acceptance' })
  await refreshedFirstRow.waitFor()
  await refreshedFirstRow.getByRole('button', { name: 'Delete notification', exact: true }).click()
  await second.row.waitFor({ state: 'detached' })
  assert.equal(await notificationFromServer(second.page), null)

  const retainedInvitation = await second.page.evaluate(async ({ apiURL, applicationToken, expectedID }) => {
    const response = await fetch(`${apiURL}/v1/path-invitations?limit=25`, {
      headers: { Authorization: `Bearer ${applicationToken}` },
    })
    if (!response.ok) throw new Error(`pending invitation request returned ${response.status}`)
    const envelope = await response.json()
    return envelope.data.find((item) => item.invitation.id === expectedID) ?? null
  }, { apiURL: apiBaseURL, applicationToken: token, expectedID: invitationID })
  assert.equal(retainedInvitation?.invitation?.id, invitationID)
  assert.equal(retainedInvitation?.invitation?.acceptedAt, undefined)
  console.log('two-page notification read/delete convergence and underlying invitation retention acceptance passed')
} finally {
  await browser.close()
}
