export function isDexLoginURL(url, dexOrigin) {
  return url.origin === dexOrigin && url.pathname.replace(/\/$/, '') === '/dex/auth/local/login'
}

export async function navigateToDexLogin(port, options) {
  const { dexOrigin, timeoutMs = 15_000 } = options
  const deadline = port.now() + timeoutMs
  const remaining = () => Math.max(0, deadline - port.now())
  const initialTimeout = remaining()
  if (initialTimeout === 0) throw new Error('Dex login navigation exhausted its bounded wait before sign-in')

  const navigationPromise = port.waitForLoginURL(
    (url) => isDexLoginURL(url, dexOrigin),
    initialTimeout,
  )
  const [loginURL] = await Promise.all([
    navigationPromise,
    port.clickSignIn(initialTimeout),
  ])

  const formTimeout = remaining()
  if (formTimeout === 0) throw new Error(`Dex login form did not become ready within ${timeoutMs}ms`)
  await port.waitForLoginForm(formTimeout)
  return loginURL
}
