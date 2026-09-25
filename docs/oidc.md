# Configure the production OIDC broker

Production uses one fixed HourPaths-owned production broker issuer. Google and
Apple are upstream identity providers; HourPaths clients and the API trust only
the broker's public OIDC issuer. The broker-hosted provider chooser must present
both Google and Apple on web and mobile. Dex is for local development and CI
only and must not be deployed as the production authority.

Create three public authorization-code clients with the display names
`hourpaths-web`, `hourpaths-mobile`, and `hourpaths-docs`. Configure each
HourPaths deployment with the broker-assigned application identifier, not its
display name. None receives a client secret. Enable authorization code with
S256 PKCE and only the `openid`, `profile`, and `email` scopes required by
HourPaths. The deployed Logto mobile application currently has App ID
`ctdb003l6t7f3d5hidfm7`; local Dex continues to use `hourpaths-mobile` as its
development-only client identifier.

Register these deployed redirect URIs:

- web: `https://APP_ORIGIN/callback`
- mobile: `hourpaths://callback`
- docs: `https://API_ORIGIN/docs`

Permit the web origin to perform the authorization-code token exchange. Configure
the broker's upstream redirect URIs with Google and Apple rather than registering
HourPaths application callbacks directly with those providers. Provider-routing
parameters, when used to bypass the chooser, must be an allowlisted `google` or
`apple` value selected by HourPaths; arbitrary upstream identifiers must not pass
through from an untrusted request.

## Identity and claim contract

The broker must not automatically link identities by email, including verified
email or an apparent match between Google and Apple. Linking remains an explicit
HourPaths workflow that proves both identities. Broker accounts and connectors
must not merge upstream identities outside that workflow.

For a given upstream identity, the same upstream identity must receive the same
broker subject through the web, mobile, and documentation clients. Subjects must
be provider-distinct, stable and must never be reassigned to another upstream
identity. A Google identity and an Apple identity therefore remain distinct
broker `(issuer, subject)` pairs until HourPaths explicitly links both pairs to
one internal user. Pairwise subjects that vary by HourPaths client are not
compatible with this contract.

Tokens must contain the exact broker issuer, stable subject, a configured client
audience, and expiry. Multiple-audience tokens must include a valid authorized
party. The broker must normalize standard upstream claims without inventing
data: Apple may omit `name`, and an Apple private-relay email is an ordinary
email value rather than proof of a separate or matching person.
`email_verified` may support invitation and duplicate-account hints, but neither
email nor provider display data is an identity key.

## Secrets and lifecycle

Keep Google connector credentials and Apple team, service, key identifiers and
private signing key in an external secret manager available only to the broker.
Do not place them in this repository, HourPaths environment variables, public
clients, application images, logs, or generated configuration. Define a bounded
key rotation procedure that overlaps valid broker/upstream keys long enough to
avoid an outage, verifies new keys in staging, removes retired keys deliberately,
and covers emergency revocation. The HourPaths API stores or exchanges no Google
or Apple access or refresh token.

Set the public issuer in every client and in `HOURPATHS_OIDC_ISSUER`. Use
`HOURPATHS_OIDC_BACKCHANNEL_URL` only when the API reaches the same issuer
through a private network address. Its path must match the public issuer; it
does not change token issuer validation.

Treat the production issuer as persisted identity authority, not a replaceable
endpoint. Changing the production issuer is an identity-data migration that
requires an explicit subject mapping and rollback plan; changing the environment
value alone would create different identities for existing people.

Set `HOURPATHS_OIDC_INSECURE=false` in production. Verify sign-in, session
exchange, refresh, revocation, account disablement, provider key rotation, and
provider unavailability before launch. Staging conformance must exercise Google
and Apple separately on web and mobile, prove the cross-client subject contract,
and cover missing Apple name, private-relay email, wrong issuer, wrong audience,
invalid authorized party, unverified email, routing tampering, key rotation, and
broker unavailability. A vendor-specific production broker manifest is
deployment-owned and intentionally absent from this repository.
