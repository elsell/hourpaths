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
S256 PKCE and the `openid`, `profile`, `email`, and `identities` scopes required by
HourPaths. The production broker must enable `identities` as an extended ID-token
claim before clients request this scope. The deployed Logto mobile application currently has App ID
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

## Provider identity management contract

The API must derive provider names only from the verified broker ID token's
`identities` claim. Exactly one supported key (`google` or `apple`) with a
nonempty upstream `userId` identifies its provider. Unsupported, malformed, or
multiple identities must not authorize linking. The upstream user identifier
must not replace the broker issuer/subject, enter audit metadata, or be retained
as an application identity key.

Existing sign-in tokens without this extended claim remain valid for ordinary
session exchange. They must not authorize provider linking. Existing identity
rows require trusted broker metadata reconciliation before their provider labels
are presented; email domains, user input, and the selected sign-in button must
never supply that metadata. Existing broker subjects and application user IDs
must remain unchanged.

Linking must use an expiring, single-use server challenge bound to the signed-in
application account and requested provider. Its random nonce must be sent in the
PKCE authorization request and compared with the verified ID token's `nonce`.
A token from a different challenge, account, or provider must fail before any
identity association changes. Consuming the challenge, adding the association,
and recording its audit event must be atomic. A token used for linking must not
also be reusable for ordinary session exchange. Cancellation must preserve the
current application session and all existing identity associations.

Unlinking must serialize changes against the owning account and recheck the
remaining identity count in the mutation transaction. Concurrent requests must
never remove its final identity. Account deletion must fence identity changes
and remove pending challenges along with the account. API responses must not
expose raw issuer/subject keys or another account's email.

### Provider-management rollout

1. Verify discovery advertises `identities` and broker automatic account linking
   is disabled. Keep the existing issuer and subjects unchanged.
2. Enable `identities` in the broker's extended ID-token claims before releasing
   clients that request its scope. Verify signed Google and Apple claims against
   the contract above; discovery support alone is not evidence that tokens carry
   the claim. Do not print tokens or upstream identities in release logs.
3. After migration 76, reconcile blank provider labels using a private, bounded
   export of broker subject and its single supported provider. Match the exact
   configured issuer and subject; reject ambiguous, missing, conflicting, or
   multiple-provider records. Commit each label change with its audit event.
   Report counts only and discard the private export after verification. Never
   overwrite a nonblank provider label or use email to supply a match.
4. Verify Settings on both clients: link the other provider, retain the current
   session, sign in through either provider to the same account, unlink one,
   and reject removal of the final provider. Verify cancel and account-switch
   callbacks cannot adopt or link a different account.

Migration 76 rollback must refuse while any user has multiple identities. Do not
unlink identities automatically to force a schema downgrade. Duplicate-account
recovery is a separate flow and is not established by the ordinary linking UI.

## Duplicate-account recovery contract

Recovery must use a dedicated ten-minute, single-use challenge, bound to the
current provisional enrollment, exact opaque onboarding credential, and its
authenticated provider identity. A
verified email match may admit the offer, but must not select or authorize the
account receiving the identity. Only successful authentication of an already
active account may identify that destination. Both providers must satisfy the
signed provider-claim contract above; unavailable metadata must fail safely.

The authorization request must use PKCE and a purpose-specific random nonce.
The API must reject recovery-purpose tokens at ordinary session exchange and
ordinary linking. Challenge completion must compare the verified token's nonce,
issuer, subject, and provider with the pending recovery operation. No provider
token may be saved for later reuse. Clients must retain only the account-bound
pending intent needed to validate the callback, and discard it on cancellation,
sign-out, expiry, or account replacement.

Completion must serialize against onboarding activation, identity changes, and
account deletion. Under the account locks it must recheck the source enrollment,
both identity associations, the active destination, the challenge, and the
presented unexpired onboarding credential. A completed source account, duplicate
provider at the destination, changed association, replay, or stale challenge
must refuse the operation without partial changes or private-account disclosure.

Associating the new identity, retiring the unfinished enrollment, invalidating
its credentials, creating the destination's application credential, and writing
their audit evidence must commit atomically. The credential transition must not
extend the original application-session family's absolute expiry. Persistence
or audit failure must roll back the whole transition. Product data must never
be migrated between accounts by this operation.

The client may enter the existing account only after securely persisting the
completed session under the same pending account lifecycle. An old callback must
not overwrite a newer session. Both mobile and Studio must expose recovery and
cancel/new-account choices with localized, text-labeled controls and retain
actionable feedback after failure. Ordinary linking must continue to reject an
identity associated with another user, including a provisional user; only this
explicit recovery flow can resolve an unfinished enrollment.

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

Recovery retirement preserves the consumed provider-token hashes used to prevent
session-exchange replay. Source session rows carrying those hashes are retained
under the surviving account with their opaque credentials revoked; their expiry
is unchanged. Other enrollment sessions are removed with the provisional user.
This retention must never grant a source credential access to the surviving user.
