import assert from 'node:assert/strict';

assert.equal(process.env.HOURPATHS_APP_ENV, 'production', 'signed mobile releases require HOURPATHS_APP_ENV=production');
for (const [name, value] of [
  ['HOURPATHS_API_URL', process.env.HOURPATHS_API_URL],
  ['HOURPATHS_OIDC_ISSUER', process.env.HOURPATHS_OIDC_ISSUER],
  ['HOURPATHS_MOBILE_OIDC_CLIENT_ID', process.env.HOURPATHS_MOBILE_OIDC_CLIENT_ID],
  ['HOURPATHS_EXPO_PROJECT_ID', process.env.HOURPATHS_EXPO_PROJECT_ID],
]) {
  assert.ok(value?.trim(), `${name} is required for a production mobile release`);
}
assert.doesNotMatch(process.env.HOURPATHS_MOBILE_OIDC_CLIENT_ID, /\s/, 'HOURPATHS_MOBILE_OIDC_CLIENT_ID must not contain whitespace');
assert.match(
  process.env.HOURPATHS_EXPO_PROJECT_ID,
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/iu,
  'HOURPATHS_EXPO_PROJECT_ID must be an Expo project UUID',
);
for (const [name, value] of [['HOURPATHS_API_URL', process.env.HOURPATHS_API_URL], ['HOURPATHS_OIDC_ISSUER', process.env.HOURPATHS_OIDC_ISSUER]]) {
  const parsed = new URL(value);
  const hostname = parsed.hostname.toLowerCase().replace(/^\[|\]$/g, '');
  const privateIPv4Address = (octets) => octets.length === 4 && octets.every((part) => Number.isInteger(part) && part >= 0 && part <= 255) && (
    octets[0] === 0 || octets[0] === 10 || octets[0] === 127 ||
    (octets[0] === 169 && octets[1] === 254) ||
    (octets[0] === 172 && octets[1] >= 16 && octets[1] <= 31) ||
    (octets[0] === 192 && octets[1] === 168)
  );
  const privateIPv4 = privateIPv4Address(hostname.split('.').map(Number));
  const mapped = /^::ffff:([0-9a-f]{1,4}):([0-9a-f]{1,4})$/.exec(hostname);
  const mappedIPv4 = mapped ? privateIPv4Address([
    Number.parseInt(mapped[1], 16) >>> 8,
    Number.parseInt(mapped[1], 16) & 0xff,
    Number.parseInt(mapped[2], 16) >>> 8,
    Number.parseInt(mapped[2], 16) & 0xff,
  ]) : false;
  const privateIPv6 = hostname.includes(':') && (hostname === '::' || hostname === '::1' || hostname.startsWith('fc') || hostname.startsWith('fd') || /^fe[89ab]/.test(hostname));
  const local = hostname === 'localhost' || hostname.endsWith('.localhost') || hostname.endsWith('.local') || privateIPv4 || mappedIPv4 || privateIPv6;
  assert.equal(parsed.protocol, 'https:', `${name} must use HTTPS`);
  assert.ok(!parsed.username && !parsed.password, `${name} must not contain URL credentials`);
  assert.ok(!parsed.search && !parsed.hash, `${name} must be a base URL without query or fragment`);
  assert.ok(!local, `${name} must not target a local or loopback host`);
}
console.log('production mobile release environment is explicit and safe');
