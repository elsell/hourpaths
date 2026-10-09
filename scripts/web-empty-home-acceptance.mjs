import { chromium } from 'playwright'
import { readFileSync } from 'node:fs'
const spanish = JSON.parse(readFileSync(new URL('../packages/i18n/src/locales/es.json', import.meta.url), 'utf8'))

const webBaseURL = process.env.WEB_ACCEPTANCE_BASE_URL ?? 'http://localhost:5173'
const apiBaseURL = process.env.WEB_ACCEPTANCE_API_URL ?? 'http://localhost:8080'
const token = process.env.WEB_ACCEPTANCE_APPLICATION_TOKEN
if (!token) throw new Error('WEB_ACCEPTANCE_APPLICATION_TOKEN is required')

const browser = await chromium.launch({ headless: true })
try {
  const context = await browser.newContext({ locale: 'es-ES' })
  await context.addInitScript(({ applicationToken }) => {
    window.sessionStorage.setItem('hourpaths_application_session', JSON.stringify({
      token: applicationToken,
      expiresAt: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      nextAction: 'home',
    }))
  }, { applicationToken: token })
  const page = await context.newPage()
  const profileResponse = page.waitForResponse(
    (response) => response.url() === `${apiBaseURL}/v1/me` && response.request().method() === 'GET',
  )
  const pathsResponse = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/v1/paths' && response.request().method() === 'GET',
  )
  await page.goto(webBaseURL, { waitUntil: 'domcontentloaded' })
  const [profile, paths] = await Promise.all([profileResponse, pathsResponse])
  if (profile.status() !== 200 || paths.status() !== 200) {
    throw new Error(`active Home requests failed: profile=${profile.status()} paths=${paths.status()}`)
  }
  const body = await paths.json()
  if (!Array.isArray(body?.data) || body.data.length !== 0) {
    throw new Error(`new active account did not have an empty server-backed Path collection: ${JSON.stringify(body)}`)
  }
  await page.waitForURL(url => ['/studio', '/studio/'].includes(url.pathname))
  await page.getByRole('heading', { name: spanish['studio.paths'], exact: true }).waitFor()
  await page.getByText(spanish['studio.empty'], { exact: true }).waitFor()
  await page.getByRole('button', { name: 'Crear ruta', exact: true }).click()
  await page.getByRole('heading', { name: 'Crear una ruta', exact: true }).waitFor()
  await page.getByRole('textbox', { name: 'Nombre de la ruta', exact: true }).waitFor()
  console.log('localized server-backed empty Home acceptance passed')
} finally {
  await browser.close()
}
