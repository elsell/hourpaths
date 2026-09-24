export async function ensureTryClientOpen(ui, buttonName, timeout) {
  const deadline = ui.now() + timeout
  const remaining = () => Math.max(0, deadline - ui.now())

  if (await ui.sendVisible()) return true
  if (remaining() === 0) return false
  if (await ui.closeVisible()) {
    const waitTimeout = remaining()
    return waitTimeout > 0 && await ui.waitForSend(waitTimeout)
  }

  const settleTimeout = Math.min(100, remaining())
  if (settleTimeout === 0) return false
  if (await ui.waitForSend(settleTimeout)) return true
  if (remaining() === 0) return false
  if (await ui.sendVisible()) return true
  if (remaining() === 0) return false
  if (await ui.closeVisible()) {
    const waitTimeout = remaining()
    return waitTimeout > 0 && await ui.waitForSend(waitTimeout)
  }

  const openTimeout = remaining()
  if (openTimeout === 0 || !await ui.openOperation(buttonName, openTimeout)) return false
  const waitTimeout = remaining()
  return waitTimeout > 0 && await ui.waitForSend(waitTimeout)
}

export async function reopenTryClient(ui, buttonName, timeout) {
  const deadline = ui.now() + timeout
  const remaining = () => Math.max(0, deadline - ui.now())

  if (!await ui.closeClient(remaining())) return false
  if (remaining() === 0 || !await ui.waitForClosed(remaining())) return false
  if (remaining() === 0 || !await ui.openOperation(buttonName, remaining())) return false
  return remaining() > 0 && await ui.waitForSend(remaining())
}

export async function waitForScalarCredential(port, expectedToken, timeout, pollIntervalMs = 50, stabilityIntervalMs = 500) {
  const deadline = port.now() + timeout
  let stableSince = undefined

  while (port.now() < deadline) {
    const matches = await port.credentialMatches(expectedToken)
    const observedAt = port.now()
    if (matches) {
      stableSince ??= observedAt
      if (observedAt <= deadline && observedAt - stableSince >= stabilityIntervalMs) return
    } else {
      stableSince = undefined
    }

    const remaining = deadline - observedAt
    if (remaining > 0) await port.pause(Math.min(pollIntervalMs, remaining))
  }

  throw new Error('Scalar did not apply the exchanged OIDC credential within the bounded readiness wait')
}

export async function waitForAuthorizedTryRequest(port, options) {
  const {
    buttonName,
    pathname,
    totalTimeoutMs = 45_000,
    responseTimeoutMs = 1_500,
    actionTimeoutMs = 2_000,
    retryPauseMs = 250,
    maxAttempts = Number.POSITIVE_INFINITY,
  } = options
  const deadline = port.now() + totalTimeoutMs
  const boundedTimeout = (maximum) => Math.max(1, Math.min(maximum, deadline - port.now()))
  const pauseBeforeRetry = async () => {
    const remaining = deadline - port.now()
    if (remaining > 0) await port.pause(Math.min(retryPauseMs, remaining))
  }

  for (let attempt = 0; attempt < maxAttempts && port.now() < deadline; attempt += 1) {
    if (!await port.ensureOpen(buttonName, boundedTimeout(actionTimeoutMs))) {
      await pauseBeforeRetry()
      continue
    }
    const responsePromise = port.waitForResponse(pathname, boundedTimeout(responseTimeoutMs))
    await port.send(boundedTimeout(actionTimeoutMs))
    const response = await responsePromise
    if (response === undefined) {
      await pauseBeforeRetry()
      continue
    }

    const authorization = await port.authorization(response)
    if (authorization?.startsWith('Bearer ')) {
      const status = port.status(response)
      if (status !== 200) {
        throw new Error(`Scalar Try It ${pathname} returned ${status}: ${await port.bodyText(response)}`)
      }
      return await port.parse(response)
    }
    await pauseBeforeRetry()
    if (port.now() < deadline) {
      await port.reopen(buttonName, boundedTimeout(actionTimeoutMs))
    }
  }
  throw new Error(`Scalar omitted the OIDC bearer token for ${pathname} after the bounded credential-application wait`)
}
