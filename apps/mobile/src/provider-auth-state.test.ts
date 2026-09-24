import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { loadProviderDiscovery } from './provider-discovery';
import { classifyProviderResponse, providerBusyAfterResponse } from './provider-auth-state';

it('keeps provider discovery network failure as controlled unavailable state', async () => {
  const discovery = await loadProviderDiscovery(async () => {
    throw new TypeError('Network request failed');
  });

  assert.equal(discovery, null);
});

describe('providerBusyAfterResponse', () => {
  it('settles immediately after provider cancellation or failure', () => {
    assert.equal(providerBusyAfterResponse('cancelled'), false);
    assert.equal(providerBusyAfterResponse('failed'), false);
  });

  it('remains active while the provider response is pending or exchanging', () => {
    assert.equal(providerBusyAfterResponse('pending'), true);
    assert.equal(providerBusyAfterResponse('success'), true);
  });
});

it('returns a successfully loaded provider discovery document', async () => {
  const expected = { authorizationEndpoint: 'https://identity.example/auth' };

  assert.equal(await loadProviderDiscovery(async () => expected), expected);
});

describe('classifyProviderResponse', () => {
  it('classifies explicit provider errors as failures', () => {
    assert.equal(classifyProviderResponse('error'), 'failed');
  });

  for (const type of ['cancel', 'dismiss']) {
    it(`classifies ${type} as cancellation`, () => {
      assert.equal(classifyProviderResponse(type), 'cancelled');
    });
  }

  it('classifies successful responses', () => {
    assert.equal(classifyProviderResponse('success'), 'success');
  });

  for (const type of ['opened', 'locked', undefined]) {
    it(`keeps ${type} pending`, () => {
      assert.equal(classifyProviderResponse(type), 'pending');
    });
  }
});
